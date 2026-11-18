// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package accounting

import (
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/execwrap"
)

func TestDirectCompactorUsesGivenNow(t *testing.T) {
	ev := execwrap.Event{TS: "2026-11-01T12:00:00Z", Compacted: true, RawBytes: 4000, EmittedBytes: 400}
	scope := Scope{Kind: ScopeDays, Days: 7}
	cases := []struct {
		name string
		now  string
		want int64
	}{
		{"inside window", "2026-11-05T00:00:00Z", 1},
		{"after window", "2026-11-20T00:00:00Z", 0},
	}
	for _, c := range cases {
		now, _ := time.Parse(time.RFC3339, c.now)
		if got := DirectCompactor([]execwrap.Event{ev}, scope, now).Commands; got != c.want {
			t.Errorf("%s: commands = %d, want %d", c.name, got, c.want)
		}
	}
}

func TestDirectCompactorClassifiesEntry(t *testing.T) {
	now, _ := time.Parse(time.RFC3339, "2026-11-05T00:00:00Z")
	scope := Scope{Kind: ScopeDays, Days: 7}
	ts := "2026-11-04T12:00:00Z"

	cases := []struct {
		name string
		ev   execwrap.Event
		want int64
	}{
		{
			name: "entry direct without session",
			ev:   execwrap.Event{TS: ts, Entry: "direct", Compacted: true, RawBytes: 4000, EmittedBytes: 400},
			want: 1,
		},
		{
			name: "entry direct with session",
			ev:   execwrap.Event{TS: ts, Entry: "direct", Session: "s1", Compacted: true, RawBytes: 4000, EmittedBytes: 400},
			want: 1,
		},
		{
			name: "entry post_tool_use without session",
			ev:   execwrap.Event{TS: ts, Entry: "post_tool_use", Session: "", Compacted: true, RawBytes: 4000, EmittedBytes: 400},
			want: 0,
		},
		{
			name: "entry hook without session",
			ev:   execwrap.Event{TS: ts, Entry: "hook", Session: "", Compacted: true, RawBytes: 4000, EmittedBytes: 400},
			want: 0,
		},
		{
			name: "legacy without entry or session",
			ev:   execwrap.Event{TS: ts, Entry: "", Session: "", Compacted: true, RawBytes: 4000, EmittedBytes: 400},
			want: 1,
		},
		{
			name: "legacy without entry with session",
			ev:   execwrap.Event{TS: ts, Entry: "", Session: "s1", Compacted: true, RawBytes: 4000, EmittedBytes: 400},
			want: 0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := DirectCompactor([]execwrap.Event{tc.ev}, scope, now).Commands
			if got != tc.want {
				t.Errorf("%s: commands = %d, want %d", tc.name, got, tc.want)
			}
		})
	}
}
