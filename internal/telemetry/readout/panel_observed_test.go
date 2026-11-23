// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package readout

import (
	"encoding/json"
	"github.com/tiagovilasboas/downshift-ai/internal/telemetry"
	"strings"
	"testing"
)

func TestPanelObservedModel(t *testing.T) {
	const decided = "claude-sonnet-5-5"
	session := strings.Repeat("b", 64)
	decision := func() telemetry.Event {
		return telemetry.Event{
			Harness: "claude-code", SessionID: session, Outcome: telemetry.OutcomeRewriteEmitted, Tier: "MID",
			FromModel: "claude-opus-5-5", ToModel: decided, CorrelationID: "corr-1",
			Timestamp: "2026-10-20T10:00:00Z", ToolUseHash: ToolUseKey("toolu_1"),
		}
	}
	link := func(outcome, model string) telemetry.Event {
		return telemetry.Event{
			Harness: "claude-code", SessionID: session, Outcome: outcome, LinkedDecision: "corr-1",
			FromModel: decided, ToModel: model, Timestamp: "2026-10-20T10:00:05Z",
		}
	}
	cases := []struct {
		name   string
		events []telemetry.Event
		want   string
	}{
		{"no observed record", []telemetry.Event{decision()}, ""},
		{"resolved equals decided", []telemetry.Event{decision(), link(telemetry.OutcomeResolved, decided)}, ""},
		{"resolved differs", []telemetry.Event{decision(), link(telemetry.OutcomeResolved, "claude-opus-5-5")}, "claude-opus-5-5"},
		{"usage differs", []telemetry.Event{decision(), link(telemetry.OutcomeUsage, "claude-haiku-5-5")}, "claude-haiku-5-5"},
		{"usage equals decided", []telemetry.Event{decision(), link(telemetry.OutcomeUsage, decided)}, ""},
		{"dated id is the same model", []telemetry.Event{decision(), link(telemetry.OutcomeResolved, "claude-sonnet-5-5-20261001")}, ""},
		{"unknown observed model", []telemetry.Event{decision(), link(telemetry.OutcomeResolved, "unknown")}, ""},
		{"resolved wins over usage", []telemetry.Event{decision(), link(telemetry.OutcomeUsage, "claude-haiku-5-5"), link(telemetry.OutcomeResolved, decided)}, ""},
		{"usage after resolved keeps resolved", []telemetry.Event{decision(), link(telemetry.OutcomeResolved, "claude-opus-5-5"), link(telemetry.OutcomeUsage, "claude-haiku-5-5")}, "claude-opus-5-5"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			v := BuildPanel(c.events, session)
			if len(v.Spawns) != 1 {
				t.Fatalf("spawns = %d, want 1", len(v.Spawns))
			}
			sp := v.Spawns[0]
			if sp.ToModel != decided {
				t.Fatalf("to_model = %q, want the decided %q", sp.ToModel, decided)
			}
			if sp.ObservedModel != c.want {
				t.Fatalf("observed_model = %q, want %q", sp.ObservedModel, c.want)
			}
			raw, err := json.Marshal(sp)
			if err != nil {
				t.Fatal(err)
			}
			has := strings.Contains(string(raw), `"observed_model"`)
			if has != (c.want != "") {
				t.Fatalf("observed_model in JSON = %v, want %v: %s", has, c.want != "", raw)
			}
		})
	}
}
