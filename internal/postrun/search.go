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

const (
	// SearchBudget bounds the text a cut search result keeps, before its marker:
	// a result over execwrap.BalanceThreshold reaches the model under it.
	SearchBudget = 12 << 10
	// SearchThresholdBytes and SearchThresholdLines trigger structured Grep and
	// Glob compaction when a result exceeds either limit.
	SearchThresholdBytes = 2 << 10
	SearchThresholdLines = 40
	searchPathMaxBytes   = 6 << 10
)

// compactSearch cuts a search tool's matches and its file list when larger
// than the search thresholds and only once its raw text is stored. Matches are
// grouped by file or directory with top matches and counts verbatim; file
// lists are grouped by directory with representative samples and counts.
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
	byteThresh, lineThresh := searchThresholds(o)
	if in.HasContent && exceedsSearchThreshold(in.Content, byteThresh, lineThresh) {
		if cut, id, ok := cutMatches(in.Content, o.Keep, o.Session); ok {
			res.Output.Content = cut
			recordKeptID(&res, id)
			emitted += len(cut) - len(in.Content)
		}
	}
	if in.HasPaths && exceedsPathsThreshold(in.Paths, byteThresh, lineThresh) {
		if cut, id, ok := cutPaths(in.Paths, searchPathBudget(byteThresh), o.Keep, o.Session); ok {
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

func searchThresholds(o Options) (byteThresh, lineThresh int) {
	if o.BalanceThreshold > 0 && o.BalanceThreshold != PostRunBalanceThreshold {
		return o.BalanceThreshold, SearchThresholdLines
	}
	return SearchThresholdBytes, SearchThresholdLines
}

func exceedsSearchThreshold(text string, byteThresh, lineThresh int) bool {
	if len(text) > byteThresh {
		return true
	}
	if lineThresh <= 0 || text == "" {
		return false
	}
	return strings.Count(strings.TrimSuffix(text, "\n"), "\n")+1 > lineThresh
}

func exceedsPathsThreshold(paths []string, byteThresh, lineThresh int) bool {
	if lineThresh > 0 && len(paths) > lineThresh {
		return true
	}
	size := max(0, len(paths)-1)
	for _, p := range paths {
		size += len(p)
	}
	return size > byteThresh
}

func searchPathBudget(threshold int) int {
	if threshold <= SearchThresholdBytes {
		return searchPathMaxBytes
	}
	if threshold >= SearchBudget {
		return SearchBudget
	}
	return max(searchPathMaxBytes, threshold*3/4)
}

// cutMatches groups and caps match lines and stores the raw text first.
func cutMatches(text string, keep func([]byte) (string, bool), session string) (string, string, bool) {
	cut, applied := compressor.CompactSearchMatches([]byte(text), SearchBudget)
	if !applied {
		return "", "", false
	}
	lines := strings.Split(strings.TrimSuffix(string(cut), "\n"), "\n")
	// Keep any trailing "… N more matches ..." summary whole and hold the body
	// above it within what remains of SearchBudget.
	i := len(lines)
	for j, l := range lines {
		if strings.HasPrefix(l, "… ") {
			i = j
			break
		}
	}
	summary := lines[i:]
	summaryLen := len(strings.Join(summary, "\n"))
	body := append(fitLines(lines[:i], "matches", SearchBudget-summaryLen), summary...)
	bodyText := strings.Join(body, "\n") + "\n"
	worstMarker := execwrap.MarkerWithID(len(text), len(bodyText), readRetrieveArg(session, "000000000000"))
	if len(bodyText)+len(worstMarker) >= len(text) {
		return "", "", false
	}
	id, ok := keep([]byte(text))
	if !ok || id == "" {
		return "", "", false
	}
	arg := readRetrieveArg(session, id)
	out := bodyText + execwrap.MarkerWithID(len(text), len(bodyText), arg)
	if len(out) >= len(text) {
		return "", "", false
	}
	return out, id, true
}

// cutPaths groups a file list by directory and stores the whole list first.
func cutPaths(paths []string, budget int, keep func([]byte) (string, bool), session string) ([]string, string, bool) {
	body, applied := compressor.CompactFilePaths(paths, budget)
	if !applied || len(body) == 0 {
		return nil, "", false
	}
	full := strings.Join(paths, "\n") + "\n"
	bodyText := strings.Join(body, "\n")
	worstMarker := execwrap.MarkerWithID(len(full), len(bodyText), readRetrieveArg(session, "000000000000"))
	if len(bodyText)+1+len(worstMarker) >= len(full) {
		return nil, "", false
	}
	id, ok := keep([]byte(full))
	if !ok || id == "" {
		return nil, "", false
	}
	arg := readRetrieveArg(session, id)
	markerLine := strings.TrimSuffix(execwrap.MarkerWithID(len(full), len(bodyText), arg), "\n")
	out := append(body, markerLine)
	if len(strings.Join(out, "\n"))+1 >= len(full) {
		return nil, "", false
	}
	return out, id, true
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
