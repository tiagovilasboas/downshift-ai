// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package childend_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/catalog"
	"github.com/tiagovilasboas/downshift-ai/internal/childend"
	"github.com/tiagovilasboas/downshift-ai/internal/hookport"
	"github.com/tiagovilasboas/downshift-ai/internal/telemetry"
)

func TestRecord_NoHarnessOrNoUsageRecordsNothing(t *testing.T) {
	t.Setenv("DOWNSHIFT_EVENT_LOG", filepath.Join(t.TempDir(), "events.jsonl"))
	res := childend.Record(hookport.ChildEnd{AgentKey: "a1", Usage: &hookport.TokenUsage{InputTokens: 5}}, "test", catalog.Load())
	if res.Usage != nil {
		t.Fatal("a ChildEnd with no harness was recorded")
	}
	res = childend.Record(hookport.ChildEnd{Harness: "codex", AgentKey: "a1", Source: hookport.SourceTranscript}, "test", catalog.Load())
	if res.Usage != nil || res.Note != "" {
		t.Fatalf("a stop with no usage recorded %+v", res)
	}
	// A transcript-sourced stop cannot be deduped or joined without an agent id.
	res = childend.Record(hookport.ChildEnd{Harness: "codex", Source: hookport.SourceTranscript, Usage: &hookport.TokenUsage{InputTokens: 5}}, "test", catalog.Load())
	if res.Usage != nil {
		t.Fatal("a stop with no agent id was priced")
	}
	if all, _ := telemetry.ReadEvents(); len(all) != 0 {
		t.Fatalf("%d events recorded", len(all))
	}
}

func TestLinkTargetBy_IsPerHarness(t *testing.T) {
	now := time.Now().UTC()
	sess := telemetry.HashSessionID("sess-1")
	prior := []telemetry.Event{
		{CorrelationID: "cx", Harness: "codex", Outcome: telemetry.OutcomeAllow, SessionID: sess, Timestamp: now.Add(-time.Minute).Format(time.RFC3339Nano)},
		{CorrelationID: "cc", Harness: "claude-code", Outcome: telemetry.OutcomeAllow, SessionID: sess, Timestamp: now.Add(-time.Second).Format(time.RFC3339Nano)},
	}
	if got := childend.LinkTargetBy("codex", prior, sess, "", "", now, telemetry.OutcomeUsage); got == nil || got.CorrelationID != "cx" {
		t.Fatalf("codex linked to %+v", got)
	}
	if got := childend.LinkTargetBy("cursor", prior, sess, "", "", now, telemetry.OutcomeUsage); got != nil {
		t.Fatalf("cursor linked to %+v", got)
	}
}

func TestRecord_CopiesComplexityAndRequestedEffortToObservationsAndUsage(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "events.jsonl")
	t.Setenv("DOWNSHIFT_EVENT_LOG", logPath)
	res := catalog.Load()
	sess := telemetry.HashSessionID("sess-effort")
	now := time.Now().UTC()

	decision := telemetry.Event{
		CorrelationID:   "dec-effort-1",
		Timestamp:       now.Add(-time.Second).Format(time.RFC3339Nano),
		Source:          "hook",
		Agent:           "subagent",
		Harness:         "claude-code",
		SessionID:       sess,
		Complexity:      "SIMPLE",
		FromModel:       "claude-sonnet-4-6",
		ToModel:         "claude-haiku-4-5",
		RequestedEffort: "high",
		FinalEffort:     "low",
		Verdict:         "DOWNSHIFT",
		Tier:            "SMALL",
		Outcome:         telemetry.OutcomeRewriteEmitted,
		ToolUseHash:     telemetry.HashSessionID("call-1"),
	}
	if err := telemetry.AppendTo(logPath, decision); err != nil {
		t.Fatal(err)
	}

	childend.Record(hookport.ChildEnd{
		Harness:       "claude-code",
		SessionID:     "sess-effort",
		ToolUseID:     "call-1",
		AgentKey:      "agent-1",
		ResolvedModel: "claude-haiku-4-5",
		Status:        telemetry.ChildCompleted,
		Usage:         &hookport.TokenUsage{InputTokens: 100, OutputTokens: 20},
		Source:        hookport.SourcePayload,
		IsChild:       true,
	}, "test", res)

	events, err := telemetry.ReadEventsFrom(logPath)
	if err != nil {
		t.Fatal(err)
	}
	var seenResolved, seenEnded, seenUsage bool
	for _, ev := range events {
		switch ev.Outcome {
		case telemetry.OutcomeResolved:
			seenResolved = true
		case telemetry.OutcomeChildEnded:
			seenEnded = true
		case telemetry.OutcomeUsage:
			seenUsage = true
		default:
			continue
		}
		if ev.Complexity != "SIMPLE" || ev.RequestedEffort != "high" {
			t.Errorf("%s event complexity=%q requested_effort=%q, want SIMPLE and high", ev.Outcome, ev.Complexity, ev.RequestedEffort)
		}
	}
	if !seenResolved || !seenEnded || !seenUsage {
		t.Fatalf("missing events: resolved=%v ended=%v usage=%v", seenResolved, seenEnded, seenUsage)
	}
}
