// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"strings"
	"testing"
)

// TestCompactCapturedLosslessUnclassified: an unclassified output with ANSI
// and repeats shrinks without a store, keeps a danger line, and the caller's
// exit code is not an input the pass changes.
func TestCompactCapturedLosslessUnclassified(t *testing.T) {
	raw := "\x1b[31mwarning: slow\x1b[0m\n" + strings.Repeat("tick\n", 6) + "done\n"
	for _, exit := range []int{0, 1} {
		res := CompactCapturedWithThreshold(nil, FamilyNone, []byte(raw), exit, nil, true, 0, "")
		got := string(res.Output)
		if !res.Compacted || len(got) >= len(raw) {
			t.Fatalf("exit %d: not compacted: %q", exit, got)
		}
		for _, want := range []string{"warning: slow", "(repeated 6 times)", "done"} {
			if !strings.Contains(got, want) {
				t.Errorf("exit %d: %q lost from %q", exit, want, got)
			}
		}
	}
}

// TestCompactCapturedLosslessNoGainStaysRaw: output the pass cannot shrink is
// returned byte for byte.
func TestCompactCapturedLosslessNoGainStaysRaw(t *testing.T) {
	raw := "plain\noutput\n"
	res := CompactCapturedWithThreshold(nil, FamilyNone, []byte(raw), 0, nil, true, 0, "")
	if res.Compacted || string(res.Output) != raw {
		t.Errorf("changed: %+v", res)
	}
}
