// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"strings"
	"testing"
)

func TestGhPrChecksSummary(t *testing.T) {
	in := strings.Join([]string{
		"build\tpass\t1m12s\thttps://github.com/x/y/runs/1",
		"lint\tfail\t0m48s\thttps://github.com/x/y/runs/2",
		"e2e\tpending\t0m00s\thttps://github.com/x/y/runs/3",
		"docs\tskipping\t0m00s\thttps://github.com/x/y/runs/4",
		"vet\tpass\t0m22s\thttps://github.com/x/y/runs/5",
	}, "\n") + "\n"

	res := CompressExit([]byte(in), ModeSafe, 0)
	if !res.Applied || res.Format != FormatGhChecks {
		t.Fatalf("expected gh_checks applied, got applied=%v format=%s reason=%q", res.Applied, res.Format, res.Reason)
	}
	out := string(res.Output)
	for _, want := range []string{
		"Summary: 2 pass, 1 fail, 1 pending, 1 skipped\n",
		"FAIL lint (fail)\n",
		"PEND e2e\n",
		"SKIP docs\n",
		"PASS build\n",
		"PASS vet\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Index(out, "FAIL lint") > strings.Index(out, "PASS build") {
		t.Errorf("expected FAIL before PASS:\n%s", out)
	}

	if failRes := CompressExit([]byte(in), ModeSafe, 1); failRes.Applied || string(failRes.Output) != in {
		t.Errorf("non-zero exit must preserve raw output")
	}
}

func TestGhPrDiffSummaryAndFallback(t *testing.T) {
	var big strings.Builder
	for f := 1; f <= 3; f++ {
		fmt.Fprintf(&big, "diff --git a/pkg/f%d.go b/pkg/f%d.go\nindex abc..def 100644\n--- a/pkg/f%d.go\n+++ b/pkg/f%d.go\n", f, f, f, f)
		fmt.Fprintf(&big, "@@ -1,45 +1,45 @@ func work%d() {\n", f)
		for i := 0; i < 40; i++ {
			fmt.Fprintf(&big, " context line %d\n", i)
		}
		fmt.Fprintf(&big, "-oldCall(%d)\n+newCall(%d)\n", f, f)
	}

	res := CompressCommandOutput([]byte(big.String()), FormatGhPrDiff, 0)
	if !res.Applied || res.Format != FormatGhPrDiff {
		t.Fatalf("expected gh_pr_diff applied, got applied=%v reason=%q", res.Applied, res.Reason)
	}
	out := string(res.Output)
	for _, want := range []string{
		"pkg/f1.go: +1 -1 (1 hunks)",
		"@@ -1,45 +1,45 @@ func work1() {",
		"Total: 3 files, +3 -3",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}

	// Small diff falls back to context-trimmed unified diff.
	small := "diff --git a/a.go b/a.go\nindex 111..222 100644\n--- a/a.go\n+++ b/a.go\n" +
		"@@ -1,9 +1,9 @@ func f() {\n c1\n c2\n c3\n c4\n-old()\n+new()\n c6\n c7\n c8\n c9\n"
	smallRes := CompressCommandOutput([]byte(small), FormatGhPrDiff, 0)
	if !smallRes.Applied || !strings.Contains(string(smallRes.Output), "+new()\n") {
		t.Fatalf("expected small diff to keep change lines, got applied=%v out=%q", smallRes.Applied, smallRes.Output)
	}

	// Danger line on stray text preserves raw.
	stray := big.String() + "fatal: could not read diff\n"
	if badRes := CompressCommandOutput([]byte(stray), FormatGhPrDiff, 0); badRes.Applied || string(badRes.Output) != stray {
		t.Errorf("expected stray danger line to preserve raw")
	}
}
