// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/setup"
)

// installBridge writes the files setup would, so the bridge row is current.
func installBridge(t *testing.T, home string) {
	t.Helper()
	tg, ok := generatedFilesTarget("opencode")
	if !ok {
		t.Fatal("opencode has no generated files")
	}
	for _, f := range tg.Files {
		body, err := fs.ReadFile(f.FS, f.Source)
		if err != nil {
			t.Fatal(err)
		}
		writeBridgeFile(t, filepath.Join(home, filepath.FromSlash(f.Dest)), body)
	}
}

func (f *doctorFixture) writeOpenCodeEvents(t *testing.T, age time.Duration) {
	t.Helper()
	ts := f.started.Add(-age).Format(time.RFC3339Nano)
	decision := `{"correlation_id":"d1","timestamp":"` + ts + `","harness":"opencode","source":"hook","session_id":"s1","from_model":"opus","to_model":"haiku","requested_model":"opus","final_model":"haiku","verdict":"DOWNSHIFT","outcome":"rewrite_emitted"}`
	resolved := `{"correlation_id":"r1","timestamp":"` + ts + `","harness":"opencode","source":"hook","session_id":"s1","from_model":"haiku","to_model":"haiku","verdict":"UNKNOWN","outcome":"resolved","linked_decision":"d1","rewrite_honored":true}`
	writeFile(t, filepath.Join(f.home, ".downshift", "events.jsonl"), decision+"\n"+resolved+"\n")
}

func TestTargetBridgeIsDerivedFromFilesWithoutHookFile(t *testing.T) {
	for _, tg := range setup.Targets {
		if got, want := tg.Bridge(), len(tg.Files) > 0 && tg.File == ""; got != want {
			t.Errorf("%s: Bridge() = %v, want %v", tg.Name, got, want)
		}
	}
	if tg, _ := generatedFilesTarget("opencode"); !tg.Bridge() {
		t.Error("opencode must be a bridge target")
	}
}

func TestDoctorBridgeEvidenceShowsLastEventAndHonored(t *testing.T) {
	f := newDoctorFixture(t)
	installBridge(t, f.home)
	f.writeOpenCodeEvents(t, 5*time.Minute)
	rep := buildDoctorReport(f.env)

	c := f.check(t, rep, "opencode", "last hook event")
	wantStatus(t, c, doctorOK)
	if !strings.Contains(c.Detail, "DOWNSHIFT") {
		t.Errorf("detail %q misses the decision", c.Detail)
	}
	c = f.check(t, rep, "opencode", "rewrite honored")
	wantStatus(t, c, doctorOK)
	if !strings.Contains(c.Detail, "seen: 1 of 1") {
		t.Errorf("detail %q", c.Detail)
	}
	wantStatus(t, f.check(t, rep, "opencode", "bridge"), doctorOK)
}

func TestDoctorBridgeEvidenceWarnsWithoutEvents(t *testing.T) {
	f := newDoctorFixture(t)
	installBridge(t, f.home)
	rep := buildDoctorReport(f.env)
	c := f.check(t, rep, "opencode", "last hook event")
	wantStatus(t, c, doctorWarn)
	for _, want := range []string{"restart opencode", "spawn one subagent", "downshift stats --days=1"} {
		if !strings.Contains(c.Fix, want) {
			t.Errorf("fix %q misses %q", c.Fix, want)
		}
	}
}

func TestDoctorBridgeAbsentStaysNotDetected(t *testing.T) {
	f := newDoctorFixture(t)
	f.writeOpenCodeEvents(t, time.Minute)
	rep := buildDoctorReport(f.env)
	found := false
	for _, h := range rep.NotDetected {
		found = found || h == "opencode"
	}
	if !found {
		t.Errorf("opencode must be NotDetected without its config dir: %v", rep.NotDetected)
	}
	for _, c := range rep.Checks {
		if c.Harness == "opencode" {
			t.Errorf("undetected harness got a row: %+v", c)
		}
	}
}

// A hook-file harness keeps its single last-event row; the bridge path adds
// rows only for the bridge harness.
func TestDoctorHookHarnessUnchangedByBridgeEvidence(t *testing.T) {
	f := newDoctorFixture(t)
	writeFile(t, filepath.Join(f.home, ".claude", "settings.json"), claudeSettings(f.bin))
	installBridge(t, f.home)
	f.writeOpenCodeEvents(t, time.Minute)
	rep := buildDoctorReport(f.env)
	// claude-code has no events while opencode is active, so it collapses into Inactive.
	foundInactive := false
	for _, h := range rep.Inactive {
		foundInactive = foundInactive || h == "claude-code"
	}
	if !foundInactive {
		t.Errorf("inactive wired claude-code should collapse into Inactive: %v", rep.Inactive)
	}
	// When explicitly targeted via --harness, claude-code keeps its single last-event row.
	f.env.Harness = "claude-code"
	rep = buildDoctorReport(f.env)
	count := map[string]int{}
	for _, c := range rep.Checks {
		if c.ID == "last hook event" {
			count[c.Harness]++
		}
	}
	if count["claude-code"] != 1 || count["opencode"] != 0 {
		t.Errorf("last hook event rows = %v, want only claude-code:1", count)
	}
	c := f.check(t, rep, "claude-code", "last hook event")
	wantStatus(t, c, doctorWarn) // the opencode events are not claude-code's
}
