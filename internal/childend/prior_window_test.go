// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package childend_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/catalog"
	"github.com/tiagovilasboas/downshift-ai/internal/childend"
	"github.com/tiagovilasboas/downshift-ai/internal/hookport"
	"github.com/tiagovilasboas/downshift-ai/internal/telemetry"
)

const filler = `{"correlation_id":"f","timestamp":"2020-01-01T00:00:00Z","harness":"other","outcome":"allow"}` + "\n"

// appendFiller pads the log past the 4 MiB read window with old lines of
// another harness.
func appendFiller(t *testing.T, path string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.WriteString(strings.Repeat(filler, 50000)); err != nil { // about 4.6 MB
		t.Fatal(err)
	}
}

func decision(call string) telemetry.Event {
	return telemetry.Event{
		CorrelationID: "d-" + call, Timestamp: time.Now().UTC().Add(-time.Minute).Format(time.RFC3339Nano),
		Harness: "claude-code", Outcome: telemetry.OutcomeAllow, Verdict: "OK",
		FromModel: "claude-sonnet-5-5", ToModel: "claude-sonnet-5-5",
		SessionID: telemetry.HashSessionID("sess"), ToolUseHash: telemetry.HashSessionID(call),
	}
}

func spawnEnd(call string) hookport.ChildEnd {
	return hookport.ChildEnd{
		Harness: "claude-code", AgentKey: "agent-" + call, ToolUseID: call, SessionID: "sess",
		Model: "claude-sonnet-5-5", Usage: &hookport.TokenUsage{InputTokens: 1000, OutputTokens: 200},
		Source: hookport.SourcePayload, IsChild: true,
	}
}

func countUsage(t *testing.T) int {
	t.Helper()
	all, err := telemetry.ReadEvents()
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, ev := range all {
		if ev.Outcome == telemetry.OutcomeUsage {
			n++
		}
	}
	return n
}

func TestRecord_TailWindow(t *testing.T) {
	cases := []struct {
		name       string
		padAfter   bool // pad the log after the decision (decision falls out of the window)
		padBefore  bool // pad the log before the decision (decision stays in the window)
		repeat     bool // the same call id ends twice
		wantLinked bool
		wantUsage  int
	}{
		{"small log links the decision", false, false, false, true, 1},
		{"decision inside the window of a big log", false, true, false, true, 1},
		{"decision older than the window records unlinked", true, false, false, false, 1},
		{"same call id is counted once", false, true, true, true, 1},
		{"duplicate older than the window is counted again", true, false, true, false, 2},
	}
	r := catalog.Load()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "events.jsonl")
			t.Setenv("DOWNSHIFT_EVENT_LOG", path)
			if tc.padBefore {
				appendFiller(t, path)
			}
			telemetry.Record(decision("toolu_1"))
			if tc.padAfter {
				appendFiller(t, path)
			}
			res := childend.Record(spawnEnd("toolu_1"), "test", r)
			if res.Linked != tc.wantLinked {
				t.Fatalf("linked = %v, want %v (%q)", res.Linked, tc.wantLinked, res.Note)
			}
			if tc.repeat {
				if tc.padAfter {
					appendFiller(t, path) // the first usage line leaves the window too
				}
				childend.Record(spawnEnd("toolu_1"), "test", r)
			}
			if got := countUsage(t); got != tc.wantUsage {
				t.Fatalf("usage records = %d, want %d", got, tc.wantUsage)
			}
		})
	}
}

// A non-spawn tool call reports nothing, so the recorder returns without
// writing and without reading the (here unreadable) log.
func TestRecord_PlainToolCallDoesNotTouchTheLog(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	t.Setenv("DOWNSHIFT_EVENT_LOG", path)
	res := childend.Record(hookport.ChildEnd{Harness: "claude-code", SessionID: "sess", ToolUseID: "toolu_9", Source: hookport.SourcePayload}, "test", catalog.Load())
	if res != (childend.Result{}) {
		t.Fatalf("plain tool call produced %+v", res)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("log was created: %v", err)
	}
}
