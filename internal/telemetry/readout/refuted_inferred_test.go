// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package readout

import (
	"github.com/tiagovilasboas/downshift-ai/internal/telemetry"
	"testing"
)

func refBool(b bool) *bool { return &b }
func refShift(id, session string) telemetry.Event {
	return telemetry.Event{CorrelationID: id, Timestamp: "2026-10-15T10:00:00Z", Harness: "claude-code", SessionID: session,
		Verdict: "DOWNSHIFT", Outcome: telemetry.OutcomeRewriteEmitted, FromModel: "claude-opus-5-5", ToModel: "claude-haiku-5-5",
		Tier: "SMALL", Complexity: "TRIVIAL", QuotaStatus: QuotaAvailable}
}

// a rewrite the harness refused (resolved model != rewrite) must not be a saving.
func TestAggregate_RefutedRewriteSavesNothing(t *testing.T) {
	events := []telemetry.Event{
		refShift("d1", "s1"),
		{Outcome: telemetry.OutcomeResolved, Harness: "claude-code", SessionID: "s1", LinkedDecision: "d1",
			Timestamp: "2026-10-15T10:00:01Z", ToModel: "claude-opus-5-5", RewriteHonored: refBool(false)},
	}
	s := Aggregate(events)
	if s.Downshifted != 0 {
		t.Fatalf("refuted rewrite counted: Downshifted=%d", s.Downshifted)
	}
}

// a resolved record that refutes the rewrite must beat the later-spawn heuristic.
func TestCountInferredHonored_RefutedBeatsLaterSpawn(t *testing.T) {
	later := refShift("d2", "s1")
	later.FromModel, later.Verdict, later.Outcome, later.Timestamp = "claude-haiku-5-5", "OK", telemetry.OutcomeAllow, "2026-10-15T10:05:00Z"
	events := []telemetry.Event{
		refShift("d1", "s1"),
		{Outcome: telemetry.OutcomeResolved, Harness: "claude-code", SessionID: "s1", LinkedDecision: "d1",
			Timestamp: "2026-10-15T10:00:01Z", ToModel: "claude-opus-5-5", RewriteHonored: refBool(false)},
		later,
	}
	shifted, honored := CountInferredHonored(events)
	if honored != 0 {
		t.Fatalf("rewrite refuted by resolved record still counted honored: shifted=%d honored=%d", shifted, honored)
	}
}

// real saving whose baseline was inferred from settings must not be presented as measured.
