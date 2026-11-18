// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package claudecode_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/adapters/claudecode"
	"github.com/tiagovilasboas/downshift-ai/internal/telemetry"
)

func TestHandlePostToolUse_SkillObservation(t *testing.T) {
	dir := isolateDownshiftState(t)
	logPath := filepath.Join(dir, "events.jsonl")
	t.Setenv("DOWNSHIFT_EVENT_LOG", logPath)

	payloads := [][]byte{
		[]byte(`{"hook_event_name":"PostToolUse","session_id":"raw-sess-1","tool_name":"Skill","tool_input":{"skill":"ponytail","args":"secret args"}}`),
		[]byte(`{"hook_event_name":"PostToolUse","session_id":"raw-sess-1","tool_name":"Skill","tool_input":{"skill":"superpowers:brainstorming"}}`),
		[]byte(`{"hook_event_name":"PostToolUse","session_id":"raw-sess-1","tool_name":"Skill","tool_input":{"skill":"sk-ant-SECRETVALUE123"}}`),
	}

	for _, raw := range payloads {
		out, _, linked := claudecode.HandlePostToolUse(raw, "v-test", cat)
		if linked {
			t.Errorf("Skill PostToolUse must not report linked=true")
		}
		if out.SystemMessage != "" || out.HookSpecificOutput != nil {
			t.Errorf("Skill PostToolUse must return neutral output, got %+v", out)
		}
	}

	b, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(b)
	for _, forbidden := range []string{"raw-sess-1", "secret args", "sk-ant-SECRETVALUE123"} {
		if strings.Contains(text, forbidden) {
			t.Errorf("event log leaked %q:\n%s", forbidden, text)
		}
	}

	events, err := telemetry.ReadEventsFrom(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 3 {
		t.Fatalf("events = %d, want 3", len(events))
	}
	wantHash := telemetry.HashSessionID("raw-sess-1")
	wantSkills := []string{"ponytail", "superpowers:brainstorming", "other"}
	for i, ev := range events {
		if ev.Outcome != telemetry.OutcomeSkillUsed || ev.Harness != "claude-code" || ev.SessionID != wantHash || ev.Agent != wantSkills[i] {
			t.Errorf("event[%d] = %+v, want outcome=%s harness=claude-code session=%s agent=%s",
				i, ev, telemetry.OutcomeSkillUsed, wantHash, wantSkills[i])
		}
	}
}
