// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package accounting

import (
	"fmt"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/execwrap"
)

// applyWindowedRetrievals re-computes RetrievedBytes on scopedExec using only
// retrievals inside scope's time window. When retrievals is nil and scope is
// ScopeDays with Days > 0, it reads exec-retrievals.jsonl from the state dir;
// if no retrievals exist on disk, scopedExec is returned unchanged (preserving
// in-memory RetrievedBytes set by tests).
func applyWindowedRetrievals(scopedExec []execwrap.Event, retrievals []execwrap.Retrieval, scope Scope, now time.Time) []execwrap.Event {
	if retrievals == nil && scope.Kind == ScopeDays && scope.Days > 0 {
		if p, err := execwrap.EventsPath(); err == nil {
			retrievals = execwrap.ReadRetrievalEntries(p)
		}
	}
	if len(retrievals) == 0 {
		return scopedExec
	}
	var filtered []execwrap.Retrieval
	if scope.Kind == ScopeDays && scope.Days > 0 {
		cutoff := now.Add(-time.Duration(scope.Days) * 24 * time.Hour)
		for _, r := range retrievals {
			if t, err := time.Parse(time.RFC3339, r.TS); err == nil && t.After(cutoff) {
				filtered = append(filtered, r)
			}
		}
	} else {
		filtered = retrievals
	}
	out := make([]execwrap.Event, len(scopedExec))
	copy(out, scopedExec)
	for i := range out {
		out[i].RetrievedBytes = 0
	}
	return execwrap.ApplyRetrievalEntries(out, filtered)
}

// compactorTotals sums the scoped exec events with execwrap's own aggregation,
// so the savings arithmetic exists once.
func compactorTotals(evs []execwrap.Event) Compactor {
	sum := execwrap.SummarizeEvents(evs)
	return Compactor{
		Commands:       sum.Commands,
		Compacted:      sum.Compacted,
		RawBytes:       sum.RawBytes,
		SavedBytes:     sum.SavedBytes,
		Tokens:         sum.EstimatedTokensSaved,
		RetrievedBytes: sum.RetrievedBytes,
		RetrievedPct:   sum.RetrievedPct,
	}
}

// compactorText is the Commands line. When the agent read output back with
// `downshift retrieve`, it adds the bytes read back, so a saving that was
// taken back is visible beside the net figure.
func compactorText(c Compactor) string {
	text := fmt.Sprintf("Commands: %d · Compacted: %d", c.Commands, c.Compacted)
	if c.RetrievedBytes > 0 {
		if c.RetrievedPct > 0 {
			text += fmt.Sprintf(" · Read back with retrieve: %d bytes (%.1f%% of cut)", c.RetrievedBytes, c.RetrievedPct)
		} else {
			text += fmt.Sprintf(" · Read back with retrieve: %d bytes", c.RetrievedBytes)
		}
	}
	return text
}
