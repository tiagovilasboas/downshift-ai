// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package accounting

import (
	"fmt"
	"github.com/tiagovilasboas/downshift-ai/internal/telemetry/readout"

	"github.com/tiagovilasboas/downshift-ai/internal/execwrap"
)

// Ledger is the token ledger every surface shows over one Scope, tokens only,
// never money. Routing tokens are per harness (Numbers.Routing) and never
// summed across harnesses, so the ledger holds only what is not per harness
// routing: the Compactor estimate and the delegate workers.
type Ledger struct {
	// CompactorSaved is the output `downshift compactor` kept out of the
	// context: saved bytes / execwrap.BytesPerToken. An estimate.
	CompactorSaved int64 `json:"compactor_saved"`
	// CompactorEstimate names how CompactorSaved was derived.
	CompactorEstimate string `json:"compactor_estimate"`
	// Workers is the tokens the MCP delegate workers spent, per CLI. They are
	// additional to the routed spawns, never part of them (see WorkerTokens). Absent when no
	// worker ended in the scope; worker events carry no session, so a session
	// scope has none.
	Workers []WorkerTokens `json:"workers,omitempty"`
}

// compactorEstimateNote is the method behind Ledger.CompactorSaved.
var compactorEstimateNote = fmt.Sprintf("estimate: saved bytes / %d", execwrap.BytesPerToken)

// ledgerOf builds the Ledger from the Numbers Compute already holds, so the
// ledger and every other figure share one window.
func ledgerOf(n Numbers) Ledger {
	return Ledger{
		CompactorSaved:    n.Compactor.Tokens,
		CompactorEstimate: compactorEstimateNote,
		Workers:           n.workers,
	}
}

// LedgerLine is the printed ledger for scope, e.g.
// "Token ledger (last 7 days): 45k cut by the compactor (estimate: saved bytes / 4)".
// Routing tokens are per harness and printed apart (readout.PrintRouting).
func LedgerLine(l Ledger, scope Scope) string {
	return fmt.Sprintf("Token ledger (%s): %s cut by the compactor (%s)",
		scope.Label(),
		readout.ShortTokens(l.CompactorSaved),
		l.CompactorEstimate)
}

// CompactorScopeLine formats the Compactor's net cut percentage, byte totals,
// run counts, and window/harness scope for `downshift stats`, matching the
// byte-basis net cut shown in `downshift report`. Returns "" when no commands
// ran in scope.
func CompactorScopeLine(c Compactor, scope Scope) string {
	if c.Commands == 0 {
		return ""
	}
	cut := "-"
	if c.RawBytes > 0 {
		pct := 100 * float64(c.SavedBytes) / float64(c.RawBytes)
		cut = fmt.Sprintf("%.0f%% (%s of %s)", pct, formatBytes(c.SavedBytes), formatBytes(c.RawBytes))
	}
	line := fmt.Sprintf("Compactor (%s, all harnesses): %s net cut, %d · %d runs · compacted",
		scope.Label(), cut, c.Commands, c.Compacted)
	if c.RetrievedBytes > 0 {
		line += fmt.Sprintf(" · %s read back", formatBytes(c.RetrievedBytes))
	}
	return line + " (use `downshift report` for per-harness)"
}

func formatBytes(n int64) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(n)/(1<<10))
	}
	return fmt.Sprintf("%d B", n)
}
