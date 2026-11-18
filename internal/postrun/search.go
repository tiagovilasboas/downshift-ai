// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package postrun

import (
	"fmt"
	"strings"

	"github.com/tiagovilasboas/downshift-ai/internal/compressor"
	"github.com/tiagovilasboas/downshift-ai/internal/execwrap"
	"github.com/tiagovilasboas/downshift-ai/internal/hookport"
)

// SearchBudget bounds the text a cut search result keeps, before its marker:
// a result over execwrap.BalanceThreshold reaches the model under it.
const SearchBudget = 12 << 10

// compactSearch cuts a search tool's matches and its file list, each only
// when larger than the balanced threshold and only once its raw text is
// stored. Matches keep the first of each file and a count of the rest
// (compressor.BalanceMatches); a file list keeps its head. Each part is then
// held to SearchBudget, and says how much it left out and the retrieve id.
func compactSearch(in hookport.ToolOutput, o Options) Result {
	raw := len(in.Content)
	for _, p := range in.Paths {
		raw += len(p) + 1
	}
	if raw == 0 {
		return Result{}
	}
	res := Result{Considered: true, Family: FamilySearchTool, Output: in, RawBytes: int64(raw), Passthrough: execwrap.PassthroughSmallOutput}
	emitted := raw
	if !o.Balanced || o.Keep == nil {
		return settle(res, in)
	}
	threshold := balanceThreshold(o)
	if in.HasContent && len(in.Content) > threshold {
		if cut, id, ok := cutMatches(in.Content, o.Keep, o.Session); ok {
			res.Output.Content = cut
			recordKeptID(&res, id)
			emitted += len(cut) - len(in.Content)
		}
	}
	if in.HasPaths && len(strings.Join(in.Paths, "\n")) > threshold {
		if cut, id, ok := cutPaths(in.Paths, searchPathBudget(threshold), o.Keep, o.Session); ok {
			before := len(strings.Join(in.Paths, "\n")) + 1
			res.Output.Paths = cut
			emitted += len(strings.Join(cut, "\n")) + 1 - before
			recordKeptID(&res, id)
		}
	}
	if res.KeptID != "" {
		res.Compacted, res.Folded, res.Passthrough = true, true, ""
	}
	res.EmittedBytes = int64(emitted)
	return settle(res, in)
}

func searchPathBudget(threshold int) int {
	if threshold >= SearchBudget {
		return SearchBudget
	}
	b := threshold * 3 / 4
	if b < 2<<10 {
		return 2 << 10
	}
	return b
}

// cutMatches cuts match lines and stores the raw text first.
func cutMatches(text string, keep func([]byte) (string, bool), session string) (string, string, bool) {
	cut, applied := compressor.Balance(compressor.BalanceMatches, []byte(text))
	if !applied {
		return "", "", false
	}
	lines := strings.Split(strings.TrimSuffix(string(cut), "\n"), "\n")
	// The cut ends with its count of the rest per file ("… N more matches in
	// M files" and one line per file); that summary is kept whole and the
	// matches above it are held to what is left of the budget.
	i := len(lines)
	for j, l := range lines {
		if strings.HasPrefix(l, "… ") {
			i = j
			break
		}
	}
	summary := lines[i:]
	body := append(fitLines(lines[:i], "matches", SearchBudget-len(strings.Join(summary, "\n"))), summary...)
	id, ok := keep([]byte(text))
	if !ok {
		return "", "", false
	}
	arg := readRetrieveArg(session, id)
	body = append(body, strings.TrimSuffix(execwrap.MarkerWithID(len(text), len(strings.Join(body, "\n")), arg), "\n"))
	return strings.Join(body, "\n") + "\n", id, true
}

// cutPaths keeps the head of a file list and stores the whole list first.
func cutPaths(paths []string, budget int, keep func([]byte) (string, bool), session string) ([]string, string, bool) {
	body := fitLines(paths, "files", budget)
	if len(body) >= len(paths) {
		return nil, "", false
	}
	full := strings.Join(paths, "\n") + "\n"
	id, ok := keep([]byte(full))
	if !ok {
		return nil, "", false
	}
	arg := readRetrieveArg(session, id)
	return append(body, strings.TrimSuffix(execwrap.MarkerWithID(len(full), len(strings.Join(body, "\n")), arg), "\n")), id, true
}

// fitLines keeps whole lines while they fit in budget bytes and ends with one
// line that counts the rest (what names the unit: "matches", "files").
func fitLines(lines []string, what string, budget int) []string {
	size := 0
	for i, l := range lines {
		size += len(l) + 1
		if size > budget {
			out := append([]string{}, lines[:i]...)
			return append(out, fmt.Sprintf("… %d more %s (downshift retrieve)", len(lines)-i, what))
		}
	}
	return lines
}
