// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/adapters/cursor"
	"github.com/tiagovilasboas/downshift-ai/internal/telemetry"
)

func touchWith(t *testing.T, in io.Reader) (int, string) {
	t.Helper()
	var out bytes.Buffer
	rc := runCursorTouch(in, &out)
	return rc, out.String()
}

func TestCursorTouchFailsOpen(t *testing.T) {
	calls := 0
	prev := cursorRemember
	cursorRemember = func(cursor.Event) { calls++ }
	t.Cleanup(func() { cursorRemember = prev })

	t.Run("valid input is recorded", func(t *testing.T) {
		calls = 0
		rc, out := touchWith(t, strings.NewReader(`{"model":"x"}`))
		if rc != 0 || out != "{}\n" || calls != 1 {
			t.Fatalf("rc=%d out=%q calls=%d", rc, out, calls)
		}
	})
	t.Run("oversized input is dropped", func(t *testing.T) {
		calls = 0
		big := `{"model":"` + strings.Repeat("a", 1<<20) + `"}`
		rc, out := touchWith(t, strings.NewReader(big))
		if rc != 0 || out != "{}\n" || calls != 0 {
			t.Fatalf("rc=%d out=%q calls=%d", rc, out, calls)
		}
	})
	t.Run("slow input times out", func(t *testing.T) {
		calls = 0
		prevT := hookReadTimeout
		hookReadTimeout = 20 * time.Millisecond
		t.Cleanup(func() { hookReadTimeout = prevT })
		pr, pw := io.Pipe()
		t.Cleanup(func() { _ = pw.Close() })
		rc, out := touchWith(t, pr)
		if rc != 0 || out != "{}\n" || calls != 0 {
			t.Fatalf("rc=%d out=%q calls=%d", rc, out, calls)
		}
	})
	t.Run("panic exits 0 with unchanged response", func(t *testing.T) {
		cursorRemember = func(cursor.Event) { panic("boom") }
		rc, out := touchWith(t, strings.NewReader(`{"model":"x"}`))
		if rc != 0 || out != "{}\n" {
			t.Fatalf("rc=%d out=%q", rc, out)
		}
	})
}

func TestCursorTouchRecordsPromptSkill(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DOWNSHIFT_STATE_DIR", dir)
	logPath := filepath.Join(dir, "events.jsonl")
	t.Setenv("DOWNSHIFT_EVENT_LOG", logPath)

	payload := `{"hook_event_name":"beforeSubmitPrompt","session_id":"cursor-raw-1","model_id":"claude-sonnet-5-5","prompt":"use /ponytail on my secret file"}`
	rc, out := touchWith(t, strings.NewReader(payload))
	if rc != 0 || out != "{}\n" {
		t.Fatalf("rc=%d out=%q, want 0 and {}", rc, out)
	}
	events, err := telemetry.ReadEventsFrom(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Outcome != telemetry.OutcomeSkillUsed ||
		events[0].Harness != "cursor" || events[0].Agent != "ponytail" ||
		events[0].SessionID != telemetry.HashSessionID("cursor-raw-1") {
		t.Fatalf("events = %+v", events)
	}
}
