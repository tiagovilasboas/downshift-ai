// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package accounting

import (
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/execwrap"
)

// New rows label by the producer program; old rows keep their label.
func TestOtherBreakdownProducerLabel(t *testing.T) {
	ev := func(inner, head, prog string, raw int64) execwrap.Event {
		return execwrap.Event{Harness: "claude-code", Family: "other", Inner: inner, Head: head,
			Prog: prog, RawBytes: raw, EmittedBytes: raw}
	}
	o := buildOther([]execwrap.Event{
		ev("wc", "git", "git", 5000),        // git log | wc -l: was "wc"
		ev("gh-auth", "cd", "python", 4000), // heredoc script: was "gh-auth"
		ev("git-other", "git", "", 3000),    // old row keeps its inner label
		ev("", "", "", 2000),                // pre head telemetry stays unknown
		ev("", "unknown", "unknown", 1000),  // unknown producer falls back to head
	})
	want := []string{"git", "python", "git-other", OtherUnknown, "unknown"}
	if o == nil || len(o.Top) != len(want) {
		t.Fatalf("breakdown = %+v", o)
	}
	for i, w := range want {
		if o.Top[i].Label != w {
			t.Errorf("row %d label = %q, want %q", i, o.Top[i].Label, w)
		}
	}
}
