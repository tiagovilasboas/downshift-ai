// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package accounting

import (
	"sort"

	"github.com/tiagovilasboas/downshift-ai/internal/execwrap"
)

// OtherTop is how many labels the `other` breakdown lists before "rest".
const OtherTop = 10

// Labels for `other` rows that name no program.
const (
	OtherUnknown = "unknown (before head telemetry)"
	OtherRest    = "rest"
)

// OtherBreakdown splits one harness's unclassified (`other`) runs by what ran:
// the inner allowlisted program of a compound command, else the head code.
// Top is the OtherTop labels by raw bytes; Rest sums the others (nil if none).
type OtherBreakdown struct {
	Top  []OtherRow `json:"top"`
	Rest *OtherRow  `json:"rest,omitempty"`
}

// OtherRow is one label. NetSavedPct is net saved bytes over raw bytes, 0 when
// nothing was measured. LineBuckets counts runs per LineBucket code; rows
// written before head telemetry carry no bucket and are not counted there.
type OtherRow struct {
	Label       string           `json:"label"`
	Runs        int64            `json:"runs"`
	RawBytes    int64            `json:"raw_bytes"`
	NetSavedPct float64          `json:"net_saved_pct"`
	LineBuckets map[string]int64 `json:"line_buckets,omitempty"`
	savedBytes  int64
}

// otherLabel names the program behind an `other` row: the producer program
// (execwrap.ProducerProgram) when known, else the older inner family, else the
// head code; honestly "unknown" when the row predates head telemetry.
func otherLabel(ev execwrap.Event) string {
	switch {
	case ev.Prog != "" && ev.Prog != execwrap.HeadUnknown:
		return ev.Prog // the producing program; older rows have none
	case ev.Inner != "":
		return ev.Inner
	case ev.Head != "":
		return ev.Head
	}
	return OtherUnknown
}

// otherAcc gathers `other` rows per label.
type otherAcc map[string]*OtherRow

func (o otherAcc) add(ev execwrap.Event) {
	l := otherLabel(ev)
	r := o[l]
	if r == nil {
		r = &OtherRow{Label: l}
		o[l] = r
	}
	r.Runs++
	r.RawBytes += ev.RawBytes
	r.savedBytes += ev.NetSavedBytes()
	if ev.LineBucket != "" {
		if r.LineBuckets == nil {
			r.LineBuckets = map[string]int64{}
		}
		r.LineBuckets[ev.LineBucket]++
	}
}

// breakdown sorts by raw bytes, then runs, then label; nil when empty.
func (o otherAcc) breakdown() *OtherBreakdown {
	if len(o) == 0 {
		return nil
	}
	rows := make([]OtherRow, 0, len(o))
	for _, r := range o {
		rows = append(rows, *r)
	}
	sort.Slice(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if a.RawBytes != b.RawBytes {
			return a.RawBytes > b.RawBytes
		}
		if a.Runs != b.Runs {
			return a.Runs > b.Runs
		}
		return a.Label < b.Label
	})
	out := &OtherBreakdown{}
	if len(rows) > OtherTop {
		rest := OtherRow{Label: OtherRest, LineBuckets: map[string]int64{}}
		for _, r := range rows[OtherTop:] {
			rest.Runs += r.Runs
			rest.RawBytes += r.RawBytes
			rest.savedBytes += r.savedBytes
			for k, v := range r.LineBuckets {
				rest.LineBuckets[k] += v
			}
		}
		rest.NetSavedPct = savedPct(rest.savedBytes, rest.RawBytes)
		out.Rest = &rest
		rows = rows[:OtherTop]
	}
	for i := range rows {
		rows[i].NetSavedPct = savedPct(rows[i].savedBytes, rows[i].RawBytes)
	}
	out.Top = rows
	return out
}

func savedPct(saved, raw int64) float64 {
	if raw <= 0 {
		return 0
	}
	return 100 * float64(saved) / float64(raw)
}
