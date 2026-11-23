// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package readout

import "github.com/tiagovilasboas/downshift-ai/internal/telemetry"

// RoutingTokensWithLookup folds the usage records of scoped while indexing
// decisions and refutations from lookup (for example, the full event log
// before day-window filtering). This keeps a usage record inside the window
// placed against its baseline even when the linked decision or resolved record
// fell before the window cutoff.
func RoutingTokensWithLookup(scoped, lookup []telemetry.Event) RoutingView {
	refuted := map[telemetry.LinkKey]string{}
	decisions := map[telemetry.LinkKey]telemetry.Event{}
	for _, ev := range lookup {
		if ev.Outcome == telemetry.OutcomeResolved && ev.LinkedDecision != "" && ev.RewriteHonored != nil && !*ev.RewriteHonored {
			refuted[telemetry.LinkKey{Harness: ev.Harness, Key: ev.LinkedDecision}] = ev.ToModel
		}
		if telemetry.IsDecisionRow(ev) {
			decisions[telemetry.LinkKey{Harness: ev.Harness, Key: telemetry.DecisionKey(ev)}] = ev
		}
	}
	spawns := map[string]*spawnPlacement{}
	for _, ev := range scoped {
		accumulateRoutingSpawn(ev, decisions, refuted, spawns)
	}
	return buildRoutingView(spawns)
}
