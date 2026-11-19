// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package kiro

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/paths"
	"github.com/tiagovilasboas/downshift-ai/internal/telemetry"
)

func TestHandlePostToolUse_SkillObservation(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv(paths.EnvStateDir, stateDir)
	logPath := filepath.Join(stateDir, "events.jsonl")
	t.Setenv("DOWNSHIFT_EVENT_LOG", logPath)

	payloads := [][]byte{
		[]byte(`{"hook_event_name":"PostToolUse","session_id":"raw-kiro-sess","tool_name":"read_file","tool_input":{"path":"/Users/alice/.kiro/skills/ponytail/SKILL.md"},"tool_response":{"content":"skill body"}}`),
		[]byte(`{"hook_event_name":"PostToolUse","session_id":"raw-kiro-sess","tool_name":"read_file","tool_input":{"file_path":"/Users/alice/.kiro/skills/downshift-audit/SKILL.md"}}`),
		[]byte(`{"hook_event_name":"PostToolUse","session_id":"raw-kiro-sess","tool_name":"read_file","tool_input":{"path":"/Users/alice/.kiro/skills/secret-corp-skill/SKILL.md"}}`),
		[]byte(`{"hook_event_name":"PostToolUse","session_id":"raw-kiro-sess","tool_name":"read_file","tool_input":{"path":"/Users/alice/repo/README.md"}}`),
	}

	for _, raw := range payloads {
		out, _, linked := HandlePostToolUse(raw, "v-test", nil)
		if linked {
			t.Errorf("read_file PostToolUse must not report linked=true")
		}
		if out != (struct{}{}) {
			t.Errorf("read_file PostToolUse must return neutral {}, got %#v", out)
		}
	}

	b, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(b)
	for _, forbidden := range []string{"raw-kiro-sess", "/Users/alice", "secret-corp-skill", "SKILL.md"} {
		if strings.Contains(text, forbidden) {
			t.Errorf("event log leaked %q:\n%s", forbidden, text)
		}
	}

	events, err := telemetry.ReadEventsFrom(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 3 {
		t.Fatalf("events = %d, want 3: %+v", len(events), events)
	}
	wantHash := telemetry.HashSessionID("raw-kiro-sess")
	wantSkills := []string{"ponytail", "downshift-audit", "other"}
	for i, ev := range events {
		if ev.Outcome != telemetry.OutcomeSkillUsed || ev.Harness != harnessID || ev.SessionID != wantHash || ev.Agent != wantSkills[i] {
			t.Errorf("event[%d] = %+v, want outcome=%s harness=%s session=%s agent=%s",
				i, ev, telemetry.OutcomeSkillUsed, harnessID, wantHash, wantSkills[i])
		}
	}
}
