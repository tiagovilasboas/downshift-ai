// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/missedsavings"
)

func TestRenderDiscoverCoverageAndDedup(t *testing.T) {
	rep := missedsavings.Report{
		Since: "7d",
		Files: 2,
		Dedup: true,
		Total: missedsavings.Stat{
			Commands:          3,
			Wrapped:           1,
			Bare:              1,
			BareBytes:         600,
			Skipped:           1,
			DuplicatesSkipped: 4,
			Coverage: missedsavings.Coverage{
				BashCompactedBytes:    200,
				BashTotalBytes:        1000,
				BashCoveragePct:       20.0,
				NonBashCompactedBytes: 0,
				NonBashTotalBytes:     1000,
				NonBashCoveragePct:    0.0,
				TotalCompactedBytes:   200,
				TotalOutputBytes:      2000,
				TotalCoveragePct:      10.0,
			},
		},
	}
	var out bytes.Buffer
	renderDiscover(&out, rep)
	text := out.String()
	for _, want := range []string{
		"dedup by tool_use_id, 4 duplicates skipped",
		"Coverage (compacted output / total tool output):",
		"bash       200 B / 1000 B (20.0%)",
		"non-bash   0 B / 1000 B (0.0%)",
		"total      200 B / 2.0 KB (10.0%)",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("renderDiscover output missing %q:\n%s", want, text)
		}
	}
}

func TestRunDiscoverDedupFlag(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	var out, errOut bytes.Buffer
	code := runDiscover([]string{"--since", "1d", "--dedup", "--json"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("runDiscover code = %d, stderr = %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), `"dedup": true`) {
		t.Fatalf("expected dedup: true in JSON output:\n%s", out.String())
	}
}
