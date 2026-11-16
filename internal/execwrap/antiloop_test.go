// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package execwrap

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/paths"
)

func TestRun_AntiLoopGuard(t *testing.T) {
	t.Setenv(paths.EnvStateDir, t.TempDir())
	fakeBin(t, "go", shellPrintf(failedSuite)+"; exit 1")

	digest := "deadbeef0001"
	runFail := func(sess string) (Result, string) {
		var out, errb bytes.Buffer
		res := Run([]string{"go", "test", "./..."}, Options{
			Stdin:   strings.NewReader(""),
			Stdout:  &out,
			Stderr:  &errb,
			Compact: true,
			Session: sess,
			KeepFailed: func([]byte) (string, bool) {
				return digest, true
			},
		})
		return res, errb.String()
	}

	sessA := "aaaaaaaaaaaaaaaa"
	sessB := "bbbbbbbbbbbbbbbb"

	for i := 1; i <= 2; i++ {
		res, stderr := runFail(sessA)
		if res.ExitCode != 1 {
			t.Fatalf("run %d: exit code = %d, want 1", i, res.ExitCode)
		}
		if strings.Contains(stderr, "Context loop detected") {
			t.Fatalf("run %d: unexpected warning: %q", i, stderr)
		}
	}

	// Different session must not inherit sessA's count.
	if res, stderr := runFail(sessB); res.ExitCode != 1 || strings.Contains(stderr, "Context loop detected") {
		t.Fatalf("sessB run 1: exit=%d stderr=%q", res.ExitCode, stderr)
	}

	// 3rd consecutive identical failure on sessA triggers the warning.
	res, stderr := runFail(sessA)
	if res.ExitCode != 1 {
		t.Fatalf("run 3: exit code = %d, want 1", res.ExitCode)
	}
	want := "⚠️ [downshift ccr] Identical failure detected 3x (digest: deadbeef0001). Context loop detected. Check approach or review previous run with 'downshift retrieve --session=aaaaaaaaaaaa deadbeef0001'."
	if !strings.Contains(stderr, want) {
		t.Fatalf("run 3: stderr = %q, want substring %q", stderr, want)
	}

	// Different digest resets the counter to 1.
	digest = "deadbeef0002"
	res, stderr = runFail(sessA)
	if res.ExitCode != 1 || strings.Contains(stderr, "Context loop detected") {
		t.Fatalf("new digest run 1: exit=%d stderr=%q", res.ExitCode, stderr)
	}

	// Passing run resets the counter.
	fakeBin(t, "go", shellPrintf(passingSuite))
	var out, errb bytes.Buffer
	passRes := Run([]string{"go", "test", "./..."}, Options{
		Stdin:   strings.NewReader(""),
		Stdout:  &out,
		Stderr:  &errb,
		Compact: true,
		Session: sessA,
		KeepFailed: func([]byte) (string, bool) {
			return digest, true
		},
	})
	if passRes.ExitCode != 0 || strings.Contains(errb.String(), "Context loop detected") {
		t.Fatalf("passing run: exit=%d stderr=%q", passRes.ExitCode, errb.String())
	}

	// Failing again after a pass starts at 1, not 2.
	fakeBin(t, "go", shellPrintf(failedSuite)+"; exit 1")
	for i := 1; i <= 2; i++ {
		if r, s := runFail(sessA); r.ExitCode != 1 || strings.Contains(s, "Context loop detected") {
			t.Fatalf("post-pass run %d: exit=%d stderr=%q", i, r.ExitCode, s)
		}
	}
	if r, s := runFail(sessA); r.ExitCode != 1 || !strings.Contains(s, "3x (digest: deadbeef0002)") {
		t.Fatalf("post-pass run 3: exit=%d stderr=%q", r.ExitCode, s)
	}
}

func TestRun_AntiLoopFailOpenOnBadStateDir(t *testing.T) {
	// Point state dir at a regular file so PrivateDir/CreateTemp fails.
	blocker := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(paths.EnvStateDir, blocker)
	fakeBin(t, "go", shellPrintf(failedSuite)+"; exit 1")

	var out, errb bytes.Buffer
	res := Run([]string{"go", "test", "./..."}, Options{
		Stdin:      strings.NewReader(""),
		Stdout:     &out,
		Stderr:     &errb,
		Compact:    true,
		Session:    "cccccccccccccccc",
		KeepFailed: func([]byte) (string, bool) { return "deadbeef0003", true },
	})
	if res.ExitCode != 1 {
		t.Fatalf("exit code = %d, want 1", res.ExitCode)
	}
}
