// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package readout

import "github.com/tiagovilasboas/downshift-ai/internal/telemetry"

// RanTier is the catalog tier of the model a spawn actually ran on: the
// applied model, not the classifier's recommendation (Event.Tier).
type RanTier string

// The ran-on tiers. RanUnknown means no model could be placed in the catalog.
const (
	RanSmall    RanTier = "small"
	RanMid      RanTier = "mid"
	RanFrontier RanTier = "frontier"
	RanUnknown  RanTier = "unknown"
)

// SpawnRef names one spawn: its harness and decision key (TaskRecord.Key).
type SpawnRef struct{ Harness, Key string }

// RanTiers returns the tier each decision's child ran on. The model resolves
// as for RoutingTokens (tokenModels): a usage record is the observation and
// wins; otherwise the applied model of the decision (the final model of an
// applied rewrite, the requested one of an allow, a hold or a rewrite a
// resolved record refuted). A resumed child with several usage records takes
// its highest tier. A model the catalog does not rank, or no catalog, is
// RanUnknown, never a guess. A rewrite the harness never confirmed is assumed
// honored, as everywhere else in the report.
func RanTiers(events []telemetry.Event) map[SpawnRef]RanTier {
	refuted := map[telemetry.LinkKey]string{}
	usage := map[telemetry.LinkKey][]telemetry.Event{}
	for _, ev := range events {
		if ev.Outcome == telemetry.OutcomeResolved && ev.LinkedDecision != "" && ev.RewriteHonored != nil && !*ev.RewriteHonored {
			refuted[telemetry.LinkKey{Harness: ev.Harness, Key: ev.LinkedDecision}] = ev.ToModel
		}
		if ev.Outcome == telemetry.OutcomeUsage && ev.LinkedDecision != "" {
			k := telemetry.LinkKey{Harness: ev.Harness, Key: ev.LinkedDecision}
			usage[k] = append(usage[k], ev)
		}
	}
	out := map[SpawnRef]RanTier{}
	for _, d := range events {
		if !IsTaskDecision(d) {
			continue
		}
		k := telemetry.LinkKey{Harness: d.Harness, Key: telemetry.DecisionKey(d)}
		records := usage[k]
		if len(records) == 0 {
			records = []telemetry.Event{{}}
		}
		best := RanUnknown
		_, isRefuted := refuted[k]
		for _, u := range records {
			_, ran := telemetry.TokenModels(u, d, true, isRefuted)
			best = higherRan(best, ranTierOf(d.Harness, ran))
		}
		out[SpawnRef{d.Harness, k.Key}] = best
	}
	return out
}

func ranTierOf(harness, model string) RanTier {
	if telemetry.DirectionCatalog == nil || !telemetry.KnownModel(model) {
		return RanUnknown
	}
	var rank int
	var ok bool
	if telemetry.IsGatewayModel(model) {
		rank, ok = telemetry.DirectionCatalog.GatewayTier(model)
	} else {
		_, rank, ok = telemetry.DirectionCatalog.Lookup(harness, model)
	}
	if !ok {
		return RanUnknown
	}
	switch rank {
	case telemetry.TierRankSmall:
		return RanSmall
	case telemetry.TierRankMid:
		return RanMid
	case telemetry.TierRankFrontier:
		return RanFrontier
	}
	return RanUnknown
}

func ranRank(t RanTier) int {
	switch t {
	case RanSmall:
		return 1
	case RanMid:
		return 2
	case RanFrontier:
		return 3
	}
	return 0
}

func higherRan(a, b RanTier) RanTier {
	if ranRank(b) > ranRank(a) {
		return b
	}
	return a
}
