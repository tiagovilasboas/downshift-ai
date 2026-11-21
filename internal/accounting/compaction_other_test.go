// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package accounting

import (
	"fmt"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/execwrap"
)

func buildOther(events []execwrap.Event) *OtherBreakdown {
	d := BuildCompactionDetail(events, nil, Scope{Kind: ScopeDays}, time.Now())
	return d.Harnesses[0].Other
}

func TestOtherBreakdown(t *testing.T) {
	ev := func(inner, head, bucket string, raw, emitted, retrieved int64) execwrap.Event {
		return execwrap.Event{Harness: "claude-code", Family: "other", Inner: inner, Head: head,
			LineBucket: bucket, RawBytes: raw, EmittedBytes: emitted, RetrievedBytes: retrieved}
	}
	o := buildOther([]execwrap.Event{
		ev("", "", "", 100, 100, 0),            // before head telemetry
		ev("", "curl", "1-20", 1000, 400, 100), // net saved 500
		ev("pytest", "bash", "21-140", 3000, 3000, 0),
		ev("", "curl", ">1000", 1000, 1000, 0),
		{Harness: "claude-code", Family: "go", RawBytes: 99999}, // not other
	})
	if o == nil || o.Rest != nil {
		t.Fatalf("breakdown = %+v", o)
	}
	want := []struct {
		label string
		runs  int64
		raw   int64
		pct   float64
	}{{"pytest", 1, 3000, 0}, {"curl", 2, 2000, 25}, {OtherUnknown, 1, 100, 0}}
	if len(o.Top) != len(want) {
		t.Fatalf("rows = %+v", o.Top)
	}
	for i, w := range want {
		g := o.Top[i]
		if g.Label != w.label || g.Runs != w.runs || g.RawBytes != w.raw || g.NetSavedPct != w.pct {
			t.Errorf("row %d = %+v, want %+v", i, g, w)
		}
	}
	if b := o.Top[1].LineBuckets; b["1-20"] != 1 || b[">1000"] != 1 {
		t.Errorf("curl buckets = %v", b)
	}
	if o.Top[2].LineBuckets != nil {
		t.Errorf("unknown row has buckets: %v", o.Top[2].LineBuckets)
	}
}

func TestOtherBreakdownTopAndRest(t *testing.T) {
	var events []execwrap.Event
	for i := 0; i < OtherTop+2; i++ {
		events = append(events, execwrap.Event{Harness: "h", Family: "other", Inner: fmt.Sprintf("p%02d", i),
			RawBytes: int64(1000 - i), EmittedBytes: 500, LineBucket: "1-20"})
	}
	o := buildOther(events)
	if len(o.Top) != OtherTop || o.Top[0].Label != "p00" {
		t.Fatalf("top = %+v", o.Top)
	}
	if o.Rest == nil || o.Rest.Label != OtherRest || o.Rest.Runs != 2 || o.Rest.RawBytes != 990+989 || o.Rest.LineBuckets["1-20"] != 2 {
		t.Errorf("rest = %+v", o.Rest)
	}
}

func TestOtherBreakdownAbsentWithoutOtherRows(t *testing.T) {
	if buildOther([]execwrap.Event{{Harness: "h", Family: "go", RawBytes: 10}}) != nil {
		t.Error("other present without other rows")
	}
}
