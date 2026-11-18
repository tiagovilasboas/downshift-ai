// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

// RoutingTokensWithLookup folds the usage records of scoped while indexing
// decisions and refutations from lookup (for example, the full event log
// before day-window filtering). This keeps a usage record inside the window
// placed against its baseline even when the linked decision or resolved record
// fell before the window cutoff.
func RoutingTokensWithLookup(scoped, lookup []Event) RoutingView {
	refuted := map[linkKey]string{}
	decisions := map[linkKey]Event{}
	for _, ev := range lookup {
		if ev.Outcome == OutcomeResolved && ev.LinkedDecision != "" && ev.RewriteHonored != nil && !*ev.RewriteHonored {
			refuted[linkKey{ev.Harness, ev.LinkedDecision}] = ev.ToModel
		}
		if IsDecisionRow(ev) {
			decisions[linkKey{ev.Harness, DecisionKey(ev)}] = ev
		}
	}
	spawns := map[string]*spawnPlacement{}
	for _, ev := range scoped {
		accumulateRoutingSpawn(ev, decisions, refuted, spawns)
	}
	return buildRoutingView(spawns)
}
