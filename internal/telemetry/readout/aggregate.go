// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package readout

import "github.com/tiagovilasboas/downshift-ai/internal/telemetry"

// Aggregate computes Stats from a slice of events.
//
// Two linear passes over events: the first indexes decisions, refutations,
// observed links, session honor groups and respawn timestamps (exported windows
// may arrive out of order); the second accumulates per-event counts, routing
// tokens, surface tallies and duplicate tool_use hashes; finalization folds
// the indexed maps without rescanning events.
func Aggregate(events []telemetry.Event) Stats {
	a := newAggregator()
	a.indexEvents(events)
	for _, ev := range events {
		a.processEvent(ev)
	}
	a.finalize(events)
	return a.s
}

// aggregator accumulates Stats over one Aggregate call.
type aggregator struct {
	s         Stats
	decisions map[telemetry.LinkKey]telemetry.Event
	// refuted holds rewrites a linked "resolved" record says the harness did
	// not run: they are not applied and save nothing. The value is the model
	// that record reports ("" when it names none).
	refuted map[telemetry.LinkKey]string
	// linked marks usage records already matched to a decision.
	linked map[telemetry.LinkKey]bool

	honorObserved    map[telemetry.LinkKey]bool
	usageObserved    map[telemetry.LinkKey]bool
	honorIdx         *honorIndex
	respawnIdx       *respawnIndex
	routingDecisions map[telemetry.LinkKey]telemetry.Event
	routingSpawns    map[string]*spawnPlacement
	dupToolUse       map[telemetry.LinkKey]int
}

func newAggregator() *aggregator {
	return &aggregator{
		s: Stats{
			ByComplexity: make(map[string]int),
			ByEntry:      make(map[string]int),
			ChildStatus:  make(map[telemetry.ChildStatus]int),
			ReasonCodes:  make(map[string]int),
			ModelMix:     make(map[string]int),
			EffortMix:    make(map[string]int),
			ByAgentType:  make(map[string]telemetry.AgentTypeCount),
		},
		decisions:        make(map[telemetry.LinkKey]telemetry.Event),
		refuted:          make(map[telemetry.LinkKey]string),
		linked:           make(map[telemetry.LinkKey]bool),
		honorObserved:    make(map[telemetry.LinkKey]bool),
		usageObserved:    make(map[telemetry.LinkKey]bool),
		honorIdx:         newHonorIndex(),
		respawnIdx:       newRespawnIndex(),
		routingDecisions: make(map[telemetry.LinkKey]telemetry.Event),
		routingSpawns:    make(map[string]*spawnPlacement),
		dupToolUse:       make(map[telemetry.LinkKey]int),
	}
}

// indexEvents builds all lookup indexes in a single pass before counting.
func (a *aggregator) indexEvents(events []telemetry.Event) {
	for i, ev := range events {
		if !telemetry.IsCostOnlyOutcome(ev.Outcome) && !telemetry.IsFailureOutcome(ev.Outcome) {
			a.decisions[telemetry.LinkKey{Harness: ev.Harness, Key: telemetry.DecisionKey(ev)}] = ev
		}
		if telemetry.IsDecisionRow(ev) {
			a.routingDecisions[telemetry.LinkKey{Harness: ev.Harness, Key: telemetry.DecisionKey(ev)}] = ev
		}
		if ev.Outcome == telemetry.OutcomeResolved && ev.LinkedDecision != "" && ev.RewriteHonored != nil && !*ev.RewriteHonored {
			a.refuted[telemetry.LinkKey{Harness: ev.Harness, Key: ev.LinkedDecision}] = ev.ToModel
		}
		if ev.Harness != "" && ev.LinkedDecision != "" {
			k := telemetry.LinkKey{Harness: ev.Harness, Key: ev.LinkedDecision}
			if ev.Outcome == telemetry.OutcomeResolved && ev.RewriteHonored != nil && *ev.RewriteHonored {
				a.honorObserved[k] = true
			}
			if ev.Outcome == telemetry.OutcomeUsage && telemetry.HasChildTokens(ev) {
				a.usageObserved[k] = true
			}
		}
		a.honorIdx.add(i, ev)
		a.respawnIdx.add(ev)
	}
}

// processEvent tallies one event in the second linear pass.
func (a *aggregator) processEvent(ev telemetry.Event) {
	if telemetry.IsDecisionRow(ev) && ev.ToolUseHash != "" {
		a.dupToolUse[telemetry.LinkKey{Harness: ev.Harness, Key: ev.ToolUseHash}]++
	}
	if ev.Harness != "" && !telemetry.IsCostOnlyOutcome(ev.Outcome) && !telemetry.IsFailureOutcome(ev.Outcome) {
		k := telemetry.LinkKey{Harness: ev.Harness, Key: telemetry.DecisionKey(ev)}
		_, isRefuted := a.refuted[k]
		tallyEventSurface(ev, &a.s, a.honorObserved[k], a.usageObserved[k], isRefuted)
	}
	if ev.Outcome == telemetry.OutcomeUsage {
		if telemetry.HasChildTokens(ev) {
			a.s.ObservedChildUsage++
			a.s.ObservedChildTokens += childTokenSum(ev)
		}
		accumulateRoutingSpawn(ev, a.routingDecisions, a.refuted, a.routingSpawns)
	}
	if a.countChildEnd(ev) {
		return // how a child ended: an observation, not a decision or cost
	}
	if ev.Outcome == telemetry.OutcomeWorkerEnded || ev.Outcome == telemetry.OutcomeMCPTool || ev.Outcome == telemetry.OutcomeSkillUsed {
		return // an adoption or worker observation, not a routed spawn or cost
	}
	if ev.Outcome == telemetry.OutcomeResolved {
		return // an observation about an earlier decision, not a cost or a decision
	}
	if ev.Outcome == telemetry.OutcomeDCI {
		return // a line left by the removed code intelligence: not a decision
	}
	if telemetry.IsCostOnlyOutcome(ev.Outcome) {
		a.countCostOnly(ev)
		return
	}
	a.countDecision(ev)
}

// finalize computes the post-pass tallies from the indexed maps.
func (a *aggregator) finalize(events []telemetry.Event) {
	t := tallyHonoredFromIndex(events, a.honorIdx)
	a.s.RewriteShifted, a.s.RewriteHonored = t.Shifted, t.Honored
	a.s.RespawnAfterFailure = len(respawnFromIndex(a.respawnIdx, respawnWindow))
	a.s.Routing = buildRoutingView(a.routingSpawns)
	for _, c := range a.dupToolUse {
		if c > 1 {
			a.s.DuplicateToolUse++
		}
	}
}
