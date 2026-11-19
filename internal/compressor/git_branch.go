// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"strings"

	"github.com/tiagovilasboas/downshift-ai/internal/lazyre"
)

// reGitBranchVerbosePad matches the column-alignment whitespace between a
// branch/ref name (or detached-HEAD label) and its commit hash or symref arrow
// in `git branch -v` / `-vv` output.
var reGitBranchVerbosePad = lazyre.New(`^((?:\([^)]+\)|\S+)) {2,}((?:[0-9a-f]{7,64}|->)(?: .+)?)$`)

type gitBranchLine struct {
	prefix string // "* ", "+ ", or "" (originally "  ")
	body   string
	group  string // "remotes/<remote>/" when eligible for factoring, else ""
	rest   string
}

// compressGitBranch compacts `git branch` output strictly losslessly: it
// strips the 2-space indent on non-current branches, collapses verbose SHA
// column padding without touching commit subjects, and factors repeated
// `remotes/<remote>/` prefixes under a section header. Any unrecognized line
// fails open and returns input unchanged.
func compressGitBranch(input []byte) ([]byte, bool, string) {
	lines := commandLines(input)
	if len(lines) == 0 {
		return input, false, "git_branch_unrecognized_structure_preserved"
	}
	parsed := make([]gitBranchLine, 0, len(lines))
	remoteCounts := map[string]int{}
	for _, line := range lines {
		if len(line) < 3 || strings.TrimSpace(line[2:]) == "" {
			return input, false, "git_branch_unrecognized_structure_preserved"
		}
		var prefix string
		switch line[:2] {
		case "* ", "+ ":
			prefix = line[:2]
		case "  ":
			prefix = ""
		default:
			return input, false, "git_branch_unrecognized_structure_preserved"
		}
		body := normalizeBranchBody(line[2:])
		var grp, rest string
		if prefix == "" {
			grp, rest = remoteBranchPrefix(body)
			if grp != "" {
				remoteCounts[grp]++
			}
		}
		parsed = append(parsed, gitBranchLine{prefix: prefix, body: body, group: grp, rest: rest})
	}

	var b strings.Builder
	curGroup := ""
	for _, e := range parsed {
		if e.group != "" && remoteCounts[e.group] >= 2 {
			if curGroup != e.group {
				curGroup = e.group
				b.WriteString(curGroup)
				b.WriteString(":\n")
			}
			b.WriteByte(' ')
			b.WriteString(e.rest)
			b.WriteByte('\n')
			continue
		}
		curGroup = ""
		b.WriteString(e.prefix)
		b.WriteString(e.body)
		b.WriteByte('\n')
	}
	out := []byte(b.String())
	if len(out) >= len(input) {
		return input, false, "no_compression_gain"
	}
	return out, true, fmt.Sprintf("compressed git branch (%d branches)", len(parsed))
}

func normalizeBranchBody(body string) string {
	if m := reGitBranchVerbosePad.FindStringSubmatch(body); m != nil {
		return m[1] + " " + m[2]
	}
	return body
}

func remoteBranchPrefix(body string) (group, rest string) {
	after, ok := strings.CutPrefix(body, "remotes/")
	if !ok {
		return "", ""
	}
	remote, tail, ok := strings.Cut(after, "/")
	if !ok || remote == "" || tail == "" || strings.ContainsRune(remote, ' ') {
		return "", ""
	}
	return "remotes/" + remote + "/", tail
}
