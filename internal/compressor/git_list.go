// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"strings"

	"github.com/tiagovilasboas/downshift-ai/internal/lazyre"
)

const (
	// gitListThreshold is the minimum line count before list outputs are
	// head-truncated with a remaining-item count.
	gitListThreshold = 40
	gitListHead      = 25
)

var (
	reGitLsTreeRow   = lazyre.New(`^(?:100644|100755|120000|040000|160000) (?:blob|tree|commit) [0-9a-f]{40}\t.+$`)
	reGitWorktreeRow = lazyre.New(`^/\S+\s+[0-9a-f]{7,40}\s+(?:\[[^\]]+\]|\(bare\)|\(detached HEAD\))$`)
	reGitRemoteVRow  = lazyre.New(`^\S+\t\S+ \((?:fetch|push)\)$`)
	reWcRow          = lazyre.New(`^\s*\d+(?:\s+\d+)*\s+\S+$`)
	reWcTotal        = lazyre.New(`^\s*\d+(?:\s+\d+)*\s+total$`)
)

// isGitList detects unambiguous multi-line (> gitListThreshold) outputs from
// git ls-tree, git branch, git worktree list, and git remote -v.
func isGitList(input []byte) bool {
	lines := nonEmptyLines(input)
	if len(lines) <= gitListThreshold {
		return false
	}
	return allLinesMatch(lines, reGitLsTreeRow.MatchString) ||
		isGitBranchList(lines) ||
		allLinesMatch(lines, reGitWorktreeRow.MatchString) ||
		allLinesMatch(lines, reGitRemoteVRow.MatchString)
}

func isGitBranchList(lines []string) bool {
	hasCurrent := false
	for _, l := range lines {
		switch {
		case strings.HasPrefix(l, "* ") || strings.HasPrefix(l, "+ "):
			hasCurrent = true
		case strings.HasPrefix(l, "  "):
		default:
			return false
		}
	}
	return hasCurrent
}

// compressGitList compacts git list outputs (git ls-files, git ls-tree,
// git branch, git worktree, git remote) exceeding gitListThreshold lines to the
// first gitListHead lines plus a remaining count.
func compressGitList(input []byte) ([]byte, bool, string) {
	lines := commandLines(input)
	if len(lines) <= gitListThreshold {
		return input, false, "no_compression_gain"
	}
	for _, l := range lines[gitListHead:] {
		if reDangerWord.MatchString(l) {
			return input, false, "git_list_danger_word_preserved"
		}
	}
	var b strings.Builder
	for _, l := range lines[:gitListHead] {
		b.WriteString(l + "\n")
	}
	fmt.Fprintf(&b, "... (%d more items)\n", len(lines)-gitListHead)
	out := []byte(b.String())
	if len(out) >= len(input) {
		return input, false, "no_compression_gain"
	}
	return out, true, fmt.Sprintf("compressed git list (%d of %d items)", gitListHead, len(lines))
}

// isWcOutput detects multi-line wc outputs exceeding gitListThreshold lines.
func isWcOutput(input []byte) bool {
	lines := nonEmptyLines(input)
	if len(lines) <= gitListThreshold {
		return false
	}
	return allLinesMatch(lines, reWcRow.MatchString)
}

// compressWc keeps the first gitListHead lines of a long wc output, counts the
// remaining files, and preserves the trailing total line if present.
func compressWc(input []byte) ([]byte, bool, string) {
	lines := commandLines(input)
	if len(lines) <= gitListThreshold {
		return input, false, "no_compression_gain"
	}
	hasTotal := reWcTotal.MatchString(lines[len(lines)-1])
	bodyLen := len(lines)
	if hasTotal {
		bodyLen--
	}
	if bodyLen <= gitListHead {
		return input, false, "no_compression_gain"
	}
	for _, l := range lines[gitListHead:bodyLen] {
		if reDangerWord.MatchString(l) {
			return input, false, "wc_danger_word_preserved"
		}
	}
	var b strings.Builder
	for _, l := range lines[:gitListHead] {
		b.WriteString(l + "\n")
	}
	fmt.Fprintf(&b, "... (%d more items)\n", bodyLen-gitListHead)
	if hasTotal {
		b.WriteString(lines[len(lines)-1] + "\n")
	}
	out := []byte(b.String())
	if len(out) >= len(input) {
		return input, false, "no_compression_gain"
	}
	return out, true, fmt.Sprintf("compressed wc output (%d of %d items)", gitListHead, bodyLen)
}

func nonEmptyLines(input []byte) []string {
	var out []string
	for _, l := range strings.Split(strings.TrimSuffix(string(input), "\n"), "\n") {
		if strings.TrimSpace(l) != "" {
			out = append(out, l)
		}
	}
	return out
}

func allLinesMatch(lines []string, fn func(string) bool) bool {
	for _, l := range lines {
		if !fn(l) {
			return false
		}
	}
	return len(lines) > 0
}
