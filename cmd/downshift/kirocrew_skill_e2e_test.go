// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/telemetry"
	"github.com/tiagovilasboas/downshift-ai/internal/testenv"
)

func TestKiroCrewE2E_SkillObservationViaPreToolUse(t *testing.T) {
	testenv.SetHome(t, t.TempDir())
	t.Setenv("DOWNSHIFT_STATE_DIR", t.TempDir())
	logPath := filepath.Join(t.TempDir(), "events.jsonl")
	t.Setenv("DOWNSHIFT_EVENT_LOG", logPath)

	payloads := []string{
		`{"hook_event_name":"PreToolUse","session_id":"kc-raw-1","tool_name":"fs_read","tool_input":{"operations":[{"mode":"Line","path":"/Users/alice/.kiro/skills/ponytail/SKILL.md"}]}}`,
		`{"hook_event_name":"PreToolUse","session_id":"kc-raw-1","tool_name":"read_file","tool_input":{"path":"/Users/alice/.kiro/skills/secret-skill/SKILL.md"}}`,
		`{"hook_event_name":"PreToolUse","session_id":"kc-raw-1","tool_name":"fs_read","tool_input":{"operations":[{"mode":"Line","path":"/repo/main.go"}]}}`,
	}

	for _, p := range payloads {
		var rc int
		stdout := captureStdout(func() {
			rc = harnessRegistry["kirocrew"].Hook(strings.NewReader(p), cmdCat)
		})
		if rc != 0 || stdout != "" {
			t.Fatalf("rc=%d stdout=%q, want 0 and empty", rc, stdout)
		}
	}

	b, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(b)
	for _, forbidden := range []string{"kc-raw-1", "/Users/alice", "secret-skill", "SKILL.md"} {
		if strings.Contains(text, forbidden) {
			t.Errorf("event log leaked %q:\n%s", forbidden, text)
		}
	}

	events, err := telemetry.ReadEventsFrom(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("events = %d, want 2: %+v", len(events), events)
	}
	wantHash := telemetry.HashSessionID("kc-raw-1")
	wantSkills := []string{"ponytail", "other"}
	for i, ev := range events {
		if ev.Outcome != telemetry.OutcomeSkillUsed || ev.Harness != "kirocrew" || ev.SessionID != wantHash || ev.Agent != wantSkills[i] {
			t.Errorf("event[%d] = %+v, want outcome=%s harness=kirocrew session=%s agent=%s",
				i, ev, telemetry.OutcomeSkillUsed, wantHash, wantSkills[i])
		}
	}
}
