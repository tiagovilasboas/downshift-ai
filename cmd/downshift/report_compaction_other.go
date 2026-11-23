// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"github.com/tiagovilasboas/downshift-ai/internal/telemetry/readout"
	"io"
	"strings"

	"github.com/tiagovilasboas/downshift-ai/internal/accounting"
	"github.com/tiagovilasboas/downshift-ai/internal/execwrap"
)

// lineBucketOrder lists the execwrap.LineBucket codes, smallest first.
var lineBucketOrder = []string{"0", "1-20", "21-140", "141-1000", ">1000"}

// lineBucketsText renders the per-run line buckets, "-" when none.
func lineBucketsText(m map[string]int64) string {
	var parts []string
	for _, b := range lineBucketOrder {
		if n := m[b]; n > 0 {
			parts = append(parts, fmt.Sprintf("%s:%d", b, n))
		}
	}
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, " ")
}

func otherRows(o *accounting.OtherBreakdown) []accounting.OtherRow {
	rows := append([]accounting.OtherRow{}, o.Top...)
	if o.Rest != nil {
		rows = append(rows, *o.Rest)
	}
	return rows
}

// printOtherMarkdown shows what the other family ran (inner program, else
// head code), largest output first.
func printOtherMarkdown(w io.Writer, o *accounting.OtherBreakdown) {
	if o == nil {
		return
	}
	fmt.Fprint(w, "**Inside other** (inner program, else head; largest output first; lines per run)\n\n")
	fmt.Fprintln(w, "| program | runs | output tokens | net saved | lines per run |")
	fmt.Fprintln(w, "|---|--:|--:|--:|---|")
	for _, r := range otherRows(o) {
		fmt.Fprintf(w, "| %s | %d | %s | %.0f%% | %s |\n", r.Label, r.Runs,
			readout.ShortTokens(r.RawBytes/execwrap.BytesPerToken), r.NetSavedPct, lineBucketsText(r.LineBuckets))
	}
	fmt.Fprintln(w)
}

// printOtherText is the same table for the plain-text report.
func printOtherText(w io.Writer, o *accounting.OtherBreakdown) {
	if o == nil {
		return
	}
	fmt.Fprintf(w, "      inside other: %-32s %5s %13s %9s  %s\n", "program", "runs", "output tokens", "net saved", "lines per run")
	for _, r := range otherRows(o) {
		fmt.Fprintf(w, "                    %-32s %5d %13s %8.0f%%  %s\n", r.Label, r.Runs,
			readout.ShortTokens(r.RawBytes/execwrap.BytesPerToken), r.NetSavedPct, lineBucketsText(r.LineBuckets))
	}
}
