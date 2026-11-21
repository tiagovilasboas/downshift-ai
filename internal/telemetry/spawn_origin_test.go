// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import "testing"

func TestOriginOf(t *testing.T) {
	cases := []struct {
		name                        string
		onSession, below, rewritten bool
		want                        SpawnOrigin
	}{
		{"applied rewrite below session", true, true, true, OriginDownshiftRewrite},
		{"refuted rewrite falls to orchestrator below", true, true, false, OriginOrchestratorBelow},
		{"rewrite that did not end below stays session", true, false, true, OriginOrchestratorSession},
		{"on session", true, false, false, OriginOrchestratorSession},
		{"no session model", false, false, false, OriginUnattributed},
		{"rewrite but no session model", false, false, true, OriginUnattributed},
	}
	for _, c := range cases {
		if got := OriginOf(c.onSession, c.below, c.rewritten); got != c.want {
			t.Error(c.name, "got", got, "want", c.want)
		}
	}
}

// The token row follows the origin: only a rewrite fills the rewritten fields,
// only below origins fill the below fields, and an unattributed spawn never
// lands in the session fields.
func TestSpawnPlacementTokensFollowOrigin(t *testing.T) {
	cases := []struct {
		name string
		p    spawnPlacement
		want RoutingRow
	}{
		{"rewrite", spawnPlacement{billable: 10, cacheRead: 2, onSession: true, belowSession: true, rewritten: true},
			RoutingRow{Spawns: 1, Billable: 10, BelowSpawns: 1, BelowBillable: 10, RewrittenSpawns: 1, RewrittenBillable: 10, CacheRead: 2}},
		{"refuted rewrite", spawnPlacement{billable: 10, onSession: true, belowSession: true},
			RoutingRow{Spawns: 1, Billable: 10, BelowSpawns: 1, BelowBillable: 10}},
		{"orchestrator session", spawnPlacement{billable: 10, onSession: true},
			RoutingRow{Spawns: 1, Billable: 10}},
		{"no session model, requested only", spawnPlacement{billable: 10, onRequested: true, belowRequested: true},
			RoutingRow{FallbackSpawns: 1, FallbackBillable: 10, FallbackBelowBillable: 10}},
		{"nothing placed", spawnPlacement{billable: 10, rewritten: true},
			RoutingRow{UnobservedSpawns: 1}},
	}
	for _, c := range cases {
		if got := c.p.tokens(); got != c.want {
			t.Error(c.name, "got", got, "want", c.want)
		}
	}
}
