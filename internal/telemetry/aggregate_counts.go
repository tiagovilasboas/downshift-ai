// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import "strings"

// countChildEnd tallies how a child ended and reports whether ev was such a
// record.
func (a *aggregator) countChildEnd(ev Event) bool {
	if ev.Outcome == OutcomeChildEnded && ev.ChildStatus != "" {
		a.s.ChildStatus[ev.ChildStatus]++
		return true
	}
	return false
}

// countCostOnly handles post-hoc cost records ("usage") and unrouted
// events ("baseline"). They are not decisions: they must not move Total,
// verdict counts, complexity counts, or normalised units. Usage records
// contribute real-dollar savings.
func (a *aggregator) countCostOnly(ev Event) {
	if ev.Outcome == OutcomeBaseline {
		a.s.Baseline++
		return
	}
	a.s.UsageRecords++
	key := linkKey{ev.Harness, ev.LinkedDecision}
	a.linkUsage(ev, key)
}

// linkUsage counts a usage record once per distinct subagent (or event) when
// it links to a known decision in the same session.
func (a *aggregator) linkUsage(ev Event, key linkKey) {
	identity := "event:" + DecisionKey(ev)
	if ev.AgentHash != "" {
		identity = "agent:" + ev.AgentHash
	}
	unique := linkKey{ev.Harness, identity}
	if decision, ok := a.decisions[key]; ok && ev.LinkedDecision != "" && !a.linked[unique] &&
		(ev.SessionID == "" || decision.SessionID == "" || ev.SessionID == decision.SessionID) {
		a.s.UsageLinked++
		a.linked[unique] = true
	}
}

// countDecision tallies one routing decision across all of its dimensions.
func (a *aggregator) countDecision(ev Event) {
	a.s.Total++
	a.countAgentType(ev)
	a.s.ByEntry[NormalizeEntry(ev.Entry)]++
	if ev.PolicyFloor {
		a.s.PolicyFloor++
	}
	a.s.ByComplexity[ev.Complexity]++
	served, isRefuted := a.refuted[linkKey{ev.Harness, DecisionKey(ev)}]
	a.countModelMix(ev, isRefuted, served)
	if isKnownEffort(ev.FinalEffort) {
		a.s.EffortMix[ev.FinalEffort]++
	} else if isKnownEffort(ev.RequestedEffort) {
		a.s.EffortMix[ev.RequestedEffort]++
	}
	a.countGatewayShadow(ev)
	for _, code := range ev.ReasonCodes {
		a.s.ReasonCodes[code]++
	}
	a.countFrontier(ev, isRefuted)
	applied := AppliedRouting(ev) && !isRefuted
	a.countVerdict(ev, applied)
	// Self-correction: the verdict stands, the safe action applied.
	if len(ev.Corrections) > 0 {
		a.s.Corrected++
	}
}

func (a *aggregator) countAgentType(ev Event) {
	if ev.AgentType == "" {
		return
	}
	c := a.s.ByAgentType[ev.AgentType]
	c.Decisions++
	if ev.FromModel == "" || ev.FromModel == "unknown" {
		c.UnknownBaseline++
	}
	a.s.ByAgentType[ev.AgentType] = c
}

// countModelMix counts the model that actually ran. For a refuted rewrite that
// is the model the resolved record reports, which another hook on the same call
// may have set; the requested model only when the record names none.
func (a *aggregator) countModelMix(ev Event, isRefuted bool, served string) {
	ran := ev.ToModel
	if isRefuted {
		ran = ranOnRefute(ev, served)
	}
	if ran != "" && ran != "unknown" {
		a.s.ModelMix[ShortModel(ran)]++
	}
}

// countGatewayShadow tallies the OpenRouter allowlist shadow observation.
func (a *aggregator) countGatewayShadow(ev Event) {
	if ev.GatewayWould == "" {
		return
	}
	a.s.GatewayObserved++
	if a.s.GatewaySlugs == nil {
		a.s.GatewaySlugs = make(map[string]int)
	}
	a.s.GatewaySlugs[ev.GatewayWould]++
}

// countFrontier counts frontier picks and the subset spent on doubt.
func (a *aggregator) countFrontier(ev Event, isRefuted bool) {
	if !strings.EqualFold(ev.Tier, TierFrontier) {
		return
	}
	a.s.FrontierDecisions++
	if spentOnDoubt(ev) && !isRefuted {
		a.s.FrontierOnDoubt++
	}
}

func (a *aggregator) countVerdict(ev Event, applied bool) {
	switch {
	case ev.Verdict == "DOWNSHIFT" && applied:
		a.s.Downshifted++
	case ev.Verdict == "UPSHIFT" && applied:
		a.s.Upshifted++
	case ev.Verdict == "DOWNSHIFT" || ev.Verdict == "UPSHIFT":
		a.s.NotApplied++
	case ev.Verdict == "OK":
		a.s.OK++
	default:
		a.s.Unknown++
	}
}
