// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"bytes"
	"strings"
)

const (
	pathPrefixMinRun   = 3    // shortest run of same-parent lines that is grouped
	pathPrefixMinPaths = 0.8  // share of non-empty lines that must be bare paths
	pathPrefixMinGain  = 0.10 // smallest saving, as a share of the input
	pathPrefixIndent   = "  "
)

// FactorPathPrefixes factors the directory prefix out of a path list, with no
// loss. Output format:
//
//	dir/            a header: a line that is not indented and is followed by
//	  a.go          one or more lines indented by two spaces
//	  b.go
//	README.md       any other line is copied as it was
//
// The parent of a path is everything up to and including its last "/" or "\"
// (both are separators, so Windows output works and the separator is kept).
// To decode: a non-indented line followed by an indented line is a header, and
// each indented line is header + the text after its two-space indent; every
// other line is the original line. Order and every byte of every path are kept.
// Only runs of three or more consecutive lines with the same parent are
// grouped, and only when that run gets shorter.
//
// It returns the input and false when fewer than 80% of the non-empty lines are
// bare paths (a line with a tab, a double space, a leading or trailing space,
// or an "ls -l" row is not), when a line starts with whitespace (the indent
// would be ambiguous), when the input has a carriage return, or when the saving
// is under 10%.
func FactorPathPrefixes(data []byte) ([]byte, bool) {
	if len(data) == 0 || bytes.IndexByte(data, '\r') >= 0 {
		return data, false
	}
	trailingNL := data[len(data)-1] == '\n'
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	paths, nonEmpty := 0, 0
	for _, l := range lines {
		if l == "" {
			continue
		}
		if l[0] == ' ' || l[0] == '\t' {
			return data, false
		}
		nonEmpty++
		if isBarePath(l) {
			paths++
		}
	}
	if nonEmpty == 0 || float64(paths) < pathPrefixMinPaths*float64(nonEmpty) {
		return data, false
	}
	var b strings.Builder
	for i := 0; i < len(lines); {
		dir := parentDir(lines[i])
		j := i + 1
		for dir != "" && j < len(lines) && isBarePath(lines[j]) && parentDir(lines[j]) == dir {
			j++
		}
		n := j - i
		// Grouping costs a header line and an indent per path, and saves dir per path.
		if n >= pathPrefixMinRun && n*(len(dir)-len(pathPrefixIndent)) > len(dir)+1 {
			b.WriteString(dir + "\n")
			for _, l := range lines[i:j] {
				b.WriteString(pathPrefixIndent + l[len(dir):] + "\n")
			}
		} else {
			for _, l := range lines[i:j] {
				b.WriteString(l + "\n")
			}
		}
		i = j
	}
	out := []byte(b.String())
	if !trailingNL {
		out = out[:len(out)-1]
	}
	if float64(len(data)-len(out)) < pathPrefixMinGain*float64(len(data)) {
		return data, false
	}
	return out, true
}

// isBarePath reports whether l is one path and nothing else: no tab, no double
// space, no edge space, and not an "ls -l" row.
func isBarePath(l string) bool {
	if l == "" || l[0] == ' ' || l[len(l)-1] == ' ' {
		return false
	}
	if strings.Contains(l, "\t") || strings.Contains(l, "  ") {
		return false
	}
	return !reLsRow.MatchString(l)
}

// parentDir returns p up to and including its last "/" or "\", or "" when p has
// no separator or is made of the separator alone.
func parentDir(p string) string {
	i := strings.LastIndexAny(p, `/\`)
	if i < 0 || i == len(p)-1 {
		return ""
	}
	return p[:i+1]
}
