// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package postrun

import (
	"os"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/core"
	"github.com/tiagovilasboas/downshift-ai/internal/hookport"
	"github.com/tiagovilasboas/downshift-ai/internal/paths"
)

// TestMain keeps the dedup records of every test in this package out of the
// user's real state dir.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "postrun-state-")
	if err != nil {
		os.Exit(1)
	}
	os.Setenv(paths.EnvStateDir, dir)
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

var sessA = strings.Repeat("ab", 32)

func seenKeep(t *testing.T) (func([]byte) (string, bool), *int) {
	n := 0
	return func([]byte) (string, bool) { n++; return "seenid123456", true }, &n
}

func shellRead(cmd, out string) hookport.ToolOutput {
	return hookport.ToolOutput{Category: core.CategoryShell, Command: cmd, Stdout: out, HasStdout: true}
}

func TestSeenRead_ShellRepeatIsDeduped(t *testing.T) {
	t.Setenv(paths.EnvStateDir, t.TempDir())
	keep, _ := seenKeep(t)
	o := Options{Keep: keep, Session: sessA}
	body := strings.Repeat("package x // some file text\n", 200)

	first := Compact(allCaps, shellRead("cat a.go", body), o)
	if first.Replaced {
		t.Fatalf("first read must pass: %+v", first)
	}
	second := Compact(allCaps, shellRead("cat a.go", body), o)
	if !second.Replaced || !second.Compacted || second.KeptID != "seenid123456" {
		t.Fatalf("second identical read must be deduped: %+v", second)
	}
	if !strings.Contains(second.Output.Stdout, "unchanged") || !strings.Contains(second.Output.Stdout, "downshift retrieve --session=") || !strings.Contains(second.Output.Stdout, "seenid123456") {
		t.Fatalf("pointer text: %q", second.Output.Stdout)
	}
	if second.EmittedBytes >= second.RawBytes {
		t.Fatalf("no saving recorded: %+v", second)
	}
	if res := Compact(allCaps, shellRead("cat a.go", body+"changed\n"), o); res.Replaced && strings.Contains(res.Output.Stdout, "unchanged") {
		t.Fatalf("changed content must pass: %q", res.Output.Stdout)
	}
	if res := Compact(allCaps, shellRead("cat a.go", body), Options{Keep: keep, Session: strings.Repeat("cd", 32)}); res.Replaced {
		t.Fatalf("different session must pass: %+v", res)
	}
}

func TestSeenRead_SmallOrNonReadOrNoKeepPasses(t *testing.T) {
	t.Setenv(paths.EnvStateDir, t.TempDir())
	keep, n := seenKeep(t)
	o := Options{Keep: keep, Session: sessA}
	small := strings.Repeat("x\n", 200) // under 512 B
	for i := 0; i < 2; i++ {
		if res := Compact(allCaps, shellRead("cat a.go", small), o); res.Replaced {
			t.Fatalf("small output must pass: %+v", res)
		}
	}
	if *n != 0 {
		t.Fatalf("small output must not be stored, stored %d", *n)
	}
	big := strings.Repeat("ok line of test output\n", 200)
	for i := 0; i < 2; i++ {
		if res := Compact(allCaps, shellRead("go build ./...", big), o); res.Replaced && strings.Contains(res.Output.Stdout, "unchanged") {
			t.Fatalf("non file-read command must not dedupe: %+v", res)
		}
	}
	text := strings.Repeat("line of file text\n", 200)
	for i := 0; i < 2; i++ {
		if res := Compact(allCaps, shellRead("cat a.go", text), Options{Session: sessA}); res.Replaced {
			t.Fatalf("no CCR store means no dedup: %+v", res)
		}
	}
}

func TestSeenRead_ReadToolRepeatIsDeduped(t *testing.T) {
	t.Setenv(paths.EnvStateDir, t.TempDir())
	keep, _ := seenKeep(t)
	o := Options{Keep: keep, Session: sessA}
	in := hookport.ToolOutput{Category: core.CategoryRead, Content: strings.Repeat("1\tline of file\n", 300), HasContent: true}
	if res := Compact(allCaps, in, o); res.Replaced {
		t.Fatalf("first read must pass: %+v", res)
	}
	res := Compact(allCaps, in, o)
	if !res.Replaced || !strings.Contains(res.Output.Content, "unchanged") || res.KeptID != "seenid123456" {
		t.Fatalf("second read must be deduped: %+v", res)
	}
}

func TestSeenRead_MidBand512To2KBDeduped(t *testing.T) {
	t.Setenv(paths.EnvStateDir, t.TempDir())
	keep, _ := seenKeep(t)
	o := Options{Keep: keep, Session: sessA}
	body := strings.Repeat("1\tline of file\n", 50) // 750 B: in [512, 2048)
	if len(body) < 512 || len(body) >= 2<<10 {
		t.Fatalf("fixture len = %d, want in [512, 2048)", len(body))
	}
	in := hookport.ToolOutput{Category: core.CategoryRead, Content: body, HasContent: true}
	first := Compact(allCaps, in, o)
	if first.Replaced || first.Output.Content != body {
		t.Fatalf("first mid-band Read must stay verbatim: %+v", first)
	}
	second := Compact(allCaps, in, o)
	if !second.Replaced || !strings.Contains(second.Output.Content, "unchanged") || second.KeptID != "seenid123456" || second.EmittedBytes >= second.RawBytes {
		t.Fatalf("second mid-band Read not deduped: %+v", second)
	}
}
