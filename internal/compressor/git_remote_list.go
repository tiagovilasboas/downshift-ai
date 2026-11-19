// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"strings"
)

type gitRemoteRow struct {
	name string
	url  string
	role string
}

// compressGitRemoteList compacts `git remote -v` output strictly losslessly by
// folding adjacent (fetch) and (push) rows for the same remote when both share
// the exact same URL into `<name>\t<url> (fetch, push)`. Any unrecognized line
// fails open and preserves input unchanged.
func compressGitRemoteList(input []byte) ([]byte, bool, string) {
	lines := commandLines(input)
	if len(lines) == 0 {
		return input, false, "git_remote_unrecognized_structure_preserved"
	}
	rows := make([]gitRemoteRow, 0, len(lines))
	for _, line := range lines {
		row, ok := parseGitRemoteVRow(line)
		if !ok {
			return input, false, "git_remote_unrecognized_structure_preserved"
		}
		rows = append(rows, row)
	}

	var b strings.Builder
	for i := 0; i < len(rows); i++ {
		cur := rows[i]
		if i+1 < len(rows) &&
			rows[i+1].name == cur.name &&
			rows[i+1].url == cur.url &&
			cur.role == "fetch" &&
			rows[i+1].role == "push" {
			fmt.Fprintf(&b, "%s\t%s (fetch, push)\n", cur.name, cur.url)
			i++
			continue
		}
		fmt.Fprintf(&b, "%s\t%s (%s)\n", cur.name, cur.url, cur.role)
	}
	out := []byte(b.String())
	if len(out) >= len(input) {
		return input, false, "no_compression_gain"
	}
	return out, true, fmt.Sprintf("compressed git remote list (%d rows)", len(rows))
}

func parseGitRemoteVRow(line string) (gitRemoteRow, bool) {
	if !reGitRemoteVRow.MatchString(line) {
		return gitRemoteRow{}, false
	}
	name, rest, ok := strings.Cut(line, "\t")
	if !ok {
		return gitRemoteRow{}, false
	}
	if url, ok := strings.CutSuffix(rest, " (fetch)"); ok {
		return gitRemoteRow{name: name, url: url, role: "fetch"}, true
	}
	if url, ok := strings.CutSuffix(rest, " (push)"); ok {
		return gitRemoteRow{name: name, url: url, role: "push"}, true
	}
	return gitRemoteRow{}, false
}
