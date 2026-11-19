// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package cursor_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/adapters/cursor"
	"github.com/tiagovilasboas/downshift-ai/internal/telemetry"
)

func TestObservePromptSkills(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DOWNSHIFT_STATE_DIR", dir)
	logPath := filepath.Join(dir, "events.jsonl")
	t.Setenv("DOWNSHIFT_EVENT_LOG", logPath)

	// beforeSubmitPrompt with /ponytail and /downshift-audit records both.
	cursor.ObservePromptSkills(cursor.Event{
		HookEventName:  "beforeSubmitPrompt",
		ConversationID: "raw-cursor-conv",
		Prompt:         "Please apply /ponytail and /downshift-audit on secret-project",
	}, "v-test")

	// beforeSubmitPrompt with $ponytail and session_id uses session_id.
	cursor.ObservePromptSkills(cursor.Event{
		HookEventName: "beforeSubmitPrompt",
		SessionID:     "raw-cursor-sess",
		Prompt:        "Use $ponytail on this bug",
	}, "v-test")

	// Ordinary slash commands, env vars, private names, and sessionStart are ignored.
	cursor.ObservePromptSkills(cursor.Event{
		HookEventName: "beforeSubmitPrompt",
		SessionID:     "raw-cursor-sess",
		Prompt:        "/clear /help $HOME /private-skill",
	}, "v-test")
	cursor.ObservePromptSkills(cursor.Event{
		HookEventName: "sessionStart",
		SessionID:     "raw-cursor-sess",
		Prompt:        "/ponytail",
	}, "v-test")

	b, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(b)
	for _, forbidden := range []string{"raw-cursor-conv", "raw-cursor-sess", "secret-project", "private-skill"} {
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
	wantConvHash := telemetry.HashSessionID("raw-cursor-conv")
	wantSessHash := telemetry.HashSessionID("raw-cursor-sess")
	want := []struct {
		sess, skill string
	}{
		{wantConvHash, "ponytail"},
		{wantConvHash, "downshift-audit"},
		{wantSessHash, "ponytail"},
	}
	for i, w := range want {
		ev := events[i]
		if ev.Outcome != telemetry.OutcomeSkillUsed || ev.Harness != "cursor" || ev.SessionID != w.sess || ev.Agent != w.skill {
			t.Errorf("event[%d] = %+v, want outcome=%s harness=cursor session=%s agent=%s",
				i, ev, telemetry.OutcomeSkillUsed, w.sess, w.skill)
		}
	}
}
