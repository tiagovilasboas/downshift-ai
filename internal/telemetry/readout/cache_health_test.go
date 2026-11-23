// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package readout

import (
	"strings"
	"testing"
)

func TestRoutingRow_CacheHealth(t *testing.T) {
	cases := []struct {
		name      string
		row       RoutingRow
		wantObs   bool
		wantKnown bool
		wantPct   float64
		wantMD    []string
		wantPlain []string
	}{
		{
			name: "zero cache emits empty string",
			row:  RoutingRow{Spawns: 1, Billable: 1000},
		},
		{
			name:    "legacy row with cache_read only keeps hit pct unknown",
			row:     RoutingRow{Spawns: 1, Billable: 1000, CacheRead: 500},
			wantObs: true,
			wantMD:  []string{"cache read 500, apart from the tokens above"},
		},
		{
			name:      "high cache hit shows 98.0 percent and write count",
			row:       RoutingRow{Spawns: 1, Billable: 1500, CacheRead: 98000, CacheWrite: 1500, Input: 500},
			wantObs:   true,
			wantKnown: true,
			wantPct:   98.0,
			wantMD:    []string{"cache read 98k, apart from the tokens above (98.0% cache hit; cache write 1.5k)"},
			wantPlain: []string{"cache read  98k (apart from billable) · 98.0% cache hit (cache write 1.5k)"},
		},
		{
			name:      "cache miss (write without read) renders 0.0 percent cache hit",
			row:       RoutingRow{Spawns: 1, Billable: 50200, CacheRead: 0, CacheWrite: 50000, Input: 200},
			wantObs:   true,
			wantKnown: true,
			wantPct:   0.0,
			wantMD:    []string{"0.0% cache hit; cache write 50k"},
			wantPlain: []string{"0.0% cache hit (cache write 50k)"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.row.CacheObserved(); got != tc.wantObs {
				t.Errorf("CacheObserved() = %v, want %v", got, tc.wantObs)
			}
			if got := tc.row.CacheHitKnown(); got != tc.wantKnown {
				t.Errorf("CacheHitKnown() = %v, want %v", got, tc.wantKnown)
			}
			if got := tc.row.CacheHitPct(); got < tc.wantPct-0.05 || got > tc.wantPct+0.05 {
				t.Errorf("CacheHitPct() = %.2f, want %.2f", got, tc.wantPct)
			}
			md := tc.row.cacheReadText(true)
			plain := tc.row.cacheReadText(false)
			if !tc.wantObs && (md != "" || plain != "") {
				t.Errorf("expected empty strings, got md=%q plain=%q", md, plain)
			}
			for _, w := range tc.wantMD {
				if !strings.Contains(md, w) {
					t.Errorf("markdown %q missing %q", md, w)
				}
			}
			for _, w := range tc.wantPlain {
				if !strings.Contains(plain, w) {
					t.Errorf("plain %q missing %q", plain, w)
				}
			}
		})
	}
}

func TestRoutingTokens_TracksCacheHitRateEndToEnd(t *testing.T) {
	ev := routingSpawn("claude-code", "d1", "claude-opus-4-8", "claude-opus-4-8", "claude-haiku-4-5", 98800)
	// routingSpawn uses: input=1000, output=500, cacheWrite=200, cacheRead=98800 -> total prompt input = 100000.
	row := RoutingTokens(ev).ByHarness["claude-code"]
	if row.CacheRead != 98800 || row.CacheWrite != 200 || row.Input != 1000 {
		t.Fatalf("unexpected row cache classes: %+v", row)
	}
	if !row.CacheHitKnown() || row.CacheHitPct() < 98.7 || row.CacheHitPct() > 98.9 {
		t.Fatalf("CacheHitPct = %.2f, want ~98.8", row.CacheHitPct())
	}
}
