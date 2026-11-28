// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func stubInteractiveStdin(t *testing.T, interactive bool) {
	t.Helper()
	prev := isInteractiveStdin
	isInteractiveStdin = func(io.Reader) bool { return interactive }
	t.Cleanup(func() { isInteractiveStdin = prev })
}

func unsetEnvForTest(t *testing.T, key string) {
	t.Helper()
	val, had := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if had {
			_ = os.Setenv(key, val)
		} else {
			_ = os.Unsetenv(key)
		}
	})
}

func TestRun_HarnessSuppressesTTYStderrRelease(t *testing.T) {
	var suite strings.Builder
	for _, p := range []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"} {
		suite.WriteString("ok  \tgithub.com/x/internal/" + p + "\t0.1s\n")
	}
	writeFakePOSIXBin(t, "make", "echo 'warning: notice' >&2\nprintf '%s' '"+suite.String()+"'")

	t.Run("explicit_harness_compacts_despite_tty_stdin", func(t *testing.T) {
		stubInteractiveStdin(t, true)
		var out, errb bytes.Buffer
		res := Run([]string{"make", "test"}, Options{
			Stdin:   strings.NewReader(""),
			Stdout:  &out,
			Stderr:  &errb,
			Compact: true,
			Harness: "antigravity",
		})
		if res.ExitCode != 0 || !res.Compacted || res.Passthrough != "" {
			t.Fatalf("expected compaction with explicit harness on PTY stdin; got %+v (stdout=%q stderr=%q)", res, out.String(), errb.String())
		}
		if errb.String() != "warning: notice\n" {
			t.Fatalf("stderr = %q, want %q", errb.String(), "warning: notice\n")
		}
	})

	t.Run("empty_harness_streams_on_stderr_with_tty_stdin", func(t *testing.T) {
		stubInteractiveStdin(t, true)
		var out, errb bytes.Buffer
		res := Run([]string{"make", "test"}, Options{
			Stdin:   strings.NewReader(""),
			Stdout:  &out,
			Stderr:  &errb,
			Compact: true,
		})
		if res.ExitCode != 0 || res.Compacted || res.Passthrough != PassthroughStderr {
			t.Fatalf("expected stderr passthrough without harness on PTY stdin; got %+v", res)
		}
		if out.String() != suite.String() || errb.String() != "warning: notice\n" {
			t.Fatalf("stdout/stderr altered: stdout=%q stderr=%q", out.String(), errb.String())
		}
	})
}

func TestRun_HarnessInjectsNonTTYEnvOnPTYStdin(t *testing.T) {
	writeFakePOSIXBin(t, "git", `printf 'GIT=%s GH=%s\n' "$GIT_TERMINAL_PROMPT" "$GH_PROMPT_DISABLED"`)

	t.Run("explicit_harness_injects_non_tty_env", func(t *testing.T) {
		unsetEnvForTest(t, "GIT_TERMINAL_PROMPT")
		unsetEnvForTest(t, "GH_PROMPT_DISABLED")
		stubInteractiveStdin(t, true)
		var out, errb bytes.Buffer
		res := Run([]string{"git", "status"}, Options{
			Stdin:   strings.NewReader(""),
			Stdout:  &out,
			Stderr:  &errb,
			Harness: "antigravity",
		})
		if res.ExitCode != 0 {
			t.Fatalf("ExitCode = %d, stderr = %q", res.ExitCode, errb.String())
		}
		if got := strings.TrimSpace(out.String()); got != "GIT=0 GH=1" {
			t.Fatalf("stdout = %q, want %q", got, "GIT=0 GH=1")
		}
	})

	t.Run("empty_harness_preserves_interactive_env_on_tty_stdin", func(t *testing.T) {
		unsetEnvForTest(t, "GIT_TERMINAL_PROMPT")
		unsetEnvForTest(t, "GH_PROMPT_DISABLED")
		stubInteractiveStdin(t, true)
		var out, errb bytes.Buffer
		res := Run([]string{"git", "status"}, Options{
			Stdin:  strings.NewReader(""),
			Stdout: &out,
			Stderr: &errb,
		})
		if res.ExitCode != 0 {
			t.Fatalf("ExitCode = %d, stderr = %q", res.ExitCode, errb.String())
		}
		if got := strings.TrimSpace(out.String()); got != "GIT= GH=" {
			t.Fatalf("stdout = %q, want %q", got, "GIT= GH=")
		}
	})
}
