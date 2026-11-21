// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"io"
	"sort"
	"text/tabwriter"

	"github.com/tiagovilasboas/downshift-ai/internal/ccr"
	"github.com/tiagovilasboas/downshift-ai/internal/compressor"
)

type sampleRow struct {
	label           string
	n               int
	bytes, lossless int64
}

// runCCR is `downshift ccr samples`: a table, per label, of the local raw
// samples and the share the lossless pass would cut. Exit 0, 1 on a state
// directory error, 2 on usage.
func runCCR(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 || args[0] != "samples" {
		fmt.Fprintln(stderr, "usage: downshift ccr samples")
		return 2
	}
	s, err := ccr.DefaultSampler()
	if err != nil {
		fmt.Fprintln(stderr, "ccr samples:", err)
		return 1
	}
	return printSamples(s, stdout)
}

func printSamples(s ccr.Sampler, w io.Writer) int {
	rows := map[string]*sampleRow{}
	for _, m := range s.Samples() {
		r := rows[m.Label]
		if r == nil {
			r = &sampleRow{label: m.Label}
			rows[m.Label] = r
		}
		r.n++
		r.bytes += int64(m.Bytes)
		raw, err := s.Raw(m.ID)
		if err != nil {
			r.lossless += int64(m.Bytes)
			continue
		}
		out, _ := compressor.LosslessText(raw)
		r.lossless += int64(len(out))
	}
	if len(rows) == 0 {
		fmt.Fprintln(w, "no samples yet (1 in 10 uncompressed outputs of 4 KiB or more is kept for 24 h; DOWNSHIFT_CCR_SAMPLES=0 turns it off)")
		return 0
	}
	list := make([]*sampleRow, 0, len(rows))
	for _, r := range rows {
		list = append(list, r)
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].bytes != list[j].bytes {
			return list[i].bytes > list[j].bytes
		}
		return list[i].label < list[j].label
	})
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "LABEL\tSAMPLES\tBYTES\tLOSSLESS CUT")
	for _, r := range list {
		share := 0.0
		if r.bytes > 0 {
			share = 100 * float64(r.bytes-r.lossless) / float64(r.bytes)
		}
		label := r.label
		if label == "" {
			label = "(none)"
		}
		fmt.Fprintf(tw, "%s\t%d\t%d\t%.1f%%\n", label, r.n, r.bytes, share)
	}
	_ = tw.Flush()
	return 0
}
