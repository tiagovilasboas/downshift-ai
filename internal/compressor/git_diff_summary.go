// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import "strings"

// gitDiffSummaryMinLines is the line count above which compressGitDiff and
// compressGitShow emit per-file +add/-del stats and hunk headers instead of
// full context-trimmed hunks.
const gitDiffSummaryMinLines = 120

// summarizeLargeDiff summarizes unified diffs with more than
// gitDiffSummaryMinLines lines into per-file +add/-del stats and hunk headers.
// When keepPreamble is true (git show), the commit metadata block before the
// first file section is preserved verbatim. Returns handled=false for small
// diffs or unrecognized structures, and a non-empty reason when a secret line
// forces fail-open preservation.
func summarizeLargeDiff(input []byte, keepPreamble bool) ([]byte, int, bool, string) {
	lines := strings.Split(strings.TrimSuffix(string(input), "\n"), "\n")
	if len(lines) <= gitDiffSummaryMinLines {
		return nil, 0, false, ""
	}
	if diffAnySecret(lines) {
		return nil, 0, true, "git_diff_secret_preserved"
	}
	firstDiff := -1
	for i, line := range lines {
		if reGitDiffHeader.MatchString(line) {
			firstDiff = i
			break
		}
	}
	if firstDiff < 0 {
		return nil, 0, false, ""
	}
	stats := parseGhDiffStats(lines[firstDiff:])
	if len(stats) == 0 {
		return nil, 0, false, ""
	}
	var b strings.Builder
	if keepPreamble && firstDiff > 0 {
		for _, line := range lines[:firstDiff] {
			b.WriteString(line)
			b.WriteByte('\n')
		}
	}
	b.WriteString(renderGhDiffStats(stats))
	return []byte(b.String()), len(stats), true, ""
}

// diffAnySecret reports whether any line in a large diff holds a secret-looking
// value. Large diff summaries drop change lines as well as context lines, so
// any secret anywhere in the diff must preserve the raw output.
func diffAnySecret(lines []string) bool {
	for _, line := range lines {
		if reGitDiffSecretValue.MatchString(line) {
			return true
		}
	}
	return false
}
