// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

func TestRunDiscoverPostRunMCPCoverage(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	stamp := time.Now().UTC().Add(-time.Hour).Format(time.RFC3339)
	mcpBody := strings.Repeat("x", 900) + "\n[downshift compact: 9000 to 900 bytes; downshift retrieve abcdef012345]\n"
	readBody := strings.Repeat("y", len(mcpBody))
	lines := []string{
		fmt.Sprintf(`{"type":"assistant","timestamp":%q,"message":{"content":[{"type":"tool_use","id":"m1","name":"mcp__grafana__query_loki","input":{"query":"up"}}]}}`, stamp),
		fmt.Sprintf(`{"type":"user","timestamp":%q,"message":{"content":[{"type":"tool_result","tool_use_id":"m1","content":%q}]}}`, stamp, mcpBody),
		fmt.Sprintf(`{"type":"assistant","timestamp":%q,"message":{"content":[{"type":"tool_use","id":"r1","name":"Read","input":{"file_path":"a.go"}}]}}`, stamp),
		fmt.Sprintf(`{"type":"user","timestamp":%q,"message":{"content":[{"type":"tool_result","tool_use_id":"r1","content":%q}]}}`, stamp, readBody),
	}
	p := filepath.Join(home, ".claude", "projects", "p1", "s.jsonl")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	code := runDiscover([]string{"--since", "1d", "--dedup"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("runDiscover code = %d, stderr = %s", code, errOut.String())
	}
	text := out.String()
	if strings.Contains(text, "non-bash   0 B /") || strings.Contains(text, "(0.0%)\n  total") {
		t.Fatalf("expected non-zero non-bash coverage, got:\n%s", text)
	}
	if !strings.Contains(text, "non-bash   973 B / 1.9 KB (50.0%)") {
		t.Fatalf("unexpected non-bash coverage line:\n%s", text)
	}
}
