// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"strings"
	"testing"
)

func TestGitListCompaction(t *testing.T) {
	var branches strings.Builder
	branches.WriteString("* main\n")
	for i := 1; i <= 55; i++ {
		fmt.Fprintf(&branches, "  feat/task-%02d\n", i)
	}
	res := CompressExit([]byte(branches.String()), ModeSafe, 0)
	if !res.Applied || res.Format != FormatGitList {
		t.Fatalf("git branch: expected git_list applied, got applied=%v format=%s reason=%q", res.Applied, res.Format, res.Reason)
	}
	if !strings.Contains(string(res.Output), "... (31 more items)\n") {
		t.Errorf("git branch missing remaining count:\n%s", res.Output)
	}

	var lsTree strings.Builder
	hash := strings.Repeat("a", 40)
	for i := 1; i <= 50; i++ {
		fmt.Fprintf(&lsTree, "100644 blob %s\tsrc/file_%02d.go\n", hash, i)
	}
	res = CompressExit([]byte(lsTree.String()), ModeSafe, 0)
	if !res.Applied || res.Format != FormatGitList {
		t.Fatalf("git ls-tree: expected git_list applied, got applied=%v format=%s", res.Applied, res.Format)
	}

	var lsFiles strings.Builder
	for i := 1; i <= 50; i++ {
		fmt.Fprintf(&lsFiles, "internal/pkg/file_%02d.go\n", i)
	}
	res = CompressCommandOutput([]byte(lsFiles.String()), FormatGitList, 0)
	if !res.Applied || !strings.Contains(string(res.Output), "... (25 more items)\n") {
		t.Fatalf("git ls-files: expected compacted, got applied=%v reason=%q", res.Applied, res.Reason)
	}

	// Below threshold stays raw.
	short := "* main\n  feat/one\n  feat/two\n"
	if shortRes := CompressExit([]byte(short), ModeSafe, 0); shortRes.Applied || string(shortRes.Output) != short {
		t.Errorf("short list should stay raw: applied=%v", shortRes.Applied)
	}

	// Non-zero exit stays raw.
	if errRes := CompressExit([]byte(branches.String()), ModeSafe, 1); errRes.Applied {
		t.Errorf("non-zero exit must stay raw")
	}

	// Danger word in dropped tail stays raw.
	var dangerTail strings.Builder
	dangerTail.WriteString("* main\n")
	for i := 1; i <= 45; i++ {
		if i == 40 {
			dangerTail.WriteString("  fatal-error-branch\n")
		} else {
			fmt.Fprintf(&dangerTail, "  feat/b-%02d\n", i)
		}
	}
	if dRes := CompressExit([]byte(dangerTail.String()), ModeSafe, 0); dRes.Applied {
		t.Errorf("danger word in dropped tail must preserve raw")
	}
}

func TestWcAndFileOnlyLsDetection(t *testing.T) {
	var wc strings.Builder
	for i := 1; i <= 50; i++ {
		fmt.Fprintf(&wc, "%6d pkg/file_%02d.go\n", i*10, i)
	}
	wc.WriteString(" 12750 total\n")

	res := CompressExit([]byte(wc.String()), ModeSafe, 0)
	if !res.Applied || res.Format != FormatWc {
		t.Fatalf("wc: expected FormatWc applied, got applied=%v format=%s reason=%q", res.Applied, res.Format, res.Reason)
	}
	out := string(res.Output)
	if !strings.Contains(out, "... (25 more items)\n") || !strings.Contains(out, " 12750 total\n") {
		t.Errorf("wc output missing count or total:\n%s", out)
	}

	// ls -l with only regular files (no drwx) must still be recognized by isLsLong and compacted via FormatLs.
	lsFilesOnly := "total 24\n" +
		"-rw-r--r--  1 user  staff  1200 Nov 17 10:00 main.go\n" +
		"-rw-r--r--  1 user  staff   800 Nov 17 10:01 util.go\n"
	if !isLsLong([]byte(lsFilesOnly)) {
		t.Fatalf("isLsLong should recognize file-only ls -l output")
	}
	lsRes := CompressCommandOutput([]byte(lsFilesOnly), FormatLs, 0)
	if !lsRes.Applied || lsRes.Format != FormatLs {
		t.Fatalf("ls -l files only: expected FormatLs applied, got applied=%v format=%s reason=%q", lsRes.Applied, lsRes.Format, lsRes.Reason)
	}
}
