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
		{"ls | grep -c go", 0, execwrap.FamilyLs},
		{"ls -la | grep -F go", 0, execwrap.FamilyLs},
		{"ls -la | grep -n go", 0, execwrap.FamilyLs},
		{"ls -la | grep -w go", 0, execwrap.FamilyLs},
		{"ls -la | grep -Fnw go", 0, execwrap.FamilyLs},
		{"find . | grep -ic test", 0, execwrap.FamilyFind},
		{"ls -la | sort -ru | uniq -c", 0, execwrap.FamilyLs},
		{"find . | sort -V | uniq -i", 0, execwrap.FamilyFind},
		{"git branch | cat -", 0, execwrap.FamilyGitBranch},
		{"git log --oneline -20 | cat -u", 0, execwrap.FamilyGitLog},
		{"git remote -v | sort -u | grep origin", 0, execwrap.FamilyGitRemote},
		{"git stash list | grep wip", 0, execwrap.FamilyGitStash},
		{"kubectl get pods | grep -v Completed", 0, execwrap.FamilyKubectlGet},
		{"docker ps | grep -i api", 0, execwrap.FamilyDockerPS},
		{"docker images | sort -r", 0, execwrap.FamilyDockerImages},
		{"git remote -v | grep origin", 1, execwrap.FamilyNone},
		{"ls | sort -o out.txt", 0, execwrap.FamilyNone},
		{"ls | uniq out.txt", 0, execwrap.FamilyNone},
		{"ls | cat out.txt", 0, execwrap.FamilyNone},
		{"ls | cat -n", 0, execwrap.FamilyNone},
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

func TestCompact_PipedFormatAcceptedGuardsDetector(t *testing.T) {
	// Header stripped by grep: detectFormat declines FormatKubectlPods, so
	// formatAccepted keeps the small result raw instead of corrupting it.
	raw := "api-1   1/1   Running   0   1h\napi-2   1/1   Running   0   2h\n"
	in := hookport.ToolOutput{Category: core.CategoryShell, Command: "kubectl get pods | grep Running", Stdout: raw, HasStdout: true, ExitCode: 0}
	res := Compact(allCaps, in, Options{Keep: keepOK, Balanced: true})
	if res.Family != execwrap.FamilyKubectlGet || res.Replaced || res.Output.Stdout != raw {
		t.Fatalf("expected uncompacted passthrough when header absent: %+v", res)
	}

	// Nonzero exit never strips error output via piped family extraction.
	errOut := "error: connection refused\n"
	errIn := hookport.ToolOutput{Category: core.CategoryShell, Command: "kubectl get pods | grep Running", Stdout: errOut, HasStdout: true, ExitCode: 1}
	errRes := Compact(allCaps, errIn, Options{Keep: keepOK, Balanced: true})
	if errRes.Family != execwrap.FamilyNone || errRes.Replaced || errRes.Output.Stdout != errOut {
		t.Fatalf("nonzero exit piped command altered: %+v", errRes)
	}
}
