// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry_test

import (
	"github.com/tiagovilasboas/downshift-ai/internal/telemetry"
	"github.com/tiagovilasboas/downshift-ai/internal/telemetry/readout"
	"testing"
)

// Claude Code stops the child before the Agent tool returns, so the usage row
// is written before the resolved row that links the agent to its decision.
func TestLinkLateUsage_UsageWrittenBeforeResolved(t *testing.T) {
	tok := int64(100)
	evs := []telemetry.Event{
		{CorrelationID: "d1", Timestamp: "2026-10-20T10:00:00Z", Harness: "claude-code", SessionID: "s", Outcome: "allow",
			Verdict: "DOWNSHIFT", FromModel: "claude-opus-4-8", ToModel: "claude-haiku-4-5", Tier: "small", Complexity: "TRIVIAL"},
		{CorrelationID: "u1", Timestamp: "2026-10-20T10:00:05Z", Harness: "claude-code", SessionID: "s", Outcome: telemetry.OutcomeUsage,
			AgentHash: "a1", FromModel: "unknown", ToModel: "claude-haiku-4-5", Verdict: "UNKNOWN", InputTokens: &tok},
		{CorrelationID: "r1", Timestamp: "2026-10-20T10:00:06Z", Harness: "claude-code", SessionID: "s", Outcome: telemetry.OutcomeResolved,
			AgentHash: "a1", LinkedDecision: "d1", FromModel: "claude-opus-4-8", ToModel: "claude-haiku-4-5"},
		{CorrelationID: "u2", Timestamp: "2026-10-20T10:01:00Z", Harness: "claude-code", SessionID: "s", Outcome: telemetry.OutcomeUsage,
			AgentHash: "other", FromModel: "unknown", ToModel: "claude-haiku-4-5", InputTokens: &tok},
	}
	telemetry.LinkLateUsage(evs)
	u := evs[1]
	if u.LinkedDecision != "d1" || u.FromModel != "claude-opus-4-8" || u.Verdict != "DOWNSHIFT" || u.Tier != "small" || u.Complexity != "TRIVIAL" {
		t.Errorf("usage not joined to its decision: %+v", u)
	}
	if evs[3].LinkedDecision != "" || evs[3].FromModel != "unknown" {
		t.Errorf("a usage with no resolved row must stay as written: %+v", evs[3])
	}
	// A usage that already names a decision is never rewired.
	evs[1].LinkedDecision = "keep"
	evs[1].FromModel = "x"
	telemetry.LinkLateUsage(evs)
	if evs[1].LinkedDecision != "keep" || evs[1].FromModel != "x" {
		t.Errorf("an existing link was overwritten: %+v", evs[1])
	}
}

// The join feeds the tokens saved view: a child that ran cheaper than requested
// counts as saved even though its usage was logged first.
func TestRoutingTokens_CountsUsageLoggedBeforeResolved(t *testing.T) {
	tok := int64(1000)
	evs := []telemetry.Event{
		{CorrelationID: "d1", Timestamp: "2026-10-20T10:00:00Z", Harness: "claude-code", SessionID: "s", Outcome: telemetry.OutcomeRewriteEmitted,
			Verdict: "DOWNSHIFT", FromModel: "claude-opus-4-8", ToModel: "claude-haiku-4-5"},
		{CorrelationID: "u1", Timestamp: "2026-10-20T10:00:05Z", Harness: "claude-code", SessionID: "s", Outcome: telemetry.OutcomeUsage,
			AgentHash: "a1", FromModel: "unknown", ToModel: "claude-haiku-4-5", InputTokens: &tok},
		{CorrelationID: "r1", Timestamp: "2026-10-20T10:00:06Z", Harness: "claude-code", SessionID: "s", Outcome: telemetry.OutcomeResolved,
			AgentHash: "a1", LinkedDecision: "d1", FromModel: "claude-opus-4-8", ToModel: "claude-haiku-4-5"},
	}
	telemetry.LinkLateUsage(evs)
	if got := readout.RoutingTokens(evs).ByHarness["claude-code"]; got.FallbackBelowBillable != 1000 {
		t.Errorf("below requested = %d, want 1000 (%+v)", got.FallbackBelowBillable, got)
	}
}
