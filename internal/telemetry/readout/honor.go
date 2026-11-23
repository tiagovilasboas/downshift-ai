// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package readout

import "github.com/tiagovilasboas/downshift-ai/internal/telemetry"

// CountInferredHonored counts applied model rewrites that later look honored.
// The result is an inference, not proof: Downshift cannot see the model a
// child actually ran on, only the model a later spawn asked for.
//
// A shift is a rewrite the hook actually wrote: outcome rewrite_emitted, no
// guardrail correction, and requested_model != final_model. Allow, blocked,
// held and effort-only events never count as shifts. A shift counts as
// (inferred) honored when rewrite_honored is true on the event, or when a
// later decision event (any outcome) in the same harness + hashed session
// arrives with requested_model equal to that final_model.
//
// A later "resolved" record that links to the shift and reports a matching
// model also counts, and needs no later spawn. Events without a session id
// cannot be inferred. A resolved record that reports another model refutes
// the shift: it is never counted honored, whatever later spawns ask for. The
// log is not rewritten.
func CountInferredHonored(events []telemetry.Event) (shifted, honored int) {
	t := TallyHonored(events)
	return t.Shifted, t.Honored
}

// HonorTally splits the shifts into those seen honored, those a linked
// "resolved" record refutes (the harness ran another model), and the rest,
// which have no evidence either way (no resolved record, no later spawn).
type HonorTally struct {
	Shifted, Honored, Refuted int
}

// Unobserved is the shifts with neither confirming nor refuting evidence.
func (t HonorTally) Unobserved() int { return t.Shifted - t.Honored - t.Refuted }

// TallyHonored is CountInferredHonored with the refuted shifts counted apart,
// so a miss can be told from a rewrite nobody observed.
func TallyHonored(events []telemetry.Event) HonorTally {
	idx := newHonorIndex()
	for i, ev := range events {
		idx.add(i, ev)
	}
	return tallyHonoredFromIndex(events, idx)
}

type honorSessionKey struct{ harness, session string }

type honorIndex struct {
	observed map[string]bool
	refuted  map[string]bool
	groups   map[honorSessionKey][]int
}

func newHonorIndex() *honorIndex {
	return &honorIndex{
		observed: make(map[string]bool),
		refuted:  make(map[string]bool),
		groups:   make(map[honorSessionKey][]int),
	}
}

func (h *honorIndex) add(i int, ev telemetry.Event) {
	if ev.Outcome == telemetry.OutcomeResolved && ev.LinkedDecision != "" && ev.RewriteHonored != nil {
		if *ev.RewriteHonored {
			h.observed[ev.LinkedDecision] = true
		} else {
			h.refuted[ev.LinkedDecision] = true
		}
	}
	if telemetry.IsCostOnlyOutcome(ev.Outcome) || telemetry.IsFailureOutcome(ev.Outcome) || ev.Harness == "" || ev.SessionID == "" {
		return
	}
	k := honorSessionKey{ev.Harness, ev.SessionID}
	h.groups[k] = append(h.groups[k], i)
}

func tallyHonoredFromIndex(events []telemetry.Event, h *honorIndex) HonorTally {
	var t HonorTally
	for _, idxs := range h.groups {
		for a, idx := range idxs {
			ev := events[idx]
			if !telemetry.IsAppliedShift(ev) {
				continue
			}
			t.Shifted++
			if h.refuted[telemetry.DecisionKey(ev)] && !h.observed[telemetry.DecisionKey(ev)] {
				t.Refuted++
				continue
			}
			if (ev.RewriteHonored != nil && *ev.RewriteHonored) || h.observed[telemetry.DecisionKey(ev)] {
				t.Honored++
				continue
			}
			for _, laterIdx := range idxs[a+1:] {
				later := events[laterIdx]
				if later.FromModel == ev.ToModel && later.FromModel != "" && later.FromModel != "unknown" {
					t.Honored++
					break
				}
			}
		}
	}
	return t
}

// RefutedRewrites returns the decisions (harness + "\n" + decision key) that
// a linked "resolved" record refutes: the harness ran another model. The value
// is the model the resolved record reports (its final_model), "" when it names
// none. That model is not always the one the decision was asked for: another
// hook on the same call can set a third model and win.
func RefutedRewrites(events []telemetry.Event) map[string]string {
	out := map[string]string{}
	for _, ev := range events {
		if ev.Outcome == telemetry.OutcomeResolved && ev.LinkedDecision != "" && ev.RewriteHonored != nil && !*ev.RewriteHonored {
			out[ev.Harness+"\n"+ev.LinkedDecision] = ev.ToModel
		}
	}
	return out
}

// ranOnRefute is the model a refuted rewrite ran on: the one the resolved
// record reports, else the model the decision was asked for.
func ranOnRefute(decision telemetry.Event, served string) string {
	if served != "" && served != "unknown" {
		return served
	}
	return decision.FromModel
}
