// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"
)

func TestRunCompletionShells(t *testing.T) {
	shells := []struct {
		arg  string
		want []string
	}{
		{"bash", []string{"_downshift_completions", "complete -F _downshift_completions downshift", "setup", "doctor", "--days"}},
		{"zsh", []string{"#compdef downshift", "_downshift()", "compdef _downshift downshift", "setup", "discover"}},
		{"fish", []string{"complete -c downshift -f", "-a 'setup'", "-a 'zsh'", "-l dedup"}},
		{"powershell", []string{"Register-ArgumentCompleter -Native -CommandName downshift", "'setup'", "'--days'"}},
		{"pwsh", []string{"Register-ArgumentCompleter -Native -CommandName downshift"}},
	}
	for _, tc := range shells {
		var out, errBuf bytes.Buffer
		if code := runCompletion([]string{tc.arg}, &out, &errBuf); code != 0 {
			t.Fatalf("runCompletion(%q) code = %d, stderr = %s", tc.arg, code, errBuf.String())
		}
		if errBuf.Len() != 0 {
			t.Fatalf("runCompletion(%q) unexpected stderr: %s", tc.arg, errBuf.String())
		}
		got := out.String()
		for _, w := range tc.want {
			if !strings.Contains(got, w) {
				t.Errorf("runCompletion(%q) missing %q", tc.arg, w)
			}
		}
		if tc.arg == "bash" {
			if bash, err := exec.LookPath("bash"); err == nil {
				cmd := exec.Command(bash, "-n")
				cmd.Stdin = strings.NewReader(got)
				if b, err := cmd.CombinedOutput(); err != nil {
					t.Errorf("bash -n failed: %v\n%s", err, b)
				}
			}
		}
		if tc.arg == "zsh" {
			if zsh, err := exec.LookPath("zsh"); err == nil {
				cmd := exec.Command(zsh, "-n")
				cmd.Stdin = strings.NewReader(got)
				if b, err := cmd.CombinedOutput(); err != nil {
					t.Errorf("zsh -n failed: %v\n%s", err, b)
				}
			}
		}
	}
}

func TestRunCompletionErrors(t *testing.T) {
	for _, args := range [][]string{nil, {"tcsh"}, {"bash", "extra"}} {
		var out, errBuf bytes.Buffer
		if code := runCompletion(args, &out, &errBuf); code != 2 {
			t.Errorf("runCompletion(%v) code = %d, want 2", args, code)
		}
		if out.Len() != 0 || errBuf.Len() == 0 {
			t.Errorf("runCompletion(%v) out=%q err=%q", args, out.String(), errBuf.String())
		}
	}
}
