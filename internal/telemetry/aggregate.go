// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

// Aggregate computes Stats from a slice of events.
//
// Two linear passes over events: the first indexes decisions, refutations,
// observed links, session honor groups and respawn timestamps (exported windows
// may arrive out of order); the second accumulates per-event counts, routing
// tokens, surface tallies and duplicate tool_use hashes; finalization folds
// the indexed maps without rescanning events.
func Aggregate(events []Event) Stats {
	a := newAggregator()
	a.indexEvents(events)
	for _, ev := range events {
		a.processEvent(ev)
	}
	a.finalize(events)
	return a.s
}

// linkKey identifies a decision (or a record linked to one) within a harness.
type linkKey struct {
	harness, decision string
}

// aggregator accumulates Stats over one Aggregate call.
type aggregator struct {
	s         Stats
	decisions map[linkKey]Event
	// refuted holds rewrites a linked "resolved" record says the harness did
	// not run: they are not applied and save nothing. The value is the model
	// that record reports ("" when it names none).
	refuted map[linkKey]string
	// linked marks usage records already matched to a decision.
	linked map[linkKey]bool

	honorObserved    map[linkKey]bool
	usageObserved    map[linkKey]bool
	honorIdx         *honorIndex
	respawnIdx       *respawnIndex
	routingDecisions map[linkKey]Event
	routingSpawns    map[string]*spawnPlacement
	dupToolUse       map[linkKey]int
}

func newAggregator() *aggregator {
	return &aggregator{
		s: Stats{
			ByComplexity: make(map[string]int),
			ByEntry:      make(map[string]int),
			ChildStatus:  make(map[ChildStatus]int),
			ReasonCodes:  make(map[string]int),
			ModelMix:     make(map[string]int),
			EffortMix:    make(map[string]int),
			ByAgentType:  make(map[string]AgentTypeCount),
		},
		decisions:        make(map[linkKey]Event),
		refuted:          make(map[linkKey]string),
		linked:           make(map[linkKey]bool),
		honorObserved:    make(map[linkKey]bool),
		usageObserved:    make(map[linkKey]bool),
		honorIdx:         newHonorIndex(),
		respawnIdx:       newRespawnIndex(),
		routingDecisions: make(map[linkKey]Event),
		routingSpawns:    make(map[string]*spawnPlacement),
		dupToolUse:       make(map[linkKey]int),
	}
}

// indexEvents builds all lookup indexes in a single pass before counting.
func (a *aggregator) indexEvents(events []Event) {
	for i, ev := range events {
		if !IsCostOnlyOutcome(ev.Outcome) && !IsFailureOutcome(ev.Outcome) {
			a.decisions[linkKey{ev.Harness, DecisionKey(ev)}] = ev
		}
		if IsDecisionRow(ev) {
			a.routingDecisions[linkKey{ev.Harness, DecisionKey(ev)}] = ev
		}
		if ev.Outcome == OutcomeResolved && ev.LinkedDecision != "" && ev.RewriteHonored != nil && !*ev.RewriteHonored {
			a.refuted[linkKey{ev.Harness, ev.LinkedDecision}] = ev.ToModel
		}
		if ev.Harness != "" && ev.LinkedDecision != "" {
			k := linkKey{ev.Harness, ev.LinkedDecision}
			if ev.Outcome == OutcomeResolved && ev.RewriteHonored != nil && *ev.RewriteHonored {
				a.honorObserved[k] = true
			}
			if ev.Outcome == OutcomeUsage && hasChildTokens(ev) {
				a.usageObserved[k] = true
			}
		}
		a.honorIdx.add(i, ev)
		a.respawnIdx.add(ev)
	}
}

// processEvent tallies one event in the second linear pass.
func (a *aggregator) processEvent(ev Event) {
	if IsDecisionRow(ev) && ev.ToolUseHash != "" {
		a.dupToolUse[linkKey{ev.Harness, ev.ToolUseHash}]++
	}
	if ev.Harness != "" && !IsCostOnlyOutcome(ev.Outcome) && !IsFailureOutcome(ev.Outcome) {
		k := linkKey{ev.Harness, DecisionKey(ev)}
		_, isRefuted := a.refuted[k]
		tallyEventSurface(ev, &a.s, a.honorObserved[k], a.usageObserved[k], isRefuted)
	}
	if ev.Outcome == OutcomeUsage {
		if hasChildTokens(ev) {
			a.s.ObservedChildUsage++
			a.s.ObservedChildTokens += childTokenSum(ev)
		}
		accumulateRoutingSpawn(ev, a.routingDecisions, a.refuted, a.routingSpawns)
	}
	if a.countChildEnd(ev) {
		return // how a child ended: an observation, not a decision or cost
	}
	if ev.Outcome == OutcomeWorkerEnded || ev.Outcome == OutcomeMCPTool {
		return // a delegated worker's end: tokens for the worker budget, not a routed spawn
	}
	if ev.Outcome == OutcomeResolved {
		return // an observation about an earlier decision, not a cost or a decision
	}
	if ev.Outcome == OutcomeDCI {
		return // a line left by the removed code intelligence: not a decision
	}
	if IsCostOnlyOutcome(ev.Outcome) {
		a.countCostOnly(ev)
		return
	}
	a.countDecision(ev)
}

// finalize computes the post-pass tallies from the indexed maps.
func (a *aggregator) finalize(events []Event) {
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
