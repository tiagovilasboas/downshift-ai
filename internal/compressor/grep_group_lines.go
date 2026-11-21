// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"strings"

	"github.com/tiagovilasboas/downshift-ai/internal/lazyre"
)

const (
	grepMinShare = 0.8  // share of non-empty lines that must be grep-shaped
	grepMinGain  = 0.15 // share of bytes the grouping must save
)

// reGrepLine matches `path:line:text` (numbered) and `path:text`. A path has
// no space and holds a dot or a separator, so "Note: x" and "http://x" are not
// paths; a Windows drive prefix (C:\x\y.go) is part of the path.
var reGrepLine = lazyre.New(`^((?:[A-Za-z]:)?[^:\s]+):(?:(\d+):)?(.*)$`)

type grepEntry struct{ num, text string }

type grepParsed struct {
	path string
	e    grepEntry
	ok   bool
}

// GroupGrepLines compacts grep-shaped output of a command no compressor
// recognises (a pipe, a compound command). When at least 80% of the non-empty
// lines are `path:line:text` or `path:text`, each path that repeats is written
// once as `path:` followed by indented `line:text` entries (`:text` when the
// line has no number). Every number and text is kept, in order within a path;
// paths appear where first seen. A line with a danger word, a line that does
// not match and a path seen once stay verbatim. A line over 400 bytes is cut in
// the middle like the balanced cut does (the raw output is in CCR). It reports
// false when the shape does not match or the gain is under 15%.
func GroupGrepLines(data []byte) ([]byte, bool) {
	if len(data) == 0 || len(data) > MaxProcessableBytes {
		return data, false
	}
	lines := strings.Split(string(data), "\n")
	ps, counts, ok := parseGrepLines(lines)
	if !ok {
		return data, false
	}
	groups := map[string][]grepEntry{}
	for _, p := range ps {
		if p.ok && counts[p.path] > 1 {
			groups[p.path] = append(groups[p.path], p.e)
		}
	}
	if len(groups) == 0 {
		return data, false
	}
	var b strings.Builder
	done := map[string]bool{}
	for i, l := range lines {
		p := ps[i]
		switch {
		case !p.ok || counts[p.path] < 2:
			b.WriteString(l)
		case done[p.path]:
			continue
		default:
			done[p.path] = true
			b.WriteString(p.path + ":")
			for _, e := range groups[p.path] {
				b.WriteString("\n  " + e.num + ":" + midCut(e.text))
			}
		}
		if i < len(lines)-1 {
			b.WriteByte('\n')
		}
	}
	out := b.String()
	if float64(len(data)-len(out)) < grepMinGain*float64(len(data)) {
		return data, false
	}
	return []byte(out), true
}

// parseGrepLines parses every line and counts lines per path. It reports false
// when under 80% of the non-empty lines are grep-shaped.
func parseGrepLines(lines []string) ([]grepParsed, map[string]int, bool) {
	ps := make([]grepParsed, len(lines))
	counts := map[string]int{}
	nonEmpty, shaped := 0, 0
	for i, l := range lines {
		if strings.TrimSpace(l) == "" {
			continue
		}
		nonEmpty++
		if reKeepLine.MatchString(l) {
			continue
		}
		m := reGrepLine.FindStringSubmatch(l)
		if m == nil || !strings.ContainsAny(m[1], `./\`) {
			continue
		}
		ps[i] = grepParsed{path: m[1], e: grepEntry{m[2], m[3]}, ok: true}
		counts[m[1]]++
		shaped++
	}
	return ps, counts, nonEmpty > 0 && float64(shaped) >= grepMinShare*float64(nonEmpty)
}
