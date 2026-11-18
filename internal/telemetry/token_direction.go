// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

// Catalog tier ranks, lowest first. A ModelCatalog reports tiers on this scale.
const (
	TierRankSmall = iota
	TierRankMid
	TierRankFrontier
)

// TierFrontier is the event-log name of the frontier tier (Event.Tier).
const TierFrontier = "frontier"

// ModelCatalog is the view of the model catalog telemetry needs: the catalog
// id and tier rank of a model, whether an id is an exact catalog id, and the
// tier rank of an allowlisted gateway row (shadow or live). It uses plain
// values only, so the routing catalog is adapted to it at the edge
// (catalog.TierView, wired by fromcore) and telemetry never imports the router.
type ModelCatalog interface {
	Lookup(harness, id string) (catalogID string, tier int, ok bool)
	IsExactID(harness, id string) bool
	GatewayTier(id string) (tier int, ok bool)
}

// DirectionCatalog ranks the requested and the ran model by catalog tier
// (small < mid < frontier), and Record writes model ids as its catalog ids
// (canonicalizeModels). The composition root sets it at startup; with nil no
// record can be placed, so every priced record is unattributed, and model ids
// are recorded as the harness sent them.
var DirectionCatalog ModelCatalog

// directionVs is the one lower-tier computation: it ranks the model that ran
// against a baseline model by catalog tier, native or gateway. The baseline is
// the session model or, as a labelled fallback, the requested model (RoutingTokens); a
// model the catalog does not know is unattributed, never a guess.
func directionVs(harness, baseline, ran string) string {
	base, got, ok := tiersVs(harness, baseline, ran)
	if !ok {
		return TokenUnattributed
	}
	return rankTiers(base, got)
}

// tiersVs returns the catalog tier ranks of a baseline model and of the model
// that ran: a native run by its catalog row, a gateway run by the tier of its
// allowlisted row. ok is false when either is unknown, never a guess.
func tiersVs(harness, baseline, ran string) (base, got int, ok bool) {
	if DirectionCatalog == nil {
		return 0, 0, false
	}
	if _, base, ok = DirectionCatalog.Lookup(harness, baseline); !ok {
		return 0, 0, false
	}
	if IsGatewayModel(ran) {
		got, ok = DirectionCatalog.GatewayTier(ran)
	} else {
		_, got, ok = DirectionCatalog.Lookup(harness, ran)
	}
	return base, got, ok
}

// rankTiers is the one ranking rule for native and gateway runs: a ran tier
// below the baseline one is cheaper, above it pricier, equal is unchanged.
func rankTiers(requested, ran int) string {
	switch {
	case ran < requested:
		return TokenCheaper
	case ran > requested:
		return TokenPricier
	}
	return TokenUnchanged
}

// tierDirection ranks a ran model against a requested one by native catalog
// tier only (the live line). A model the catalog does not know is
// unattributed, never a guess.
func tierDirection(harness, requested, ran string) string {
	if DirectionCatalog == nil {
		return TokenUnattributed
	}
	_, req, okReq := DirectionCatalog.Lookup(harness, requested)
	_, got, okRan := DirectionCatalog.Lookup(harness, ran)
	if !okReq || !okRan {
		return TokenUnattributed
	}
	return rankTiers(req, got)
}
