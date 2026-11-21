// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package postrun

import (
	"fmt"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/core"
	"github.com/tiagovilasboas/downshift-ai/internal/execwrap"
	"github.com/tiagovilasboas/downshift-ai/internal/hookport"
)

func TestExtractPostRunFamily_Piped(t *testing.T) {
	for _, tc := range []struct {
		cmd  string
		exit int
		want execwrap.Family
	}{
		{"git status | head -20", 0, execwrap.FamilyGitStatus},
		{"git log --oneline -50 | cat", 0, execwrap.FamilyGitLog},
		{"go test ./... 2>&1 | tail -30", 0, execwrap.FamilyGoTest},
		{"git show HEAD | head -200", 0, execwrap.FamilyGitShow},
		{"git diff | head -n 100", 0, execwrap.FamilyGitDiff},
		{"cd repo && go test ./... 2>&1 | tail -n30", 0, execwrap.FamilyGoTest},
		{"ls -la | grep go", 0, execwrap.FamilyLs},
		{"ls -la | grep -v '^d' | sort | head -5", 0, execwrap.FamilyLs},
		{"git branch | grep -i feat", 0, execwrap.FamilyGitBranch},
		{"cat f | head", 0, execwrap.FamilyNone},
		{"git log | xargs echo", 0, execwrap.FamilyNone},
		{"git log | head -5 | xargs echo", 0, execwrap.FamilyNone},
		{"git status | head -20", 1, execwrap.FamilyNone},
		{"git status | grep modified", 0, execwrap.FamilyNone}, // multi-line records
		{"git log | sort", 0, execwrap.FamilyNone},
		{"git log | head -5 > out.txt", 0, execwrap.FamilyNone},
		{"git log | head DOLLAR(echo 5)", 0, execwrap.FamilyNone},
		{"git log | head -c 100", 0, execwrap.FamilyNone},
		{"git log || head", 0, execwrap.FamilyNone},
		{"ls | grep -c go", 0, execwrap.FamilyNone},
		{"ls | grep -e a b", 0, execwrap.FamilyNone},
		{"grep -rn foo . | head -20", 0, execwrap.FamilyNone}, // search-led: CutSearch owns it
		{"wc -l f | head", 0, execwrap.FamilyNone},
	} {
		cmd := strings.ReplaceAll(tc.cmd, "DOLLAR", "$")
		got, _ := extractPipedPostRunFamily(cmd, tc.exit)
		if got != tc.want {
			t.Errorf("%q exit %d: family %q, want %q", cmd, tc.exit, got, tc.want)
		}
	}
}

// Piped family commands reach their compressor and shrink the output.
func TestCompact_PipedFamilyShrinks(t *testing.T) {
	var goTest, status strings.Builder
	for i := 0; i < 60; i++ {
		fmt.Fprintf(&goTest, "=== RUN   TestCase%d\n--- PASS: TestCase%d (0.00s)\n", i, i)
	}
	goTest.WriteString("PASS\nok  \texample.com/pkg\t0.123s\n")
	status.WriteString("On branch main\nChanges not staged for commit:\n")
	for i := 0; i < 80; i++ {
		fmt.Fprintf(&status, "\tmodified:   internal/pkg/file_%02d.go\n", i)
	}
	for _, tc := range []struct {
		cmd, out string
		want     execwrap.Family
	}{
		{"go test ./... 2>&1 | tail -300", goTest.String(), execwrap.FamilyGoTest},
		{"git status | head -200", status.String(), execwrap.FamilyGitStatus},
	} {
		in := hookport.ToolOutput{Category: core.CategoryShell, Command: tc.cmd, Stdout: tc.out, HasStdout: true, ExitCode: 0}
		res := Compact(allCaps, in, Options{Keep: keepOK, Balanced: true})
		if res.Family != tc.want || !res.Replaced || res.EmittedBytes >= res.RawBytes {
			t.Errorf("%q: %+v", tc.cmd, res)
		}
		t.Logf("%q: %d -> %d bytes", tc.cmd, res.RawBytes, res.EmittedBytes)
	}
}

// A listing the agent limited keeps every row, even when the family
// compressor would fold them; the balanced cut is untouched here.
func TestCompact_PipedLimitedKeepsEveryRow(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 30; i++ {
		fmt.Fprintf(&b, "-rw-r--r--  1 u g %6d Nov 21 10:00 some_long_file_name_%04d.go\n", i, i)
	}
	in := hookport.ToolOutput{Category: core.CategoryShell, Command: "ls -la | head -30", Stdout: b.String(), HasStdout: true, ExitCode: 0}
	res := Compact(allCaps, in, Options{Keep: keepOK, Balanced: true})
	if res.Family != execwrap.FamilyNone {
		t.Fatalf("family %q, want none: the family compressor would drop rows", res.Family)
	}
	if res.Replaced {
		if n := strings.Count(res.Output.Stdout, "some_long_file_name_"); n < 30 {
			t.Errorf("limited listing kept %d of 30 rows", n)
		}
	}
}
