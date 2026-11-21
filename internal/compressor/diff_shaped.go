// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import "bytes"

// diffShapedMinGain is the least share of bytes DiffShaped must save; a
// smaller cut is not worth the rewrite.
const diffShapedMinGain = 0.15

// DiffShaped compresses output that carries "diff --git" blocks but did not
// come from a plain `git diff` or `git show` command, for example a diff piped
// through head or grep. It reuses the git diff rules: the text before the first
// "diff --git" line (a git show commit header, or whatever a pipe printed)
// stays verbatim; in the diff part only the "index" line is dropped and the
// unchanged context shrinks to 2 lines around each change. Every added or
// removed line, every file header, hunk header and "\ No newline" marker stays.
// A diff whose stray lines hold a danger word, or whose droppable context holds
// a secret, comes back unchanged (the git diff safety gate).
// It returns false when there is no diff block, the diff cannot be rewritten
// safely, or the gain is under 15%.
func DiffShaped(data []byte) ([]byte, bool) {
	start := diffStart(data)
	if start < 0 {
		return data, false
	}
	// The safety gate keeps a commit header as is, but text before a diff that
	// is not a commit header must not name a failure. Large diffs are not
	// summarised here: that path drops change lines, and this one never does.
	if _, safe := gitDiffUnsafe(data); !safe {
		return data, false
	}
	out, _, recognized := rewriteUnifiedDiff(data, true)
	if !recognized || !changeLinesPreserved(data, out) {
		return data, false
	}
	if float64(len(data)-len(out)) < diffShapedMinGain*float64(len(data)) {
		return data, false
	}
	return out, true
}

// diffStart returns the offset of the first line that opens a diff block, or
// -1 when the data holds none.
func diffStart(data []byte) int {
	if bytes.HasPrefix(data, []byte("diff --git a/")) {
		return 0
	}
	if i := bytes.Index(data, []byte("\ndiff --git a/")); i >= 0 {
		return i + 1
	}
	return -1
}
