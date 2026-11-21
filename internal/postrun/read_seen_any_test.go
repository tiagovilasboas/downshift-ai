// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package postrun

import (
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/paths"
)

func TestSeenAny_Table(t *testing.T) {
	body := strings.Repeat("abc1234 some commit subject line here\n", 100)
	small := strings.Repeat("x\n", 500)
	sessB := strings.Repeat("cd", 32)
	cases := []struct {
		name    string
		cmd     string
		out     string
		exit    int
		stderr  string
		session string
		want    bool
	}{
		{"git log piped to cat", "git log --oneline -n 100 | cat", body, 0, "", sessA, true},
		{"wc listing", "wc -l /some/dir/*", body, 0, "", sessA, true},
		{"exit nonzero", "git log --oneline | cat", body, 1, "", sessA, false},
		{"different session", "git log --oneline | cat", body, 0, "", sessB, false},
		{"refused store", "curl -s https://example.test/x | cat", body, 0, "", sessA, false},
		{"below 2 KiB", "git log --oneline | cat", small, 0, "", sessA, false},
		{"large stderr", "git log --oneline | cat", body, 0, strings.Repeat("e", 2000), sessA, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv(paths.EnvStateDir, t.TempDir())
			keep, _ := seenKeep(t)
			in := shellRead(c.cmd, c.out)
			in.ExitCode, in.Stderr, in.HasStderr = c.exit, c.stderr, c.stderr != ""
			Compact(allCaps, in, Options{Keep: keep, Session: sessA})
			res := Compact(allCaps, in, Options{Keep: keep, Session: c.session})
			got := res.Replaced && strings.Contains(res.Output.Stdout, "same output as an earlier run")
			if got != c.want {
				t.Fatalf("dedup=%v want %v: %+v", got, c.want, res)
			}
			if got && (strings.Contains(res.Output.Stdout, "already read") || res.KeptID != "seenid123456") {
				t.Fatalf("pointer: %q kept=%q", res.Output.Stdout, res.KeptID)
			}
		})
	}
}

func TestSeenAny_FileReadPointerUnchanged(t *testing.T) {
	t.Setenv(paths.EnvStateDir, t.TempDir())
	keep, _ := seenKeep(t)
	o := Options{Keep: keep, Session: sessA}
	body := strings.Repeat("package x // some file text\n", 200)
	Compact(allCaps, shellRead("cat a.go", body), o)
	res := Compact(allCaps, shellRead("cat a.go", body), o)
	if !strings.Contains(res.Output.Stdout, "unchanged, same") || !strings.Contains(res.Output.Stdout, "already read in this session") {
		t.Fatalf("file-read pointer changed: %q", res.Output.Stdout)
	}
}
