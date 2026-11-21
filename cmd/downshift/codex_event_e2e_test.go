// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/execwrap"
	"github.com/tiagovilasboas/downshift-ai/internal/paths"
	"github.com/tiagovilasboas/downshift-ai/internal/telemetry"
	"github.com/tiagovilasboas/downshift-ai/internal/testenv"
)

// H5: a Codex PreToolUse payload leaves attributable rows. A spawn writes a
// routing decision row, and an allowlisted shell command is rewritten to a
// compact call whose exec row names the harness and the hook entry.
func TestCodexHookE2E_EventsCarryHarnessAndEntry(t *testing.T) {
	testenv.SetHome(t, t.TempDir())
	t.Setenv(paths.EnvStateDir, t.TempDir())
	eventLog := filepath.Join(t.TempDir(), "events.jsonl")
	t.Setenv("DOWNSHIFT_EVENT_LOG", eventLog)
	t.Setenv("DOWNSHIFT_CONTROL_SHARE", "0")
	t.Setenv(execwrap.DepthEnv, "")
	withExec(t, true, nil, true)

	hook := func(payload string) (int, string) {
		var rc int
		out := captureStdout(func() {
			rc = harnessRegistry["codex"].Hook(strings.NewReader(payload), cmdCat)
		})
		return rc, out
	}

	t.Run("spawn decision row", func(t *testing.T) {
		stdin, _ := json.Marshal(map[string]any{
			"hook_event_name": "PreToolUse", "tool_name": "spawn_agent", "session_id": "s1",
			"cwd": t.TempDir(), "tool_input": map[string]any{"message": "rename the variable userId"},
		})
		hook(string(stdin))
		raw, err := os.ReadFile(eventLog)
		if err != nil {
			t.Fatalf("no event log: %v", err)
		}
		var ev map[string]any
		if err := json.Unmarshal(bytes.TrimSpace(raw), &ev); err != nil {
			t.Fatalf("one decision row expected, got %q: %v", raw, err)
		}
		if ev["harness"] != "codex" || ev["entry"] != telemetry.EntryHook {
			t.Errorf("decision row harness=%v entry=%v, want codex/hook", ev["harness"], ev["entry"])
		}
	})

	t.Run("shell exec row", func(t *testing.T) {
		rc, out := hook(codexBashPayload(t, "Bash", "git status --short"))
		if rc != 0 {
			t.Fatalf("hook rc = %d", rc)
		}
		var resp struct {
			HookSpecificOutput struct {
				UpdatedInput struct {
					Command string `json:"command"`
				} `json:"updatedInput"`
			} `json:"hookSpecificOutput"`
		}
		if err := json.Unmarshal([]byte(out), &resp); err != nil {
			t.Fatalf("stdout %q: %v", out, err)
		}
		_, tail, ok := strings.Cut(resp.HookSpecificOutput.UpdatedInput.Command, " compact ")
		if !ok {
			t.Fatalf("command not rewritten: %q", resp.HookSpecificOutput.UpdatedInput.Command)
		}
		// Run the rewritten call the way Codex's shell would.
		var o, e bytes.Buffer
		runExec(strings.Fields(tail), strings.NewReader(""), &o, &e)

		path, err := execwrap.EventsPath()
		if err != nil {
			t.Fatal(err)
		}
		evs, err := execwrap.ReadEvents(path)
		if err != nil || len(evs) != 1 {
			t.Fatalf("exec events = %+v, err = %v", evs, err)
		}
		if evs[0].Harness != "codex" || evs[0].Entry != "hook" || evs[0].Session != testSession {
			t.Errorf("exec row harness=%q entry=%q session=%q, want codex/hook/%s",
				evs[0].Harness, evs[0].Entry, evs[0].Session, testSession)
		}
	})
}
