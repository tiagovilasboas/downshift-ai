// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package readout

import (
	"strings"
	"testing"
)

func TestPrintRouting(t *testing.T) {
	cases := []struct {
		name    string
		view    RoutingView
		want    []string
		wantNot []string
	}{
		{
			name: "empty view",
			view: RoutingView{},
			want: []string{"tokens not observed in this window"},
		},
		{
			name: "quota estimate printed after harness rows",
			view: RoutingView{
				ByHarness: map[string]RoutingRow{
					"claude-code": {Spawns: 1, BelowSpawns: 1, Billable: 1700, BelowBillable: 1700, CacheRead: 500},
					"codex":       {UnobservedSpawns: 1},
				},
				QuotaByHarness: map[string]QuotaUnits{
					"claude-code": {Session: 3400, Ran: 1700},
				},
			},
			want: []string{
				"  claude-code\n",
				"    ran below the session model",
				"    cache read  500 (apart from billable)\n    quota estimate: 50.0% lower than on the session model (1.7k of 3.4k mid-tier units)",
				"  codex\n    unobserved  1 spawn (no model could be ranked)",
			},
			wantNot: []string{"  codex\n    unobserved  1 spawn (no model could be ranked)\n    quota estimate:"},
		},
		{
			name: "higher quota estimate when spawn ran above session tier",
			view: RoutingView{
				ByHarness:      map[string]RoutingRow{"claude-code": {Spawns: 1, Billable: 1700}},
				QuotaByHarness: map[string]QuotaUnits{"claude-code": {Session: 85, Ran: 3400}},
			},
			want: []string{"    quota estimate: 3900.0% higher than on the session model"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var b strings.Builder
			PrintRouting(&b, tc.view)
			out := b.String()
			for _, w := range tc.want {
				if !strings.Contains(out, w) {
					t.Errorf("missing %q in:\n%s", w, out)
				}
			}
			for _, w := range tc.wantNot {
				if strings.Contains(out, w) {
					t.Errorf("unexpected %q in:\n%s", w, out)
				}
			}
		})
	}
}
