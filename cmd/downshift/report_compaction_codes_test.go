// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/accounting"
)

// The report shows the top passthrough details and metachar classes in both
// the markdown and the plain-text forms, and nothing when a harness has none.
func TestCompactionDetailShowsCodes(t *testing.T) {
	d := accounting.CompactionDetail{Harnesses: []accounting.HarnessCompaction{{
		Harness: "claude-code", Commands: 3,
		PassthroughDetails: []accounting.ReasonCount{{Reason: "cr", Count: 2}, {Reason: "anti_loop_bypass", Count: 1}},
		SkipMetachars:      []accounting.ReasonCount{{Reason: "pipe", Count: 5}},
	}, {Harness: "kirocrew"}}}
	var md, txt bytes.Buffer
	printCompactionDetailMarkdown(&md, d)
	printCompactionDetailText(&txt, d)
	for name, out := range map[string]string{"markdown": md.String(), "text": txt.String()} {
		for _, want := range []string{"carriage return", "repeated failure in session; raw output kept to break loop", "| pipe"} {
			if !strings.Contains(out, want) {
				t.Errorf("%s output lacks %q:\n%s", name, want, out)
			}
		}
	}
	if strings.Count(md.String(), "metacharacter") != 1 {
		t.Errorf("harness with no codes printed a table:\n%s", md.String())
	}
}

// TestCompactionDetailShowsPassthroughReasonLabels verifies that the G3
// passthrough reasons (too_large, stderr_output, hold_timeout) render with
// their human-readable labels in both markdown and plain-text reports.
func TestCompactionDetailShowsPassthroughReasonLabels(t *testing.T) {
	d := accounting.CompactionDetail{Harnesses: []accounting.HarnessCompaction{{
		Harness: "claude-code", Commands: 3,
		NotCompacted: []accounting.ReasonCount{
			{Reason: "too_large", Count: 1},
			{Reason: "stderr_output", Count: 2},
			{Reason: "hold_timeout", Count: 3},
		},
	}}}
	var md, txt bytes.Buffer
	printCompactionDetailMarkdown(&md, d)
	printCompactionDetailText(&txt, d)
	wants := []string{
		"over the hold limit, raw output not kept",
		"stderr with a terminal on stdin, streamed",
		"held past the hold bound, streamed",
	}
	for name, out := range map[string]string{"markdown": md.String(), "text": txt.String()} {
		for _, want := range wants {
			if !strings.Contains(out, want) {
				t.Errorf("%s output lacks %q:\n%s", name, want, out)
			}
		}
		if strings.Contains(out, "reason not recorded") {
			t.Errorf("%s output unexpectedly contains %q:\n%s", name, "reason not recorded", out)
		}
	}
}
