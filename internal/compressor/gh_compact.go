// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"strings"
)

const (
	// ghPrDiffMinLines is the line count above which compressGhPrDiff emits a
	// per-file stat and hunk summary instead of full context-trimmed hunks.
	ghPrDiffMinLines  = 120
	ghPrDiffMaxHunks  = 10
	ghChecksSumThresh = 10
)

type ghDiffFileStat struct {
	path  string
	adds  int
	dels  int
	hunks []string
}

// compressGhPrDiff summarizes large unified diffs (above ghPrDiffMinLines)
// into per-file +add/-del stats and hunk headers. Smaller diffs fall back to
// compressGitDiff. Danger words on stray lines or secrets in context stay raw.
func compressGhPrDiff(input []byte) ([]byte, bool, string) {
	if reason, ok := gitDiffUnsafe(input); !ok {
		return input, false, reason
	}
	lines := strings.Split(strings.TrimSuffix(string(input), "\n"), "\n")
	if len(lines) <= ghPrDiffMinLines {
		return compressGitDiff(input)
	}
	stats := parseGhDiffStats(lines)
	if len(stats) == 0 {
		return input, false, "gh_pr_diff_unrecognized_structure_preserved"
	}
	out := renderGhDiffStats(stats)
	if len(out) >= len(input) {
		return input, false, "no_compression_gain"
	}
	return []byte(out), true, fmt.Sprintf("compressed gh pr diff (%d files, %d lines)", len(stats), len(lines))
}

func parseGhDiffStats(lines []string) []ghDiffFileStat {
	var stats []ghDiffFileStat
	var cur *ghDiffFileStat
	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "diff --git a/"):
			path := diffHeaderPath(line)
			stats = append(stats, ghDiffFileStat{path: path})
			cur = &stats[len(stats)-1]
		case cur == nil:
		case strings.HasPrefix(line, "+++ b/"):
			cur.path = strings.TrimPrefix(line, "+++ b/")
		case reGitDiffHunk.MatchString(line):
			cur.hunks = append(cur.hunks, line)
		case strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++"):
			cur.adds++
		case strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---"):
			cur.dels++
		}
	}
	return stats
}

func diffHeaderPath(line string) string {
	rest := strings.TrimPrefix(line, "diff --git a/")
	if idx := strings.Index(rest, " b/"); idx >= 0 {
		return rest[idx+3:]
	}
	return rest
}

func renderGhDiffStats(stats []ghDiffFileStat) string {
	var b strings.Builder
	totalAdd, totalDel := 0, 0
	for _, f := range stats {
		totalAdd += f.adds
		totalDel += f.dels
		fmt.Fprintf(&b, "%s: +%d -%d (%d hunks)\n", f.path, f.adds, f.dels, len(f.hunks))
		limit := len(f.hunks)
		if limit > ghPrDiffMaxHunks {
			limit = ghPrDiffMaxHunks
		}
		for i := 0; i < limit; i++ {
			b.WriteString("  " + f.hunks[i] + "\n")
		}
		if len(f.hunks) > limit {
			fmt.Fprintf(&b, "  ... (%d more hunks)\n", len(f.hunks)-limit)
		}
	}
	fmt.Fprintf(&b, "Total: %d files, +%d -%d\n", len(stats), totalAdd, totalDel)
	return b.String()
}

// formatGhCheckRows formats parsed gh pr checks rows with FAIL/PEND/SKIP/PASS
// prefixes and an optional summary line when failures, pending checks, or many
// rows are present.
func formatGhCheckRows(checks []ghCheckRow) (string, int) {
	var failRows, pendRows, skipRows, passRows []ghCheckRow
	for _, c := range checks {
		switch {
		case c.fail:
			failRows = append(failRows, c)
		case c.state == "pending":
			pendRows = append(pendRows, c)
		case c.state == "skipping":
			skipRows = append(skipRows, c)
		default:
			passRows = append(passRows, c)
		}
	}
	var b strings.Builder
	if len(failRows) > 0 || len(pendRows) > 0 || len(checks) > ghChecksSumThresh {
		fmt.Fprintf(&b, "Summary: %d pass, %d fail", len(passRows), len(failRows))
		if len(pendRows) > 0 {
			fmt.Fprintf(&b, ", %d pending", len(pendRows))
		}
		if len(skipRows) > 0 {
			fmt.Fprintf(&b, ", %d skipped", len(skipRows))
		}
		b.WriteByte('\n')
	}
	for _, c := range failRows {
		fmt.Fprintf(&b, "FAIL %s (%s)\n", c.name, c.state)
	}
	for _, c := range pendRows {
		fmt.Fprintf(&b, "PEND %s\n", c.name)
	}
	for _, c := range skipRows {
		fmt.Fprintf(&b, "SKIP %s\n", c.name)
	}
	for _, c := range passRows {
		fmt.Fprintf(&b, "PASS %s\n", c.name)
	}
	return b.String(), len(failRows)
}

type ghCheckRow struct {
	name  string
	state string
	fail  bool
}
