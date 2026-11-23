// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package readout

import (
	"github.com/tiagovilasboas/downshift-ai/internal/telemetry"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/catalog"
	"github.com/tiagovilasboas/downshift-ai/internal/core"
)

const (
	rankHarness      = "claude-code"
	allowlistSmallID = "vendor/cheap-model"
	allowlistMidID   = "vendor/mid-model"
)

func tokenDirection(usage, decision telemetry.Event, found, _, refuted bool) string {
	requested, ran := telemetry.TokenModels(usage, decision, found, refuted)
	if !telemetry.KnownModel(requested) || !telemetry.KnownModel(ran) {
		return telemetry.TokenUnattributed
	}
	return telemetry.DirectionVs(decision.Harness, requested, ran)
}

// allowlistResolver is the catalog plus two allowlisted gateway rows, so the
// tests do not depend on the owner's openrouter.json.
type allowlistResolver struct {
	core.Resolver
	rows []core.Model
}

func (r allowlistResolver) ShadowGateways() []core.Model { return r.rows }

func withAllowlist(t *testing.T) {
	t.Helper()
	prev := telemetry.DirectionCatalog
	telemetry.DirectionCatalog = catalog.TierViewOf(allowlistResolver{Resolver: catalog.Load(), rows: []core.Model{
		{ID: allowlistSmallID, Tier: core.TierSmall, Harness: "opencode"},
		{ID: allowlistMidID, Tier: core.TierMid, Harness: "opencode"},
	}})
	t.Cleanup(func() { telemetry.DirectionCatalog = prev })
}

// gatewayEvents builds an applied rewrite and its priced usage record for a
// child that ran on ran. Native and gateway ids take the same shape.
func gatewayEvents(requested, ran string) (usage, decision telemetry.Event) {
	in, out := int64(1000), int64(500)
	decision = telemetry.Event{
		CorrelationID: "d", Harness: rankHarness, SessionID: "s",
		Outcome: telemetry.OutcomeRewriteEmitted, FromModel: requested, ToModel: ran,
	}
	usage = telemetry.Event{
		CorrelationID: "u", Harness: rankHarness, SessionID: "s", Outcome: telemetry.OutcomeUsage,
		LinkedDecision: "d", FromModel: requested, ToModel: ran,
		BaselineSource: telemetry.BaselineRequested, InputTokens: &in, OutputTokens: &out,
	}
	return usage, decision
}

func TestTokenDirection_GatewayRankedByAllowlistTier(t *testing.T) {
	withAllowlist(t)
	cat := catalog.Load()
	mid := cat.ModelFor(rankHarness, core.TierMid).ID
	small := cat.ModelFor(rankHarness, core.TierSmall).ID
	haiku := "claude-haiku-4-5"
	if m, ok := cat.LookupByID(rankHarness, haiku); !ok || m.Tier != core.TierSmall {
		t.Fatalf("%s must be a small-tier native model for this table", haiku)
	}
	cases := []struct {
		name      string
		requested string
		ran       string
		refuted   bool
		want      string
	}{
		{name: "small gateway row for a requested mid is saved", requested: mid, ran: allowlistSmallID, want: telemetry.TokenCheaper},
		{name: "mid gateway row for a requested haiku is pricier", requested: haiku, ran: allowlistMidID, want: telemetry.TokenPricier},
		{name: "same-tier gateway row is unchanged", requested: mid, ran: allowlistMidID, want: telemetry.TokenUnchanged},
		{name: "unknown gateway id is unattributed", requested: mid, ran: "vendor/unlisted-model", want: telemetry.TokenUnattributed},
		{name: "refuted gateway run is unchanged", requested: mid, ran: allowlistSmallID, refuted: true, want: telemetry.TokenUnchanged},
		{name: "native small for a requested mid is saved", requested: mid, ran: small, want: telemetry.TokenCheaper},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			usage, decision := gatewayEvents(tc.requested, tc.ran)
			if got := tokenDirection(usage, decision, true, false, tc.refuted); got != tc.want {
				t.Fatalf("tokenDirection = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestGatewayRun_SmallRowBelowInRouting(t *testing.T) {
	withAllowlist(t)
	mid := catalog.Load().ModelFor(rankHarness, core.TierMid).ID
	usage, decision := gatewayEvents(mid, allowlistSmallID)
	s := RoutingTokens([]telemetry.Event{decision, usage}).ByHarness[decision.Harness]
	if s.FallbackSpawns != 1 || s.FallbackBelowBillable == 0 || s.UnobservedSpawns != 0 {
		t.Fatalf("got %+v; want one fallback spawn below the requested model", s)
	}
}

// liveResolver exposes an allowlisted row the user made live, the routing the
// owner's row carries. It is the catalog plus LiveGateways only.
type liveResolver struct {
	core.Resolver
	rows []core.Model
}

func (r liveResolver) LiveGateways() []core.Model { return r.rows }

// A vendor-prefixed requested id (anthropic/claude-opus-4.6) resolves to its
// catalog family, and a run on a live small gateway row is saved against it.
func TestTokenDirection_VendorPrefixedRequestedWithLiveRowIsCheaper(t *testing.T) {
	const liveSmall = "deepseek/deepseek-v4.1-flash-1017"
	prev := telemetry.DirectionCatalog
	telemetry.DirectionCatalog = catalog.TierViewOf(liveResolver{Resolver: catalog.Load(), rows: []core.Model{
		{ID: liveSmall, Tier: core.TierSmall, Harness: rankHarness, Gateway: "openrouter"},
	}})
	t.Cleanup(func() { telemetry.DirectionCatalog = prev })

	usage, decision := gatewayEvents("anthropic/claude-opus-4.6", liveSmall)
	if got := tokenDirection(usage, decision, true, false, false); got != telemetry.TokenCheaper {
		t.Fatalf("tokenDirection = %q, want %q", got, telemetry.TokenCheaper)
	}
}
