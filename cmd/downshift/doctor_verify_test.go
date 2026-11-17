// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/execwrap"
	"github.com/tiagovilasboas/downshift-ai/internal/paths"
	"github.com/tiagovilasboas/downshift-ai/internal/telemetry"
	"github.com/tiagovilasboas/downshift-ai/internal/testenv"
)

func TestDoctorVerify_UnknownHarness(t *testing.T) {
	var out, errs bytes.Buffer
	if got := runDoctor([]string{"--verify", "not-a-harness"}, &out, &errs, nil); got != 2 {
		t.Fatalf("exit = %d, want 2; err=%s", got, errs.String())
	}
	if !strings.Contains(errs.String(), "unknown harness: not-a-harness") {
		t.Fatalf("stderr = %q, want unknown harness error", errs.String())
	}
}

func TestDoctorVerify_ConflictsWithHarnessFlag(t *testing.T) {
	var out, errs bytes.Buffer
	if got := runDoctor([]string{"--verify", "claude-code", "--harness", "codex"}, &out, &errs, nil); got != 2 {
		t.Fatalf("exit = %d, want 2; err=%s", got, errs.String())
	}
	if !strings.Contains(errs.String(), "conflicts with --harness") {
		t.Fatalf("stderr = %q, want conflict error", errs.String())
	}
}

func TestDoctorVerify_MissingEventsAndInstructions(t *testing.T) {
	now := time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)
	stale := now.Add(-30 * time.Minute).Format(time.RFC3339Nano)
	cases := []struct {
		name     string
		harness  string
		routing  []telemetry.Event
		execs    []execwrap.Event
		wantText []string
	}{
		{
			name:    "both missing",
			harness: "claude-code",
			wantText: []string{
				"✗ spawn/decision",
				"✗ shell/exec",
				"git status",
				"FAIL: missing required events for claude-code",
			},
		},
		{
			name:    "only spawn arrived and stale exec ignored",
			harness: "codex",
			routing: []telemetry.Event{
				{Harness: "codex", Source: "hook", Outcome: telemetry.OutcomeRewriteEmitted, Verdict: "DOWNSHIFT", FromModel: "opus", ToModel: "sonnet", Timestamp: now.Add(-2 * time.Minute).Format(time.RFC3339Nano)},
			},
			execs: []execwrap.Event{
				{Harness: "codex", Family: execwrap.FamilyGitStatus, ExitClass: "ok", Compacted: true, TS: stale},
				{Harness: "claude-code", Family: execwrap.FamilyGitStatus, ExitClass: "ok", Compacted: true, TS: now.Add(-time.Minute).Format(time.RFC3339)},
			},
			wantText: []string{
				"✓ spawn/decision",
				"✗ shell/exec",
				"FAIL: missing required events for codex",
			},
		},
		{
			name:    "kiro uses direct compact instruction when missing",
			harness: "kiro",
			execs: []execwrap.Event{
				{Harness: "kiro", Family: execwrap.FamilyGitStatus, ExitClass: "ok", Compacted: true, TS: now.Add(-time.Minute).Format(time.RFC3339)},
			},
			wantText: []string{
				"✗ spawn/decision",
				"✓ shell/exec",
				"downshift compact --harness=kiro -- git status",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out, errs bytes.Buffer
			env := verifyEnv{
				Now:         func() time.Time { return now },
				ReadRouting: func() ([]telemetry.Event, error) { return tc.routing, nil },
				ReadExec:    func() ([]execwrap.Event, error) { return tc.execs, nil },
				Window:      15 * time.Minute,
			}
			if got := runDoctorVerify(tc.harness, false, &out, &errs, env); got != 1 {
				t.Fatalf("exit = %d, want 1; out=%s err=%s", got, out.String(), errs.String())
			}
			for _, want := range tc.wantText {
				if !strings.Contains(out.String(), want) {
					t.Errorf("output missing %q:\n%s", want, out.String())
				}
			}
		})
	}
}

func TestDoctorVerify_BothEventsArrived_TempStateDir(t *testing.T) {
	home := t.TempDir()
	testenv.SetHome(t, home)
	stateDir := t.TempDir()
	t.Setenv(paths.EnvStateDir, stateDir)
	t.Setenv("DOWNSHIFT_EVENT_LOG", "")

	now := time.Now().UTC()
	routingLine, err := json.Marshal(telemetry.Event{
		CorrelationID: "v1",
		Timestamp:     now.Add(-30 * time.Second).Format(time.RFC3339Nano),
		Source:        "hook",
		Harness:       "claude-code",
		FromModel:     "claude-opus-5-5",
		ToModel:       "claude-sonnet-5-5",
		Verdict:       "DOWNSHIFT",
		Outcome:       telemetry.OutcomeRewriteEmitted,
	})
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(stateDir, "events.jsonl"), string(routingLine)+"\n")

	execLine, err := json.Marshal(execwrap.Event{
		TS:        now.Add(-20 * time.Second).Format(time.RFC3339),
		Family:    execwrap.FamilyGitStatus,
		ExitClass: "ok",
		Compacted: true,
		Harness:   "claude-code",
	})
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(stateDir, execwrap.EventsFileName), string(execLine)+"\n")

	var out, errs bytes.Buffer
	if got := runDoctor([]string{"--verify", "claude-code"}, &out, &errs, nil); got != 0 {
		t.Fatalf("exit = %d, want 0; out=%s err=%s", got, out.String(), errs.String())
	}
	if !strings.Contains(out.String(), "PASS: verified spawn and shell events for claude-code") {
		t.Fatalf("unexpected output:\n%s", out.String())
	}

	var jsonOut bytes.Buffer
	if got := runDoctor([]string{"--verify", "claude-code", "--json"}, &jsonOut, &errs, nil); got != 0 {
		t.Fatalf("json exit = %d, want 0; err=%s", got, errs.String())
	}
	var res verifyResult
	if err := json.Unmarshal(jsonOut.Bytes(), &res); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, jsonOut.String())
	}
	if !res.OK || !res.SpawnEvent.Found || !res.ShellEvent.Found || res.Harness != "claude-code" {
		t.Fatalf("unexpected json result: %+v", res)
	}
}
