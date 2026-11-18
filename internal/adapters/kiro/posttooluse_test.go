// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package kiro

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/core"
	"github.com/tiagovilasboas/downshift-ai/internal/hookport"
	"github.com/tiagovilasboas/downshift-ai/internal/mainwork"
	"github.com/tiagovilasboas/downshift-ai/internal/paths"
	"github.com/tiagovilasboas/downshift-ai/internal/sensor"
	"github.com/tiagovilasboas/downshift-ai/internal/telemetry"
)

func TestPort_HonorIsWired(t *testing.T) {
	t.Setenv(paths.EnvStateDir, t.TempDir())
	p := Port()
	if p.Honor == nil {
		t.Fatal("Port().Honor must be non-nil")
	}
	obs := hookport.ObserveHonor(p, []byte(`{"hook_event_name":"PostToolUse","tool_name":"read_file","session_id":"s1","tool_response":"hello"}`), "test", nil)
	if !obs.Observed {
		t.Fatal("ObserveHonor must report Observed=true")
	}
}

func TestHandlePostToolUse_SensorAndMainWorkForExecuteBash(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv(paths.EnvStateDir, stateDir)

	bigOut := strings.Repeat("FAIL: assertion failed\n", 80)
	raw, _ := json.Marshal(map[string]any{
		"hook_event_name": "PostToolUse",
		"session_id":      "sess-kiro-1",
		"tool_name":       "execute_bash",
		"tool_input":      map[string]any{"command": "go test ./..."},
		"tool_response": map[string]any{
			"stdout":    bigOut,
			"exit_code": 0,
		},
	})

	out, _, linked := HandlePostToolUse(raw, "test", nil)
	if linked {
		t.Fatalf("execute_bash must not link childend, got linked=true")
	}
	if out != (struct{}{}) {
		t.Fatalf("out = %#v, want empty struct", out)
	}

	st, err := sensor.DefaultStore()
	if err != nil {
		t.Fatalf("DefaultStore: %v", err)
	}
	summary, err := st.GetSummary()
	if err != nil {
		t.Fatalf("GetSummary: %v", err)
	}
	sessHash := telemetry.HashSessionID("sess-kiro-1")
	obs, ok := summary[sessHash]
	if !ok {
		t.Fatalf("missing sensor observation for session hash %s: %+v", sessHash, summary)
	}
	if obs.Harness != harnessID || obs.ToolExecutions != 1 || obs.TotalOutputBytes != int64(len(bigOut)) {
		t.Errorf("unexpected sensor obs: %+v", obs)
	}

	recs, err := mainwork.Read(filepath.Join(stateDir, mainwork.FileName))
	if err != nil || len(recs) != 1 {
		t.Fatalf("mainwork records = %d (err=%v), want 1", len(recs), err)
	}
	if recs[0].Harness != harnessID || recs[0].Tool != "execute_bash" || recs[0].Category != string(core.CategoryShell) {
		t.Errorf("unexpected mainwork record: %+v", recs[0])
	}
}

func TestHandlePostToolUse_ReadAndSearchSkipSensorAndRecordMainWork(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv(paths.EnvStateDir, stateDir)

	for _, tc := range []struct {
		tool string
		cat  core.ToolCategory
	}{
		{"read_file", core.CategoryRead},
		{"grep_search", core.CategorySearch},
		{"file_search", core.CategorySearch},
	} {
		raw, _ := json.Marshal(map[string]any{
			"hook_event_name": "PostToolUse",
			"session_id":      "sess-kiro-2",
			"tool_name":       tc.tool,
			"tool_response":   map[string]any{"content": strings.Repeat("x", 2048)},
		})
		HandlePostToolUse(raw, "test", nil)
	}

	if _, err := os.Stat(filepath.Join(stateDir, "context-sensor.json")); !os.IsNotExist(err) {
		t.Fatalf("read/search must not write context-sensor.json, stat err=%v", err)
	}
	recs, err := mainwork.Read(filepath.Join(stateDir, mainwork.FileName))
	if err != nil || len(recs) != 3 {
		t.Fatalf("mainwork records = %d (err=%v), want 3", len(recs), err)
	}
	if recs[0].ResultBytes != 2048 {
		t.Errorf("read_file ResultBytes = %d, want 2048", recs[0].ResultBytes)
	}
}

func TestHandlePostToolUse_SubagentLinksDecision(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv(paths.EnvStateDir, stateDir)
	t.Setenv("DOWNSHIFT_EVENT_LOG", filepath.Join(stateDir, "events.jsonl"))

	sessHash := telemetry.HashSessionID("sess-kiro-3")
	callHash := telemetry.HashSessionID("call-1")
	telemetry.Record(telemetry.Event{
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
		Harness:     harnessID,
		SessionID:   sessHash,
		ToolUseHash: callHash,
		Verdict:     "OK",
		Outcome:     telemetry.OutcomeAllow,
		FromModel:   "claude-sonnet-5-5",
		ToModel:     "claude-sonnet-5-5",
		Source:      "hook",
	})

	raw, _ := json.Marshal(map[string]any{
		"hook_event_name": "PostToolUse",
		"session_id":      "sess-kiro-3",
		"tool_use_id":     "call-1",
		"tool_name":       "invoke_sub_agent",
		"tool_input":      map[string]any{"prompt": "inspect logs", "model": "claude-sonnet-5-5"},
		"tool_response":   map[string]any{"status": "completed"},
	})
	_, _, _ = HandlePostToolUse(raw, "test", nil)

	events, err := telemetry.ReadEventsFrom(filepath.Join(stateDir, "events.jsonl"))
	if err != nil {
		t.Fatalf("ReadEvents: %v", err)
	}
	var sawChildEnd bool
	for _, ev := range events {
		if ev.Outcome == telemetry.OutcomeChildEnded && ev.LinkedDecision != "" {
			sawChildEnd = true
		}
	}
	if !sawChildEnd {
		t.Errorf("expected child_ended observation linked to decision, got events: %+v", events)
	}
}

func TestHandlePostToolUse_InvalidJSONFailsOpen(t *testing.T) {
	out, note, linked := HandlePostToolUse([]byte(`not-json`), "test", nil)
	if out != (struct{}{}) || note != "" || linked {
		t.Fatalf("unexpected result on invalid JSON: out=%#v note=%q linked=%v", out, note, linked)
	}
}

func TestShellCommand(t *testing.T) {
	bashEv := Event{
		ToolName:  "execute_bash",
		ToolInput: json.RawMessage(`{"command":"go test ./..."}`),
	}
	if cmd, ok := bashEv.ShellCommand(); !ok || cmd != "go test ./..." {
		t.Fatalf("ShellCommand() = (%q, %v), want (%q, true)", cmd, ok, "go test ./...")
	}
	missingInput := Event{ToolName: "execute_bash"}
	if cmd, ok := missingInput.ShellCommand(); !ok || cmd != "" {
		t.Fatalf("missing ToolInput ShellCommand() = (%q, %v), want (\"\", true)", cmd, ok)
	}
	spawnEv := Event{ToolName: "invoke_sub_agent", ToolInput: json.RawMessage(`{"prompt":"hi"}`)}
	if cmd, ok := spawnEv.ShellCommand(); ok || cmd != "" {
		t.Fatalf("invoke_sub_agent ShellCommand() = (%q, %v), want (\"\", false)", cmd, ok)
	}
}
