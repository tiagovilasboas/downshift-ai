// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package readout

import (
	"github.com/tiagovilasboas/downshift-ai/internal/telemetry"
	"sort"
	"strings"
)

// PanelLabel is the short phrase for one spawn.
func PanelLabel(r telemetry.SpawnRecord, ev telemetry.Event) string {
	switch r.State {
	case telemetry.StateSwitched:
		if r.FromModel == r.ToModel {
			return "effort changed (" + strings.ToLower(ev.Tier) + ")"
		}
		return telemetry.ShortOrQ(r.FromModel) + " → " + telemetry.ShortOrQ(r.ToModel) + " (" + strings.ToLower(ev.Tier) + ")"
	case telemetry.StateHeld:
		return "held: original model kept"
	case telemetry.StatePaused:
		return "paused (unrouted)"
	case telemetry.StateUnrouted:
		return "unrouted (older log)"
	case telemetry.StateBlocked:
		return "blocked: respawn on another model"
	}
	return "kept"
}

// ChildTokensLabel is one agent's token text: "48k tokens · below the session
// model", "48k tokens · below the requested model" when only the requested model
// ranks, "48k tokens" otherwise, or "" before its tokens are known. Billable
// tokens only; cache reads stay out.
func ChildTokensLabel(c RoutingRow) string {
	switch {
	case c.Observed():
		out := telemetry.FormatTokens(c.Billable) + " tokens"
		if c.BelowSpawns > 0 {
			out += " · below the session model"
		}
		return out
	case c.FallbackObserved():
		out := telemetry.FormatTokens(c.FallbackBillable) + " tokens"
		if c.FallbackBelowBillable > 0 {
			out += " · below the requested model"
		}
		return out
	}
	return ""
}

// AgentTypeLines is one row per agent type with its AgentTypeLine, ordered by
// billable tokens, most first.
func AgentTypeLines(v RoutingView) []PanelAgentType {
	out := make([]PanelAgentType, 0, len(v.ByAgentType))
	for typ, s := range v.ByAgentType {
		out = append(out, PanelAgentType{AgentType: typ, Tokens: s, Line: AgentTypeLine(typ, s)})
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i].Tokens, out[j].Tokens
		if ta, tb := a.Billable+a.FallbackBillable, b.Billable+b.FallbackBillable; ta != tb {
			return ta > tb
		}
		return out[i].AgentType < out[j].AgentType
	})
	return out
}

// BelowTypeLines is the AgentTypeLine of every type that ran something below
// the session model, or nil when there is nothing worth a line: one untyped
// group repeats the total, and a row of zeros is noise.
func BelowTypeLines(types []PanelAgentType) []string {
	var out []string
	for _, t := range types {
		if t.Tokens.BelowBillable+t.Tokens.FallbackBelowBillable > 0 {
			out = append(out, t.Line)
		}
	}
	if len(out) == 1 && len(types) == 1 && types[0].AgentType == UntypedAgent {
		return nil
	}
	return out
}

// AgentTypeLine is "general-purpose: 12k of 40k billable below the session
// model (3 of 5 spawns)".
func AgentTypeLine(typ string, s RoutingRow) string {
	if line := s.Line(); line != "" {
		return typ + ": " + line
	}
	return typ + ": unobserved"
}
