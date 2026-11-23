// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestParseTimeout(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want time.Duration
	}{
		{name: "empty", raw: "", want: 0},
		{name: "whitespace", raw: "   ", want: 0},
		{name: "invalid", raw: "not-a-duration", want: 0},
		{name: "zero", raw: "0s", want: 0},
		{name: "negative", raw: "-5s", want: 0},
		{name: "milliseconds", raw: "50ms", want: 50 * time.Millisecond},
		{name: "seconds", raw: "2s", want: 2 * time.Second},
		{name: "trimmed_compound", raw: " 1m30s ", want: 90 * time.Second},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := parseTimeout(tc.raw); got != tc.want {
				t.Fatalf("parseTimeout(%q) = %v, want %v", tc.raw, got, tc.want)
			}
		})
	}
}

func TestNonTTYEnv(t *testing.T) {
	cases := []struct {
		name        string
		base        []string
		interactive bool
		wantHas     []string
		wantAbsent  []string
	}{
		{
			name:        "non_tty_injects_defaults",
			base:        []string{"PATH=/usr/bin"},
			interactive: false,
			wantHas:     []string{"PATH=/usr/bin", DepthEnv + "=1", "GIT_TERMINAL_PROMPT=0", "GH_PROMPT_DISABLED=1"},
		},
		{
			name:        "non_tty_preserves_explicit_git_prompt",
			base:        []string{"GIT_TERMINAL_PROMPT=1"},
			interactive: false,
			wantHas:     []string{"GIT_TERMINAL_PROMPT=1", DepthEnv + "=1", "GH_PROMPT_DISABLED=1"},
			wantAbsent:  []string{"GIT_TERMINAL_PROMPT=0"},
		},
		{
			name:        "non_tty_preserves_explicit_gh_prompt",
			base:        []string{"GH_PROMPT_DISABLED=0"},
			interactive: false,
			wantHas:     []string{"GH_PROMPT_DISABLED=0", DepthEnv + "=1", "GIT_TERMINAL_PROMPT=0"},
			wantAbsent:  []string{"GH_PROMPT_DISABLED=1"},
		},
		{
			name:        "interactive_skips_prompt_guards",
			base:        []string{"PATH=/usr/bin"},
			interactive: true,
			wantHas:     []string{"PATH=/usr/bin", DepthEnv + "=1"},
			wantAbsent:  []string{"GIT_TERMINAL_PROMPT=0", "GH_PROMPT_DISABLED=1"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := nonTTYEnv(tc.base, tc.interactive)
			for _, want := range tc.wantHas {
				if !slices.Contains(got, want) {
					t.Errorf("nonTTYEnv(%v, %v) missing %q in %v", tc.base, tc.interactive, want, got)
				}
			}
			for _, absent := range tc.wantAbsent {
				if slices.Contains(got, absent) {
					t.Errorf("nonTTYEnv(%v, %v) unexpectedly contains %q in %v", tc.base, tc.interactive, absent, got)
				}
			}
		})
	}
}

func writeFakePOSIXBin(t *testing.T, name, script string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell fake binary")
	}
	dir := t.TempDir()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"+script+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestRunTimeout(t *testing.T) {
	writeFakePOSIXBin(t, "git", "/bin/echo 'partial output'\nsleep 5")

	t.Run("options_timeout", func(t *testing.T) {
		var out, errb bytes.Buffer
		res := Run([]string{"git", "status"}, Options{
			Stdin:   strings.NewReader(""),
			Stdout:  &out,
			Stderr:  &errb,
			Compact: true,
			Timeout: 150 * time.Millisecond,
		})
		if res.ExitCode != ExitTimeout {
			t.Fatalf("ExitCode = %d, want %d (stderr=%q)", res.ExitCode, ExitTimeout, errb.String())
		}
		if res.ExitClass != ExitClassNonZero {
			t.Errorf("ExitClass = %q, want %q", res.ExitClass, ExitClassNonZero)
		}
		if res.Passthrough != PassthroughNonZeroExit {
			t.Errorf("Passthrough = %q, want %q", res.Passthrough, PassthroughNonZeroExit)
		}
		if !strings.Contains(errb.String(), "downshift compact: git: timed out after 150ms\n") {
			t.Errorf("stderr = %q, want timeout notice", errb.String())
		}
		if !strings.Contains(out.String(), "partial output") {
			t.Errorf("stdout = %q, want buffered output flushed", out.String())
		}
	})

	t.Run("env_timeout", func(t *testing.T) {
		t.Setenv(TimeoutEnv, "150ms")
		var out, errb bytes.Buffer
		res := Run([]string{"git", "status"}, Options{
			Stdin:   strings.NewReader(""),
			Stdout:  &out,
			Stderr:  &errb,
			Compact: true,
		})
		if res.ExitCode != ExitTimeout {
			t.Fatalf("ExitCode = %d, want %d (stderr=%q)", res.ExitCode, ExitTimeout, errb.String())
		}
		if !strings.Contains(errb.String(), "downshift compact: git: timed out after 150ms\n") {
			t.Errorf("stderr = %q, want timeout notice", errb.String())
		}
	})
}

func TestRunNonTTYPromptEnv(t *testing.T) {
	writeFakePOSIXBin(t, "git", `printf 'GIT=%s GH=%s\n' "$GIT_TERMINAL_PROMPT" "$GH_PROMPT_DISABLED"`)

	t.Run("injects_when_missing", func(t *testing.T) {
		var out, errb bytes.Buffer
		res := Run([]string{"git", "status"}, Options{
			Stdin:  strings.NewReader(""),
			Stdout: &out,
			Stderr: &errb,
		})
		if res.ExitCode != 0 {
			t.Fatalf("ExitCode = %d, stderr = %q", res.ExitCode, errb.String())
		}
		if got := strings.TrimSpace(out.String()); got != "GIT=0 GH=1" {
			t.Errorf("stdout = %q, want %q", got, "GIT=0 GH=1")
		}
	})

	t.Run("preserves_caller_override", func(t *testing.T) {
		t.Setenv("GIT_TERMINAL_PROMPT", "1")
		var out, errb bytes.Buffer
		res := Run([]string{"git", "status"}, Options{
			Stdin:  strings.NewReader(""),
			Stdout: &out,
			Stderr: &errb,
		})
		if res.ExitCode != 0 {
			t.Fatalf("ExitCode = %d, stderr = %q", res.ExitCode, errb.String())
		}
		if got := strings.TrimSpace(out.String()); got != "GIT=1 GH=1" {
			t.Errorf("stdout = %q, want %q", got, "GIT=1 GH=1")
		}
	})
}
