// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"bytes"
	"fmt"
	"strings"
	"unicode/utf8"
)

const (
	noisyMatchPerFile = 5
	noisyMatchTotal   = 30
)

type matchFileGroup struct {
	file string
	rows []string
}

type matchDirGroup struct {
	dir   string
	files int
	rows  []string
}

// CompactSearchMatches groups Grep-style match lines by file or directory,
// keeping top matches and per-file counts verbatim. It falls back to
// BalanceMatches for unstructured text and never returns output >= len(text).
func CompactSearchMatches(text []byte, maxBytes int) ([]byte, bool) {
	if len(text) == 0 || len(text) > MaxProcessableBytes || bytes.IndexByte(text, 0) != -1 || !utf8.Valid(text) {
		return text, false
	}
	lines := strings.Split(strings.TrimSuffix(string(text), "\n"), "\n")
	if out, ok, isCounts := compactSearchCounts(lines, len(text), maxBytes); isCounts {
		return out, ok && len(out) < len(text)
	}
	groups, total, maxInFile, ok := parseSearchMatchGroups(lines)
	if !ok {
		return Balance(BalanceMatches, text)
	}
	if total > balanceMatchLines || maxInFile > balanceMatchPerFile {
		out := renderGroupedMatches(groups, balanceMatchPerFile, balanceMatchLines)
		if len(out) >= len(text) {
			return text, false
		}
		return out, true
	}
	var cand []byte
	if total >= len(groups)*2 {
		cand = renderGroupedMatches(groups, total, total)
	} else if dirOut, ok := renderMatchesByDir(groups); ok {
		cand = dirOut
	}
	if len(cand) > 0 && (maxBytes <= 0 || len(cand) <= maxBytes) && len(cand)*4 <= len(text)*3 {
		return cand, true
	}
	out := renderGroupedMatches(groups, noisyMatchPerFile, noisyMatchTotal)
	if len(out) >= len(text) {
		return text, false
	}
	return out, true
}

func parseSearchMatchGroups(lines []string) ([]*matchFileGroup, int, int, bool) {
	known := map[string]bool{}
	for _, l := range lines {
		if k, ok := matchRowKey(l); ok {
			known[k] = true
		}
	}
	byFile := map[string]*matchFileGroup{}
	var groups []*matchFileGroup
	cur, parsed, total, loose := "", 0, 0, 0
	for _, l := range lines {
		if isSeparator(l) {
			continue
		}
		total++
		var file, row string
		var ok bool
		if k, matched := contextOrMatchKey(l, known); matched {
			file, row, ok = k, trimMatchRow(l[len(k)+1:]), true
		} else if k, text, matched := parseUnnumberedRow(l); matched {
			file, row, ok = k, strings.TrimLeft(text, " \t"), true
		}
		if ok {
			parsed++
			loose = 0
		} else {
			loose++
			if cur == "" || loose > searchCapMaxLoose {
				return nil, 0, 0, false
			}
			file, row = cur, strings.TrimLeft(l, " \t")
		}
		g := byFile[file]
		if g == nil {
			g = &matchFileGroup{file: file}
			byFile[file] = g
			groups = append(groups, g)
		}
		g.rows = append(g.rows, row)
		cur = file
	}
	if total == 0 || parsed*10 < total*8 {
		return nil, 0, 0, false
	}
	maxInFile := 0
	for _, g := range groups {
		if len(g.rows) > maxInFile {
			maxInFile = len(g.rows)
		}
	}
	return groups, parsed, maxInFile, true
}

func trimMatchRow(rest string) string {
	if i := strings.IndexByte(rest, ':'); i > 0 {
		return rest[:i+1] + " " + strings.TrimLeft(rest[i+1:], " \t")
	}
	if i := strings.IndexByte(rest, '-'); i > 0 {
		return rest[:i+1] + " " + strings.TrimLeft(rest[i+1:], " \t")
	}
	return strings.TrimLeft(rest, " \t")
}

func parseUnnumberedRow(l string) (string, string, bool) {
	file, text, ok := strings.Cut(l, ":")
	if !ok || file == "" || strings.ContainsAny(file, " \t") {
		return "", "", false
	}
	if strings.HasPrefix(text, "//") {
		return "", "", false // URL scheme like https://
	}
	if !strings.ContainsAny(file, "/\\.") {
		return "", "", false
	}
	return file, text, true
}

func renderGroupedMatches(groups []*matchFileGroup, perFile, totalCap int) []byte {
	var b strings.Builder
	cutCounts := map[string]int{}
	var cutOrder []string
	shown, cutTotal := 0, 0
	for _, g := range groups {
		allow := min(perFile, totalCap-shown)
		if allow < 0 {
			allow = 0
		}
		take := min(len(g.rows), allow)
		if take > 0 {
			shown += take
			if take == 1 && len(g.rows) == 1 {
				b.WriteString(balanceCut(g.file+":"+g.rows[0]) + "\n")
			} else {
				fmt.Fprintf(&b, "%s (%d):\n", g.file, len(g.rows))
				for _, r := range g.rows[:take] {
					b.WriteString(" " + balanceCut(r) + "\n")
				}
			}
		}
		if rem := len(g.rows) - take; rem > 0 {
			cutTotal += rem
			cutOrder = append(cutOrder, g.file)
			cutCounts[g.file] = rem
		}
	}
	if cutTotal > 0 {
		fmt.Fprintf(&b, "… %d more matches in %d files (downshift retrieve):\n", cutTotal, len(cutOrder))
		for i, f := range cutOrder {
			if i == 30 {
				fmt.Fprintf(&b, "  … and %d more files\n", len(cutOrder)-30)
				break
			}
			fmt.Fprintf(&b, "  %s: %d more\n", f, cutCounts[f])
		}
	}
	return []byte(b.String())
}

func renderMatchesByDir(groups []*matchFileGroup) ([]byte, bool) {
	byDir := map[string]*matchDirGroup{}
	var order []string
	for _, g := range groups {
		dir, base := splitDirBase(g.file)
		dg := byDir[dir]
		if dg == nil {
			dg = &matchDirGroup{dir: dir}
			byDir[dir] = dg
			order = append(order, dir)
		}
		dg.files++
		for _, r := range g.rows {
			dg.rows = append(dg.rows, base+":"+r)
		}
	}
	if len(order) >= len(groups) {
		return nil, false
	}
	var b strings.Builder
	for _, dir := range order {
		dg := byDir[dir]
		if dg.files == 1 || dir == "." {
			for _, r := range dg.rows {
				prefix := dir
				if dir == "." {
					prefix = ""
				}
				b.WriteString(balanceCut(prefix+r) + "\n")
			}
			continue
		}
		fmt.Fprintf(&b, "%s (%d matches in %d files):\n", dir, len(dg.rows), dg.files)
		for _, r := range dg.rows {
			b.WriteString(" " + balanceCut(r) + "\n")
		}
	}
	return []byte(b.String()), true
}
