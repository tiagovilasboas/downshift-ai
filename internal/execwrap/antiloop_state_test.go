// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/paths"
)

const loopSess = "0123456789abcdef"

func loopStateFiles(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	var out []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), antiLoopPrefix) {
			out = append(out, e.Name())
		}
	}
	return out
}

func failN(stderr *bytes.Buffer, session, id string, n int, now time.Time) {
	for i := 0; i < n; i++ {
		checkAntiLoopAt(stderr, session, 1, id, now)
	}
}

// Sessionless runs are not tracked: no state file, no warning, no cross-talk.
func TestAntiLoop_NoSessionSkipsGuard(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(paths.EnvStateDir, dir)
	var errb bytes.Buffer
	now := time.Now()
	for _, s := range []string{"", "not-a-hash"} {
		failN(&errb, s, "deadbeef0001", 5, now)
		checkAntiLoopAt(&errb, s, 0, "", now)
	}
	if errb.Len() != 0 {
		t.Fatalf("stderr = %q, want nothing", errb.String())
	}
	if got := loopStateFiles(t, dir); len(got) != 0 {
		t.Fatalf("state files = %v, want none", got)
	}
}

func TestAntiLoop_WarningCarriesSessionRef(t *testing.T) {
	t.Setenv(paths.EnvStateDir, t.TempDir())
	var errb bytes.Buffer
	failN(&errb, loopSess, "deadbeef0001", 3, time.Now())
	want := "'downshift retrieve --session=" + SessionRef(loopSess) + " deadbeef0001'"
	if !strings.Contains(errb.String(), want) {
		t.Fatalf("stderr = %q, want %q", errb.String(), want)
	}
}

// A streak older than the window restarts at 1.
func TestAntiLoop_StaleStreakRestarts(t *testing.T) {
	t.Setenv(paths.EnvStateDir, t.TempDir())
	var errb bytes.Buffer
	t0 := time.Now()
	failN(&errb, loopSess, "deadbeef0001", 2, t0)
	later := t0.Add(antiLoopWindow + time.Minute)
	failN(&errb, loopSess, "deadbeef0001", 2, later)
	if errb.Len() != 0 {
		t.Fatalf("stderr = %q, want no warning after the window expired", errb.String())
	}
	failN(&errb, loopSess, "deadbeef0001", 1, later)
	if !strings.Contains(errb.String(), "3x") {
		t.Fatalf("stderr = %q, want the 3x warning within the window", errb.String())
	}
}

// A passing run removes the state only when it exists, and with none it is a no-op.
func TestAntiLoop_PassRemovesState(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(paths.EnvStateDir, dir)
	now := time.Now()
	checkAntiLoopAt(nil, loopSess, 0, "", now) // nothing to remove, must not fail
	failN(nil, loopSess, "deadbeef0001", 1, now)
	if got := loopStateFiles(t, dir); len(got) != 1 {
		t.Fatalf("state files = %v, want one", got)
	}
	checkAntiLoopAt(nil, loopSess, 0, "", now)
	if got := loopStateFiles(t, dir); len(got) != 0 {
		t.Fatalf("state files = %v, want none after a pass", got)
	}
}

// A failing write removes other sessions' state files older than 24h and
// keeps the recent ones.
func TestAntiLoop_SweepsStaleFiles(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(paths.EnvStateDir, dir)
	now := time.Now()
	stale := filepath.Join(dir, antiLoopPrefix+"aaaaaaaaaaaa.json")
	fresh := filepath.Join(dir, antiLoopPrefix+"bbbbbbbbbbbb.json")
	other := filepath.Join(dir, "unrelated.json")
	for _, p := range []string{stale, fresh, other} {
		if err := os.WriteFile(p, []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	old := now.Add(-antiLoopStaleAge - time.Hour)
	for _, p := range []string{stale, other} {
		if err := os.Chtimes(p, old, old); err != nil {
			t.Fatal(err)
		}
	}
	failN(nil, loopSess, "deadbeef0001", 1, now)
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatalf("stale state still present: %v", err)
	}
	for _, p := range []string{fresh, other, filepath.Join(dir, antiLoopPrefix+SessionRef(loopSess)+".json")} {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("%s removed: %v", filepath.Base(p), err)
		}
	}
}
