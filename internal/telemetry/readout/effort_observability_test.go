// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package readout

import (
	"bytes"
	"encoding/json"
	"github.com/tiagovilasboas/downshift-ai/internal/telemetry"
	"strings"
	"testing"
	"time"
)

func TestSessionTree_EffortObservability(t *testing.T) {
	cases := []struct {
		name         string
		reqEffort    string
		finalEffort  string
		usageEffort  string
		wantEffort   string
		wantModelCol string
		wantJSONKey  bool
	}{
		{
			name:         "decision final effort shown",
			reqEffort:    "high",
			finalEffort:  "low",
			wantEffort:   "low",
			wantModelCol: "haiku [effort: low]",
			wantJSONKey:  true,
		},
		{
			name:         "usage final effort wins over decision",
			reqEffort:    "high",
			finalEffort:  "low",
			usageEffort:  "medium",
			wantEffort:   "medium",
			wantModelCol: "haiku [effort: medium]",
			wantJSONKey:  true,
		},
		{
			name:         "falls back to requested effort when final is unknown",
			reqEffort:    "high",
			finalEffort:  "unknown",
			wantEffort:   "high",
			wantModelCol: "haiku [effort: high]",
			wantJSONKey:  true,
		},
		{
			name:         "graceful fallback when effort unset",
			reqEffort:    "",
			finalEffort:  "",
			wantEffort:   "",
			wantModelCol: "haiku  ",
			wantJSONKey:  false,
		},
		{
			name:         "graceful fallback when effort unknown or unobserved",
			reqEffort:    telemetry.EffortUnobserved,
			finalEffort:  "unknown",
			wantEffort:   "",
			wantModelCol: "haiku  ",
			wantJSONKey:  false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := td("d1", sessA, 0, "claude-haiku-5-5", telemetry.ReasonClassifierConfident)
			d.RequestedEffort = tc.reqEffort
			d.FinalEffort = tc.finalEffort
			evs := []telemetry.Event{d}
			if tc.usageEffort != "" {
				u := tuse("d1", sessA, time.Second, "claude-haiku-5-5", 0.01, 0.10)
				u.FinalEffort = tc.usageEffort
				evs = append(evs, u)
			}

			view := BuildSessionTree(evs, sessA, TreeOptions{})
			if len(view.Rows) != 1 {
				t.Fatalf("got %d rows, want 1", len(view.Rows))
			}
			if got := view.Rows[0].Effort; got != tc.wantEffort {
				t.Errorf("row.Effort = %q, want %q", got, tc.wantEffort)
			}
			raw, err := json.Marshal(view.Rows[0])
			if err != nil {
				t.Fatalf("json.Marshal: %v", err)
			}
			hasKey := strings.Contains(string(raw), `"effort"`)
			if hasKey != tc.wantJSONKey {
				t.Errorf("json has effort key = %v, want %v (%s)", hasKey, tc.wantJSONKey, raw)
			}
			rendered := view.Render()
			if !strings.Contains(rendered, tc.wantModelCol) {
				t.Errorf("rendered missing %q:\n%s", tc.wantModelCol, rendered)
			}
			if !tc.wantJSONKey && strings.Contains(rendered, "[effort:") {
				t.Errorf("rendered unexpectedly includes [effort:]:\n%s", rendered)
			}
		})
	}
}

func TestPrintStats_EffortMixDetail(t *testing.T) {
	now := time.Now().UTC()
	mk := func(id, req, fin string) telemetry.Event {
		return telemetry.Event{
			CorrelationID: id, Timestamp: now.Format(time.RFC3339Nano),
			Harness: "claude-code", Outcome: telemetry.OutcomeRewriteEmitted,
			Complexity: "SIMPLE", FromModel: "claude-opus-5-5", ToModel: "claude-haiku-5-5",
			Tier: "SMALL", Verdict: "DOWNSHIFT",
			RequestedEffort: req, FinalEffort: fin,
		}
	}

	cases := []struct {
		name       string
		events     []telemetry.Event
		detail     bool
		wantHeader bool
		wantLines  []string
	}{
		{
			name: "detail shows sorted effort breakdown",
			events: []telemetry.Event{
				mk("e1", "high", "low"),
				mk("e2", "medium", ""),
				mk("e3", "high", "high"),
				mk("e4", telemetry.EffortUnobserved, "unknown"),
			},
			detail:     true,
			wantHeader: true,
			wantLines:  []string{"By effort", "low", "medium", "high"},
		},
		{
			name: "detail omits By effort when no known effort",
			events: []telemetry.Event{
				mk("e1", "", ""),
				mk("e2", telemetry.EffortUnobserved, "unknown"),
			},
			detail:     true,
			wantHeader: false,
		},
		{
			name: "short stats hides By effort even when present",
			events: []telemetry.Event{
				mk("e1", "high", "low"),
			},
			detail:     false,
			wantHeader: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var b bytes.Buffer
			PrintStats(tc.events, StatsOptions{Days: 7, Detail: tc.detail}, &b)
			out := b.String()
			if got := strings.Contains(out, "By effort"); got != tc.wantHeader {
				t.Errorf("contains 'By effort' = %v, want %v\n%s", got, tc.wantHeader, out)
			}
			for _, w := range tc.wantLines {
				if !strings.Contains(out, w) {
					t.Errorf("output missing %q:\n%s", w, out)
				}
			}
		})
	}
}
