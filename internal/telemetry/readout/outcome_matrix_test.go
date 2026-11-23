// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package readout

import (
	"fmt"
	"github.com/tiagovilasboas/downshift-ai/internal/telemetry"
	"testing"
)

// matrixSpawn is a decision, one usage record and a completed end. A
// rewrite_emitted decision with from != to is an applied rewrite.
func matrixSpawn(id, outcome, agent, parent, from, to, verification string) []telemetry.Event {
	in, out := int64(100), int64(50)
	ts := fmt.Sprintf("2026-01-01T00:00:%02dZ", len(id))
	return []telemetry.Event{
		{CorrelationID: id, Timestamp: ts, Harness: "claude-code", SessionID: "s" + id, Outcome: outcome, FromModel: from, ToModel: to, ParentModel: parent, AgentType: agent},
		{CorrelationID: id + "u", Timestamp: ts, Harness: "claude-code", Outcome: telemetry.OutcomeUsage, LinkedDecision: id, FromModel: from, ToModel: to,
			InputTokens: &in, OutputTokens: &out, Verification: verification},
		{CorrelationID: id + "e", Timestamp: ts, Harness: "claude-code", Outcome: telemetry.OutcomeChildEnded, LinkedDecision: id, ChildStatus: telemetry.ChildCompleted},
	}
}

func matrixRow(t *testing.T, rows []OutcomeMatrixRow, origin telemetry.SpawnOrigin, tier RanTier, agent string) OutcomeMatrixRow {
	t.Helper()
	for _, r := range rows {
		if r.Origin == origin && r.Tier == tier && r.AgentType == agent {
			return r
		}
	}
	t.Fatalf("no row %s/%s/%s in %+v", origin, tier, agent, rows)
	return OutcomeMatrixRow{}
}

func TestOutcomeMatrix_EachOrigin(t *testing.T) {
	const opus, sonnet = "claude-opus-4-8", "claude-sonnet-4-6"
	var ev []telemetry.Event
	ev = append(ev, matrixSpawn("a", telemetry.OutcomeRewriteEmitted, "Explore", opus, opus, sonnet, "passed")...)
	ev = append(ev, matrixSpawn("bb", telemetry.OutcomeAllow, "Explore", opus, sonnet, sonnet, "none")...)
	ev = append(ev, matrixSpawn("ccc", telemetry.OutcomeAllow, "", opus, opus, opus, "failed")...)
	ev = append(ev, matrixSpawn("dddd", telemetry.OutcomeAllow, "Plan", "", opus, opus, "")...)
	rows := BuildReport(ev, 0).OutcomeMatrix
	if len(rows) != 4 {
		t.Fatalf("rows = %+v", rows)
	}
	if r := matrixRow(t, rows, telemetry.OriginDownshiftRewrite, RanMid, "Explore"); r.N != 1 || r.Outcomes.AcceptedChecked != 1 || r.NoCheckShare != 0 {
		t.Fatalf("rewrite %+v", r)
	}
	if r := matrixRow(t, rows, telemetry.OriginOrchestratorBelow, RanMid, "Explore"); r.N != 1 || r.Outcomes.AcceptedUnchecked != 1 || r.NoCheckShare != 1 {
		t.Fatalf("below %+v", r)
	}
	if r := matrixRow(t, rows, telemetry.OriginOrchestratorSession, RanFrontier, UntypedAgent); r.Outcomes.CheckFailed != 1 {
		t.Fatalf("session %+v", r)
	}
	if r := matrixRow(t, rows, telemetry.OriginUnattributed, RanFrontier, "Plan"); r.N != 1 || r.Outcomes.AcceptedUnchecked != 1 {
		t.Fatalf("unattributed %+v", r)
	}
}

func TestOutcomeMatrix_ResumedChildCountsOnce(t *testing.T) {
	const opus, sonnet = "claude-opus-4-8", "claude-sonnet-4-6"
	ev := matrixSpawn("a", telemetry.OutcomeAllow, "Explore", opus, opus, sonnet, "passed")
	second := ev[1]
	second.CorrelationID = "u2"
	ev = append(ev, second)
	rows := BuildReport(ev, 0).OutcomeMatrix
	if len(rows) != 1 || rows[0].N != 1 {
		t.Fatalf("rows = %+v", rows)
	}
}

func TestOutcomeMatrix_NoUsageIsUnattributedUnknownTier(t *testing.T) {
	dec := telemetry.Event{CorrelationID: "a", Timestamp: "2026-01-01T00:00:00Z", Harness: "claude-code", SessionID: "s", Outcome: telemetry.OutcomeAllow}
	rows := BuildReport([]telemetry.Event{dec}, 0).OutcomeMatrix
	if len(rows) != 1 || rows[0].Origin != telemetry.OriginUnattributed || rows[0].Tier != RanUnknown || rows[0].Outcomes.Unknown != 1 {
		t.Fatalf("rows = %+v", rows)
	}
}
