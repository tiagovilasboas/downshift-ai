// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package childend_test

import (
	"github.com/tiagovilasboas/downshift-ai/internal/telemetry/readout"
	"path/filepath"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/catalog"
	"github.com/tiagovilasboas/downshift-ai/internal/childend"
	"github.com/tiagovilasboas/downshift-ai/internal/hookport"
	"github.com/tiagovilasboas/downshift-ai/internal/telemetry"
)

// A background child that died on an API error or a cancel has no usage, so
// its stop used to record nothing and the task folded to unknown. The stop now
// records the end through the agent id, once.
func TestRecord_StopWithoutUsageRecordsTheEnd(t *testing.T) {
	cases := []struct {
		name   string
		status telemetry.ChildStatus
		want   readout.TaskOutcome
	}{
		{"api error", telemetry.ChildError, readout.TaskErrored},
		{"cancelled", telemetry.ChildInterrupted, readout.TaskErrored},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("DOWNSHIFT_EVENT_LOG", filepath.Join(t.TempDir(), "events.jsonl"))
			r := catalog.Load()
			const harness, session, call, agent = "claude-code", "sess-1", "toolu_1", "agent-1"
			telemetry.Record(telemetry.Event{
				CorrelationID: "d1", Timestamp: time.Now().UTC().Add(-time.Minute).Format(time.RFC3339Nano),
				Harness: harness, Outcome: telemetry.OutcomeRewriteEmitted, Verdict: "DOWNSHIFT",
				FromModel: "claude-opus-5-5", ToModel: "claude-sonnet-5-5",
				SessionID: telemetry.HashSessionID(session), ToolUseHash: telemetry.HashSessionID(call),
			})
			childend.Record(hookport.ChildEnd{
				Harness: harness, AgentKey: agent, ToolUseID: call, SessionID: session,
				ResolvedModel: "claude-sonnet-5-5", Status: telemetry.ChildLaunched,
				Source: hookport.SourcePayload, IsChild: true,
			}, "test", r)

			stop := hookport.ChildEnd{
				Harness: harness, AgentKey: agent, SessionID: session, Status: tc.status,
				Source: hookport.SourceTranscript, IsChild: true,
			}
			for range 2 { // a repeated stop must not add a second end
				if res := childend.Record(stop, "test", r); res.Usage != nil {
					t.Fatalf("a stop without usage priced something: %+v", res.Usage)
				}
			}
			events, _ := telemetry.ReadEvents()
			ends := 0
			for _, e := range events {
				if e.Outcome == telemetry.OutcomeChildEnded && e.ChildStatus == tc.status {
					ends++
				}
			}
			if ends != 1 {
				t.Fatalf("recorded %d %s ends, want 1", ends, tc.status)
			}
			tasks := readout.BuildTasks(events)
			if len(tasks) != 1 || tasks[0].Outcome() != tc.want {
				t.Fatalf("tasks = %+v, want one %s", tasks, tc.want)
			}
		})
	}
}

// A stop with no launch observation has nothing to link to: nothing is written.
func TestRecord_StopEndWithoutLaunchIsDropped(t *testing.T) {
	t.Setenv("DOWNSHIFT_EVENT_LOG", filepath.Join(t.TempDir(), "events.jsonl"))
	childend.Record(hookport.ChildEnd{
		Harness: "claude-code", AgentKey: "agent-x", SessionID: "s", Status: telemetry.ChildError,
		Source: hookport.SourceTranscript, IsChild: true,
	}, "test", catalog.Load())
	if events, _ := telemetry.ReadEvents(); len(events) != 0 {
		t.Fatalf("unlinked stop wrote %d events", len(events))
	}
}
