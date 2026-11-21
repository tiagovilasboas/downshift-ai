// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"io"

	"github.com/tiagovilasboas/downshift-ai/internal/telemetry"
)

const outcomeOriginNote = "Observational: origins are not randomized, so this does not show that a cheaper model causes fewer or more failures."

func checkedShareText(o telemetry.OriginOutcome) string {
	if s, ok := o.CheckedShare(); ok {
		return fmt.Sprintf("%.0f%%", s*100)
	}
	return "-"
}

func originLabel(o telemetry.OriginOutcome) string {
	if o.SmallSample() {
		return fmt.Sprintf("%s (small sample, n<%d)", o.Origin, telemetry.SmallSampleN)
	}
	return string(o.Origin)
}

// printOutcomeOriginMarkdown writes accepted vs rework per spawn origin.
func printOutcomeOriginMarkdown(w io.Writer, matrix []telemetry.OutcomeMatrixRow) {
	rows := telemetry.OutcomesByOrigin(matrix)
	if len(rows) == 0 {
		return
	}
	fmt.Fprintf(w, "## Outcomes by origin (observational)\n\n%s\n\n", outcomeOriginNote)
	fmt.Fprintln(w, "| origin | n | accepted | checked | check failed | reworked + errored | unknown |")
	fmt.Fprintln(w, "|---|--:|--:|--:|--:|--:|--:|")
	for _, o := range rows {
		fmt.Fprintf(w, "| %s | %d | %d | %s | %d | %d | %d |\n", originLabel(o), o.N, o.Accepted(),
			checkedShareText(o), o.Outcomes.CheckFailed, o.ReworkedOrErrored(), o.Outcomes.Unknown)
	}
	fmt.Fprintln(w)
}

// printOutcomeOriginText is the same block for the plain-text report.
func printOutcomeOriginText(w io.Writer, matrix []telemetry.OutcomeMatrixRow) {
	rows := telemetry.OutcomesByOrigin(matrix)
	if len(rows) == 0 {
		return
	}
	fmt.Fprintf(w, "\nOUTCOMES BY ORIGIN (observational)\n   %s\n", outcomeOriginNote)
	for _, o := range rows {
		fmt.Fprintf(w, "   %s: n=%d accepted=%d checked=%s check_failed=%d reworked+errored=%d unknown=%d\n",
			originLabel(o), o.N, o.Accepted(), checkedShareText(o), o.Outcomes.CheckFailed, o.ReworkedOrErrored(), o.Outcomes.Unknown)
	}
}
