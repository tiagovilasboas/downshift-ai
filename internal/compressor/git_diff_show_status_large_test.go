// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/compressor"
)

func buildLargeGitDiff(files, linesPerHunk int) []byte {
	var b strings.Builder
	for f := 0; f < files; f++ {
		path := fmt.Sprintf("internal/pkg/module_%02d.go", f)
		fmt.Fprintf(&b, "diff --git a/%s b/%s\n", path, path)
		fmt.Fprintf(&b, "index 1234567..89abcdef 100644\n")
		fmt.Fprintf(&b, "--- a/%s\n", path)
		fmt.Fprintf(&b, "+++ b/%s\n", path)
		fmt.Fprintf(&b, "@@ -10,%d +10,%d @@ func process%d() {\n", linesPerHunk+3, linesPerHunk+3, f)
		b.WriteString(" \tctx := context.Background()\n")
		for i := 0; i < linesPerHunk; i++ {
			fmt.Fprintf(&b, "-\toldValue%d := computeOld(%d)\n", i, i)
			fmt.Fprintf(&b, "+\tnewValue%d := computeNew(ctx, %d)\n", i, i)
		}
		b.WriteString(" \treturn nil\n")
	}
	return []byte(b.String())
}

func buildLargeGitStatus(staged, unstaged, untracked int) []byte {
	var b strings.Builder
	b.WriteString("On branch feat/large-status\n")
	b.WriteString("Your branch is ahead of 'origin/feat/large-status' by 2 commits.\n")
	b.WriteString("  (use \"git push\" to publish your local commits)\n\n")
	if staged > 0 {
		b.WriteString("Changes to be committed:\n")
		b.WriteString("  (use \"git restore --staged <file>...\" to unstage)\n")
		for i := 0; i < staged; i++ {
			fmt.Fprintf(&b, "\tmodified:   internal/staged/file_%02d.go\n", i)
		}
		b.WriteByte('\n')
	}
	if unstaged > 0 {
		b.WriteString("Changes not staged for commit:\n")
		b.WriteString("  (use \"git add <file>...\" to update what will be committed)\n")
		for i := 0; i < unstaged; i++ {
			fmt.Fprintf(&b, "\tmodified:   internal/unstaged/file_%02d.go\n", i)
		}
		b.WriteByte('\n')
	}
	if untracked > 0 {
		b.WriteString("Untracked files:\n")
		b.WriteString("  (use \"git add <file>...\" to include in what will be committed)\n")
		for i := 0; i < untracked; i++ {
			fmt.Fprintf(&b, "\tscratch/notes_%02d.txt\n", i)
		}
	}
	return []byte(b.String())
}

func TestLargeGitDiff_RatioAndStats(t *testing.T) {
	in := buildLargeGitDiff(5, 20) // > 200 lines
	res := compressor.CompressExit(in, compressor.ModeSafe, 0)
	if !res.Applied || res.Format != compressor.FormatGitDiff {
		t.Fatalf("expected FormatGitDiff applied=true, got applied=%v format=%s reason=%q", res.Applied, res.Format, res.Reason)
	}
	ratio := float64(len(res.Output)) / float64(len(in))
	if ratio > 0.50 {
		t.Fatalf("ratio %.3f > 0.50 (%d -> %d bytes)", ratio, len(in), len(res.Output))
	}
	out := string(res.Output)
	for _, want := range []string{
		"internal/pkg/module_00.go: +20 -20 (1 hunks)",
		"@@ -10,23 +10,23 @@ func process0() {",
		"Total: 5 files, +100 -100",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output:\n%s", want, out)
		}
	}
	// Idempotent: re-compressing compacted output leaves it unchanged.
	twice := compressor.CompressExit(res.Output, compressor.ModeSafe, 0)
	if !bytes.Equal(twice.Output, res.Output) {
		t.Fatalf("large git diff compaction not idempotent")
	}
}

func TestLargeGitShow_RatioAndPreamble(t *testing.T) {
	preamble := "commit 0123456789abcdef0123456789abcdef01234567\n" +
		"Author: Developer <dev@example.com>\n" +
		"Date:   Tue Nov 17 15:00:00 2026 -0300\n\n" +
		"    refactor: tighten module processing\n\n"
	in := append([]byte(preamble), buildLargeGitDiff(4, 25)...)
	res := compressor.CompressExit(in, compressor.ModeSafe, 0)
	if !res.Applied || res.Format != compressor.FormatGitShow {
		t.Fatalf("expected FormatGitShow applied=true, got applied=%v format=%s reason=%q", res.Applied, res.Format, res.Reason)
	}
	ratio := float64(len(res.Output)) / float64(len(in))
	if ratio > 0.50 {
		t.Fatalf("ratio %.3f > 0.50 (%d -> %d bytes)", ratio, len(in), len(res.Output))
	}
	out := string(res.Output)
	for _, want := range []string{
		"commit 0123456789abcdef0123456789abcdef01234567",
		"refactor: tighten module processing",
		"internal/pkg/module_00.go: +25 -25 (1 hunks)",
		"Total: 4 files, +100 -100",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output:\n%s", want, out)
		}
	}
	twice := compressor.CompressExit(res.Output, compressor.ModeSafe, 0)
	if !bytes.Equal(twice.Output, res.Output) {
		t.Fatalf("large git show compaction not idempotent")
	}
}

func TestLargeGitDiff_PreservesDangerAndSecrets(t *testing.T) {
	t.Run("stray danger line", func(t *testing.T) {
		in := append(buildLargeGitDiff(4, 20), []byte("error: unable to read object\n")...)
		res := compressor.CompressExit(in, compressor.ModeSafe, 0)
		if res.Applied || !bytes.Equal(res.Output, in) {
			t.Fatalf("expected raw preservation, got applied=%v reason=%q", res.Applied, res.Reason)
		}
	})
	t.Run("secret on change line in large diff", func(t *testing.T) {
		in := append(buildLargeGitDiff(4, 20), []byte("diff --git a/cfg.go b/cfg.go\n--- a/cfg.go\n+++ b/cfg.go\n@@ -1,2 +1,2 @@\n-old = 1\n+api_key = \"supersecret\"\n")...)
		res := compressor.CompressExit(in, compressor.ModeSafe, 0)
		if res.Applied || !bytes.Equal(res.Output, in) || res.Reason != "git_diff_secret_preserved" {
			t.Fatalf("expected secret preservation, got applied=%v reason=%q", res.Applied, res.Reason)
		}
	})
	t.Run("small diff keeps change lines verbatim", func(t *testing.T) {
		in := buildLargeGitDiff(1, 5) // <= 120 lines
		res := compressor.CompressExit(in, compressor.ModeSafe, 0)
		if !res.Applied {
			t.Fatalf("expected small diff to compact, reason=%q", res.Reason)
		}
		if !strings.Contains(string(res.Output), "+	newValue0 := computeNew(ctx, 0)") {
			t.Fatalf("small diff dropped change line:\n%s", res.Output)
		}
	})
}

func TestLargeGitStatus_RatioAndGrouping(t *testing.T) {
	in := buildLargeGitStatus(15, 20, 12)
	res := compressor.CompressExit(in, compressor.ModeSafe, 0)
	if !res.Applied || res.Format != compressor.FormatGitStatus {
		t.Fatalf("expected FormatGitStatus applied=true, got applied=%v format=%s reason=%q", res.Applied, res.Format, res.Reason)
	}
	ratio := float64(len(res.Output)) / float64(len(in))
	if ratio > 0.50 {
		t.Fatalf("ratio %.3f > 0.50 (%d -> %d bytes)", ratio, len(in), len(res.Output))
	}
	out := string(res.Output)
	for _, want := range []string{
		"* feat/large-status...origin/feat/large-status [ahead 2]",
		"Summary: 15 staged, 20 unstaged, 12 untracked",
		"Staged (15):",
		"  ... (+10 more)",
		"Unstaged (20):",
		"  ... (+15 more)",
		"Untracked (12):",
		"  ... (+7 more)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output:\n%s", want, out)
		}
	}
	twice := compressor.CompressExit(res.Output, compressor.ModeSafe, 0)
	if !bytes.Equal(twice.Output, res.Output) {
		t.Fatalf("large git status compaction not idempotent")
	}
}

func TestLargeGitStatus_PreservesConflictsAndSmall(t *testing.T) {
	t.Run("conflicts stay raw", func(t *testing.T) {
		in := append(buildLargeGitStatus(10, 10, 10), []byte("\nUnmerged paths:\n\tboth modified:   conflict.go\n")...)
		res := compressor.CompressExit(in, compressor.ModeSafe, 0)
		if res.Applied || !bytes.Equal(res.Output, in) {
			t.Fatalf("expected conflict status preserved raw, got applied=%v reason=%q", res.Applied, res.Reason)
		}
	})
	t.Run("small status keeps every file", func(t *testing.T) {
		in := buildLargeGitStatus(2, 2, 1) // <= 30 lines
		res := compressor.CompressExit(in, compressor.ModeSafe, 0)
		if !res.Applied {
			t.Fatalf("expected small status to compact, reason=%q", res.Reason)
		}
		if !strings.Contains(string(res.Output), "internal/unstaged/file_01.go") {
			t.Fatalf("small status dropped file:\n%s", res.Output)
		}
	})
}
