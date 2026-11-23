// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package readout

import (
	"fmt"
	"sort"
	"strings"

	"github.com/tiagovilasboas/downshift-ai/internal/telemetry"
)

// SessionRouting is the routing picture of one session, from the event log.
type SessionRouting struct {
	Spawns    int    // routing decisions made for the session
	TopModel  string // short name of the model most children ran on
	TopModelN int    // how many children ran on TopModel
	// Tokens is the session's routing tokens (RoutingTokens). Live views show
	// tokens only; never dollars.
	Tokens   RoutingRow
	Switched int // spawns whose model or effort Downshift changed
	Held     int // spawns Downshift considered and left unchanged
	OnDoubt  int // frontier decisions resting on an unconfident classification
	// QuotaStale marks that an expired quota snapshot is still holding model
	// rewrites (core.StaleQuotaHold). The caller sets it; the event log has no
	// say in it.
	QuotaStale bool
}

// QuotaStaleMarker is the segment appended while a stale snapshot holds.
const QuotaStaleMarker = "quota stale"

// SessionSummary folds the events that belong to sessionHash (see
// HashSessionID). An empty hash matches nothing: a summary must never mix
// sessions. Cost-only records contribute only their signed real delta.
func SessionSummary(events []telemetry.Event, sessionHash string) SessionRouting {
	var s SessionRouting
	if sessionHash == "" {
		return s
	}
	byModel := map[string]int{}
	refuted := RefutedRewrites(events)
	var mine []telemetry.Event
	for _, ev := range events {
		if ev.SessionID != sessionHash {
			continue
		}
		mine = append(mine, ev)
	}
	s.Tokens = RoutingTokens(mine).Only()
	for _, ev := range mine {
		if telemetry.IsCostOnlyOutcome(ev.Outcome) || telemetry.IsFailureOutcome(ev.Outcome) {
			continue
		}
		s.Spawns++
		switch telemetry.SpawnStateOf(ev) {
		case telemetry.StateSwitched:
			s.Switched++
		case telemetry.StateHeld:
			s.Held++
		}
		served, isRefuted := refuted[ev.Harness+"\n"+telemetry.DecisionKey(ev)]
		ran := ev.FromModel
		switch {
		case isRefuted:
			ran = ranOnRefute(ev, served)
		case telemetry.AppliedRewrite(ev.Outcome, ev.Corrections) && ev.Outcome == telemetry.OutcomeRewriteEmitted:
			ran = ev.ToModel
		}
		if ran != "" && ran != "unknown" {
			byModel[telemetry.ShortModel(ran)]++
		}
		if spentOnDoubt(ev) && !isRefuted {
			s.OnDoubt++
		}
	}
	names := make([]string, 0, len(byModel))
	for m := range byModel {
		names = append(names, m)
	}
	sort.Strings(names) // deterministic tie-break
	for _, m := range names {
		if byModel[m] > s.TopModelN {
			s.TopModel, s.TopModelN = m, byModel[m]
		}
	}
	return s
}

// Line renders the statusline summary, or "" when the session has no routing
// activity and no stale quota to flag. Segments without data are omitted.
func (s SessionRouting) Line() string {
	if s.Spawns == 0 {
		// Nothing routed yet, but a holding snapshot is the reason nothing will.
		if s.QuotaStale {
			return "◆ downshift · " + QuotaStaleMarker
		}
		return ""
	}
	noun := "subagents"
	if s.Spawns == 1 {
		noun = "subagent"
	}
	parts := []string{"◆ downshift", fmt.Sprintf("%d %s", s.Spawns, noun)}
	parts = append(parts, fmt.Sprintf("%d switched · %d held", s.Switched, s.Held))
	if s.TopModel != "" {
		parts = append(parts, fmt.Sprintf("%d on %s", s.TopModelN, s.TopModel))
	}
	if s.Tokens.Line() != "" {
		parts = append(parts, s.Tokens.Line())
	}
	parts = append(parts, fmt.Sprintf("%d on doubt", s.OnDoubt))
	if s.QuotaStale {
		parts = append(parts, QuotaStaleMarker)
	}
	return strings.Join(parts, " · ")
}
