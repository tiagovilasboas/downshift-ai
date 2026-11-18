// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"strings"
	"testing"
)

// routingSpawn records billable 1700 (1000 in, 500 out, 200 cache write) plus
// cacheRead; weights: small 0.05 (cache read 0.1), mid 1, frontier 2.
func TestRoutingTokens_QuotaUnits(t *testing.T) {
	cases := []struct {
		name             string
		parent, req, ran string
		cacheRead        int64
		want             QuotaUnits
	}{
		{"frontier to mid halves", "claude-opus-4-8", "claude-opus-4-8", "claude-sonnet-4-6", 0, QuotaUnits{Session: 3400, Ran: 1700}},
		{"same tier saves nothing", "claude-sonnet-4-6", "claude-sonnet-4-6", "claude-sonnet-4-6", 0, QuotaUnits{Session: 1700, Ran: 1700}},
		{"cache read has its own small weight", "claude-opus-4-8", "claude-opus-4-8", "claude-haiku-4-5", 50000,
			QuotaUnits{Session: 103400, Ran: 85 + 5000}},
		{"above the session tier costs more", "claude-haiku-4-5", "claude-haiku-4-5", "claude-opus-4-8", 0, QuotaUnits{Session: 85, Ran: 3400}},
		{"no session model is not weighted", "", "claude-opus-4-8", "claude-sonnet-4-6", 0, QuotaUnits{}},
		{"unknown model is not weighted", "claude-opus-4-8", "claude-opus-4-8", "mystery", 0, QuotaUnits{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := RoutingTokens(routingSpawn("claude-code", "d", tc.parent, tc.req, tc.ran, tc.cacheRead)).QuotaByHarness["claude-code"]
			if got != tc.want {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestRoutingTokens_QuotaUnitsPerHarness(t *testing.T) {
	ev := routingSpawn("claude-code", "a", "claude-opus-4-8", "claude-opus-4-8", "claude-sonnet-4-6", 0)
	ev = append(ev, routingSpawn("claude-code", "b", "claude-opus-4-8", "claude-opus-4-8", "claude-opus-4-8", 0)...)
	ev = append(ev, routingSpawn("codex", "c", "claude-opus-4-8", "claude-opus-4-8", "claude-sonnet-4-6", 0)...)
	v := RoutingTokens(ev)
	// The codex spawn never folds into the claude-code estimate.
	if got, want := v.QuotaByHarness["claude-code"], (QuotaUnits{Session: 6800, Ran: 5100}); got != want {
		t.Fatalf("claude-code got %+v, want %+v", got, want)
	}
}

func TestQuotaUnitsLine(t *testing.T) {
	if got := (QuotaUnits{}).Line(); got != "" {
		t.Fatalf("empty estimate printed %q", got)
	}
	lower := QuotaUnits{Session: 4000, Ran: 3000}.Line()
	for _, want := range []string{"25.0% lower", "1.0k of 4.0k mid-tier units", "same tokens assumed", "Claude API list rates 2026-11-16"} {
		if !strings.Contains(lower, want) {
			t.Fatalf("line %q lacks %q", lower, want)
		}
	}
	if higher := (QuotaUnits{Session: 1000, Ran: 1500}).Line(); !strings.Contains(higher, "50.0% higher") {
		t.Fatalf("line %q should read higher", higher)
	}
}
