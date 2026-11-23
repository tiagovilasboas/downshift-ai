// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package readout

import "github.com/tiagovilasboas/downshift-ai/internal/telemetry"

// Tokens routing placed below the session model: the one routing figure.
//
// Owner decision: savings are reported only as tokens, per harness, never in
// dollars and never summed across harnesses. A lower tier is not fewer tokens;
// these tokens ran, and this view only says where.
//
// Population: the priced usage records (outcome usage, Total > 0), folded per
// spawn, so a resumed child counts once. Baseline per spawn: the session model
// (Event.ParentModel) when the catalog ranks it, else the requested model
// (Event.FromModel) as a labelled fallback, kept in separate fields and never
// added to the session ones. A spawn neither baseline can place is unobserved.
// The one comparator is directionVs (rankTiers). Billable is input, output and
// cache write; cache read has its own field and is never inside billable or
// below. Counts only: no prompt, path or command text.

// RoutingRow is one population of spawns on one harness (or one agent type).
type RoutingRow struct {
	// Spawns and Billable cover the spawns placed against the session model.
	// BelowSpawns and BelowBillable are the part that ran below it.
	Spawns        int   `json:"spawns"`
	BelowSpawns   int   `json:"below_spawns"`
	Billable      int64 `json:"billable"`
	BelowBillable int64 `json:"below_billable"`
	// RewrittenSpawns and RewrittenBillable are the part of the below-session
	// figures whose decision was an applied Downshift rewrite (the rest ran
	// below because the caller or the session already chose it).
	RewrittenSpawns   int   `json:"rewritten_spawns"`
	RewrittenBillable int64 `json:"rewritten_billable"`
	// The fallback fields cover spawns with no usable session model, placed
	// against the requested model instead. Never added to the session fields.
	FallbackSpawns        int   `json:"fallback_spawns"`
	FallbackBillable      int64 `json:"fallback_billable"`
	FallbackBelowBillable int64 `json:"fallback_below_billable"`
	// UnobservedSpawns could not be placed against either baseline.
	UnobservedSpawns int `json:"unobserved_spawns"`
	// CacheRead is the cache-read tokens of every spawn, apart from billable.
	CacheRead int64 `json:"cache_read"`
	// CacheWrite and Input hold the non-read prompt input classes alongside
	// CacheRead so reports can show prompt-cache hit rate per harness.
	CacheWrite int64 `json:"cache_write,omitempty"`
	Input      int64 `json:"input,omitempty"`
}

// Observed reports whether any spawn was placed against the session model.
// False reads "unobserved" in a report, never 0.
func (r RoutingRow) Observed() bool { return r.Spawns > 0 }

// FallbackObserved reports whether any spawn was placed against the requested
// model only.
func (r RoutingRow) FallbackObserved() bool { return r.FallbackSpawns > 0 }

// BelowShare is the share of session-placed billable tokens that ran below the
// session model, in percent; 0 with none.
func (r RoutingRow) BelowShare() float64 { return share(r.BelowBillable, r.Billable) }

// Add folds o into r, for a caller that merges rows of the same bucket (the
// operator export folds harnesses it cannot name into "unknown").
func (r *RoutingRow) Add(o RoutingRow) {
	r.Spawns += o.Spawns
	r.BelowSpawns += o.BelowSpawns
	r.Billable += o.Billable
	r.BelowBillable += o.BelowBillable
	r.RewrittenSpawns += o.RewrittenSpawns
	r.RewrittenBillable += o.RewrittenBillable
	r.FallbackSpawns += o.FallbackSpawns
	r.FallbackBillable += o.FallbackBillable
	r.FallbackBelowBillable += o.FallbackBelowBillable
	r.UnobservedSpawns += o.UnobservedSpawns
	r.CacheRead += o.CacheRead
	r.CacheWrite += o.CacheWrite
	r.Input += o.Input
}

// RoutingView is RoutingRow folded per harness and per agent type. It has no
// cross-harness total: harness "unknown" is its own row. ByAgentType mixes the
// harnesses of the events it was given, so callers scope the events to one
// harness (a session, a panel) before reading it.
type RoutingView struct {
	ByHarness   map[string]RoutingRow `json:"by_harness"`
	ByAgentType map[string]RoutingRow `json:"by_agent_type,omitempty"`
	// QuotaByHarness is the quota-unit estimate of the session-placed spawns,
	// per harness (quota_units.go). Like ByHarness, never summed.
	QuotaByHarness map[string]QuotaUnits `json:"quota_by_harness,omitempty"`
	// bySpawn is keyed by harness + "\n" + decision key.
	bySpawn map[string]RoutingRow
	// originBySpawn has the same key: the placement rule (OriginOf) per spawn.
	originBySpawn map[string]telemetry.SpawnOrigin
}

// Spawn returns the figures of the spawn made by decision d.
func (v RoutingView) Spawn(d telemetry.Event) RoutingRow {
	return v.bySpawn[d.Harness+"\n"+telemetry.DecisionKey(d)]
}

// spawnPlacement accumulates the usage records of one spawn.
type spawnPlacement struct {
	harness, agent      string
	billable, cacheRead int64
	cacheWrite, input   int64
	onSession           bool // some record ranked against the session model
	belowSession        bool
	quota               QuotaUnits // session-placed records weighted by tier
	rewritten           bool       // the decision was an applied model rewrite the harness did not refute
	onRequested         bool       // some record ranked against the requested model
	belowRequested      bool
}

func (p *spawnPlacement) tokens() RoutingRow {
	t := RoutingRow{CacheRead: p.cacheRead, CacheWrite: p.cacheWrite, Input: p.input}
	origin := telemetry.OriginOf(p.onSession, p.belowSession, p.rewritten)
	switch {
	case origin != telemetry.OriginUnattributed:
		t.Spawns, t.Billable = 1, p.billable
		if origin != telemetry.OriginOrchestratorSession {
			t.BelowSpawns, t.BelowBillable = 1, p.billable
		}
		if origin == telemetry.OriginDownshiftRewrite {
			t.RewrittenSpawns, t.RewrittenBillable = 1, p.billable
		}
	case p.onRequested:
		t.FallbackSpawns, t.FallbackBillable = 1, p.billable
		if p.belowRequested {
			t.FallbackBelowBillable = p.billable
		}
	default:
		t.UnobservedSpawns = 1
	}
	return t
}

// RoutingTokens folds the usage records of events into the routing view. A
// resumed child has several records and counts as one spawn: below when any of
// its records ran below the baseline.
func RoutingTokens(events []telemetry.Event) RoutingView {
	return RoutingTokensWithLookup(events, events)
}

func accumulateRoutingSpawn(ev telemetry.Event, decisions map[telemetry.LinkKey]telemetry.Event, refuted map[telemetry.LinkKey]string, spawns map[string]*spawnPlacement) {
	if ev.Outcome != telemetry.OutcomeUsage || !ev.HasTokenUsage() {
		return
	}
	c := telemetry.ClassesOf(ev.TokenUsageOrZero())
	if c.Total == 0 {
		return
	}
	key := telemetry.LinkKey{Harness: ev.Harness, Key: ev.LinkedDecision}
	decision, found := decisions[key]
	_, isRefuted := refuted[key]
	id := ev.Harness + "\n" + ev.LinkedDecision
	if !found {
		id = ev.Harness + "\nusage:" + telemetry.DecisionKey(ev)
	}
	sp := spawns[id]
	if sp == nil {
		sp = &spawnPlacement{harness: ev.Harness, agent: UntypedAgent}
		if found && decision.AgentType != "" {
			sp.agent = decision.AgentType
		}
		sp.rewritten = found && telemetry.IsAppliedShift(decision) && !isRefuted
		spawns[id] = sp
	}
	sp.billable += c.Billable()
	sp.cacheRead += c.CacheRead
	sp.cacheWrite += c.CacheWrite
	sp.input += c.Input
	requested, ran := telemetry.TokenModels(ev, decision, found, isRefuted)
	if found && decision.ParentModel != "" {
		if base, got, ok := telemetry.TiersVs(ev.Harness, decision.ParentModel, ran); ok {
			sp.onSession = true
			sp.belowSession = sp.belowSession || telemetry.RankTiers(base, got) == telemetry.TokenCheaper
			sp.addQuota(base, got, c)
		}
	}
	if d := telemetry.DirectionVs(ev.Harness, requested, ran); d != telemetry.TokenUnattributed {
		sp.onRequested = true
		sp.belowRequested = sp.belowRequested || d == telemetry.TokenCheaper
	}
}

func buildRoutingView(spawns map[string]*spawnPlacement) RoutingView {
	v := RoutingView{ByHarness: map[string]RoutingRow{}, ByAgentType: map[string]RoutingRow{}, QuotaByHarness: map[string]QuotaUnits{}, bySpawn: map[string]RoutingRow{}, originBySpawn: map[string]telemetry.SpawnOrigin{}}
	for id, sp := range spawns {
		if sp.quota.Observed() {
			q := v.QuotaByHarness[sp.harness]
			q.add(sp.quota)
			v.QuotaByHarness[sp.harness] = q
		}
		t := sp.tokens()
		v.bySpawn[id] = t
		v.originBySpawn[id] = telemetry.OriginOf(sp.onSession, sp.belowSession, sp.rewritten)
		h := v.ByHarness[sp.harness]
		h.Add(t)
		v.ByHarness[sp.harness] = h
		g := v.ByAgentType[sp.agent]
		g.Add(t)
		v.ByAgentType[sp.agent] = g
	}
	return v
}

// Only returns the row of a view that holds exactly one harness, as a session
// does. With none or several it returns the zero value: harnesses are never
// summed.
func (v RoutingView) Only() RoutingRow {
	if len(v.ByHarness) != 1 {
		return RoutingRow{}
	}
	for _, r := range v.ByHarness {
		return r
	}
	return RoutingRow{}
}

// UntypedAgent labels spawns whose decision named no agent type.
const UntypedAgent = "untyped"
