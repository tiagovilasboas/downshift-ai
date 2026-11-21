// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import "sort"

// OutcomeMatrixRow is the folded outcomes of the routed tasks that share one
// origin (who placed the model), one tier the child ran on and one agent type.
// Observational: the groups were not randomized, so a row compares nothing.
type OutcomeMatrixRow struct {
	Origin    SpawnOrigin   `json:"origin"`
	Tier      RanTier       `json:"tier"`
	AgentType string        `json:"agent_type"`
	N         int           `json:"n"`
	Outcomes  OutcomeCounts `json:"outcomes"`
	// NoCheckShare is accepted_unchecked over n, so a missing check never
	// reads as a pass.
	NoCheckShare float64 `json:"no_check_share"`
}

// OriginOfSpawn returns the origin of the spawn of harness and decision key
// (TaskRecord.Key). A spawn with no priced usage is unattributed.
func (v RoutingView) OriginOfSpawn(harness, key string) SpawnOrigin {
	if o, ok := v.originBySpawn[harness+"\n"+key]; ok {
		return o
	}
	return OriginUnattributed
}

type matrixKey struct {
	origin SpawnOrigin
	tier   RanTier
	agent  string
}

// BuildOutcomeMatrix folds the routed tasks into origin x tier x agent rows.
// Origin comes from the routing view (the rule tokens use), the tier from
// RanTiers and the agent type from the decision. A task counts once, however
// many usage records its resumed child wrote. Rows are sorted by key.
func BuildOutcomeMatrix(events []Event, tasks []TaskRecord, view RoutingView) []OutcomeMatrixRow {
	agents := map[linkKey]string{}
	for _, ev := range events {
		if isTaskDecision(ev) && ev.AgentType != "" {
			agents[linkKey{ev.Harness, DecisionKey(ev)}] = ev.AgentType
		}
	}
	tiers := RanTiers(events)
	rows := map[matrixKey]*OutcomeMatrixRow{}
	for _, t := range tasks {
		if t.Group != GroupRouted {
			continue
		}
		agent := agents[linkKey{t.Harness, t.Key}]
		if agent == "" {
			agent = UntypedAgent
		}
		tier, ok := tiers[SpawnRef{t.Harness, t.Key}]
		if !ok {
			tier = RanUnknown
		}
		k := matrixKey{view.OriginOfSpawn(t.Harness, t.Key), tier, agent}
		r := rows[k]
		if r == nil {
			r = &OutcomeMatrixRow{Origin: k.origin, Tier: k.tier, AgentType: k.agent}
			rows[k] = r
		}
		r.N++
		r.Outcomes.add(t.Outcome())
	}
	out := make([]OutcomeMatrixRow, 0, len(rows))
	for _, r := range rows {
		r.NoCheckShare = float64(r.Outcomes.AcceptedUnchecked) / float64(r.N)
		out = append(out, *r)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Origin != b.Origin {
			return a.Origin < b.Origin
		}
		if a.Tier != b.Tier {
			return a.Tier < b.Tier
		}
		return a.AgentType < b.AgentType
	})
	return out
}
