// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package postrun

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/core"
	"github.com/tiagovilasboas/downshift-ai/internal/execwrap"
	"github.com/tiagovilasboas/downshift-ai/internal/hookport"
)

func verboseLogLines(n int, prefix string) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "%s line %04d: payload item with enough text to exceed threshold\n", prefix, i)
	}
	return b.String()
}

func TestCompact_VerboseNonBashTools(t *testing.T) {
	caps := core.HarnessCapabilities{
		ReplacesToolOutput: []core.ToolCategory{
			core.CategoryShell, core.CategorySearch, core.CategoryRead, core.CategoryOther,
		},
	}
	largeText := verboseLogLines(200, "entry") // ~13 KiB > 4 KiB
	smallText := verboseLogLines(20, "small")  // ~1.3 KiB <= 4 KiB
	mediumGrep := func() string {
		var b strings.Builder
		for i := 1; i <= 120; i++ {
			fmt.Fprintf(&b, "pkg/handler.go:%d:const value = %d // repeated match\n", i, i)
		}
		return b.String()
	}() // ~6.7 KiB (in 4 KiB..16 KiB band)
	mediumGlob := func() []string {
		out := make([]string, 120)
		for i := range out {
			out[i] = fmt.Sprintf("internal/module%02d/subpackage/component_handler_%03d.go", i%10, i)
		}
		return out
	}() // ~7.3 KiB (in 4 KiB..16 KiB band)

	cases := []struct {
		name        string
		in          hookport.ToolOutput
		opts        Options
		wantConsid  bool
		wantReplace bool
		wantFamily  string
	}{
		{
			name:        "mcp large text compacted",
			in:          hookport.ToolOutput{Category: core.CategoryOther, VerboseKind: VerboseKindMCP, Content: largeText, HasContent: true, ExitCode: -1},
			opts:        Options{Keep: keepOK, Balanced: true},
			wantConsid:  true,
			wantReplace: true,
			wantFamily:  string(FamilyMCPTool),
		},
		{
			name:        "mcp small text passes through",
			in:          hookport.ToolOutput{Category: core.CategoryOther, VerboseKind: VerboseKindMCP, Content: smallText, HasContent: true, ExitCode: -1},
			opts:        Options{Keep: keepOK, Balanced: true},
			wantConsid:  true,
			wantReplace: false,
			wantFamily:  string(FamilyMCPTool),
		},
		{
			name:        "mcp error stays whole",
			in:          hookport.ToolOutput{Category: core.CategoryOther, VerboseKind: VerboseKindMCP, Content: largeText, HasContent: true, ExitCode: 1},
			opts:        Options{Keep: keepOK, Balanced: true},
			wantConsid:  true,
			wantReplace: false,
			wantFamily:  string(FamilyMCPTool),
		},
		{
			name:        "webfetch large text compacted",
			in:          hookport.ToolOutput{Category: core.CategoryOther, VerboseKind: VerboseKindFetch, Content: largeText, HasContent: true, ExitCode: -1},
			opts:        Options{Keep: keepOK, Balanced: true},
			wantConsid:  true,
			wantReplace: true,
			wantFamily:  string(FamilyFetchTool),
		},
		{
			name:        "webfetch small text passes through",
			in:          hookport.ToolOutput{Category: core.CategoryOther, VerboseKind: VerboseKindFetch, Content: smallText, HasContent: true, ExitCode: -1},
			opts:        Options{Keep: keepOK, Balanced: true},
			wantConsid:  true,
			wantReplace: false,
			wantFamily:  string(FamilyFetchTool),
		},
		{
			name:        "other tool without VerboseKind ignored",
			in:          hookport.ToolOutput{Category: core.CategoryOther, Content: largeText, HasContent: true, ExitCode: -1},
			opts:        Options{Keep: keepOK, Balanced: true},
			wantConsid:  false,
			wantReplace: false,
		},
		{
			name:        "grep in 4K-16K band compacted",
			in:          hookport.ToolOutput{Category: core.CategorySearch, Content: mediumGrep, HasContent: true, ExitCode: -1},
			opts:        Options{Keep: keepOK, Balanced: true},
			wantConsid:  true,
			wantReplace: true,
			wantFamily:  string(FamilySearchTool),
		},
		{
			name:        "glob in 4K-16K band compacted",
			in:          hookport.ToolOutput{Category: core.CategorySearch, Paths: mediumGlob, HasPaths: true, ExitCode: -1},
			opts:        Options{Keep: keepOK, Balanced: true},
			wantConsid:  true,
			wantReplace: true,
			wantFamily:  string(FamilySearchTool),
		},
		{
			name:        "read under 64K stays untouched",
			in:          hookport.ToolOutput{Category: core.CategoryRead, Content: largeText, HasContent: true, ExitCode: -1},
			opts:        Options{Keep: keepOK, Balanced: true},
			wantConsid:  true,
			wantReplace: false,
			wantFamily:  string(FamilyReadTool),
		},
		{
			name:        "mcp without CCR stays whole",
			in:          hookport.ToolOutput{Category: core.CategoryOther, VerboseKind: VerboseKindMCP, Content: largeText, HasContent: true, ExitCode: -1},
			opts:        Options{Balanced: true},
			wantConsid:  true,
			wantReplace: false,
			wantFamily:  string(FamilyMCPTool),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := Compact(caps, tc.in, tc.opts)
			if res.Considered != tc.wantConsid {
				t.Fatalf("Considered = %v, want %v", res.Considered, tc.wantConsid)
			}
			if res.Replaced != tc.wantReplace {
				t.Fatalf("Replaced = %v, want %v (passthrough=%q raw=%d emitted=%d)",
					res.Replaced, tc.wantReplace, res.Passthrough, res.RawBytes, res.EmittedBytes)
			}
			if string(res.Family) != tc.wantFamily {
				t.Errorf("Family = %q, want %q", res.Family, tc.wantFamily)
			}
			if tc.wantReplace {
				if res.EmittedBytes >= res.RawBytes {
					t.Errorf("EmittedBytes (%d) >= RawBytes (%d)", res.EmittedBytes, res.RawBytes)
				}
				if res.KeptID == "" {
					t.Errorf("expected non-empty KeptID")
				}
			}
		})
	}
}

func TestCompact_MCPJSONPruningAndFallback(t *testing.T) {
	caps := core.HarnessCapabilities{
		ReplacesToolOutput: []core.ToolCategory{core.CategoryOther},
	}

	makeIssue := func(i int, bodyPad string) map[string]any {
		return map[string]any{
			"id":                1000 + i,
			"node_id":           fmt.Sprintf("MDQ6VXNlcj%06d", i),
			"html_url":          fmt.Sprintf("https://github.com/org/repo/issues/%d", i),
			"comments_url":      fmt.Sprintf("https://api.github.com/repos/org/repo/issues/%d/comments", i),
			"events_url":        fmt.Sprintf("https://api.github.com/repos/org/repo/issues/%d/events", i),
			"labels_url":        fmt.Sprintf("https://api.github.com/repos/org/repo/issues/%d/labels{/name}", i),
			"repository_url":    "https://api.github.com/repos/org/repo",
			"followers_url":     "https://api.github.com/users/dev/followers",
			"following_url":     "https://api.github.com/users/dev/following{/other_user}",
			"subscriptions_url": "https://api.github.com/users/dev/subscriptions",
			"title":             fmt.Sprintf("Issue %d", i),
			"body":              bodyPad,
		}
	}

	t.Run("under 4KB prunable JSON is pruned and raw stored in CCR", func(t *testing.T) {
		rawBytes, err := json.Marshal(makeIssue(1, "fix login bug"))
		if err != nil {
			t.Fatal(err)
		}
		raw := string(rawBytes) // ~650 bytes (< 4 KiB, no trailing newline)
		if len(raw) >= PostRunBalanceThreshold {
			t.Fatalf("fixture %d >= 4 KiB", len(raw))
		}
		var stored []byte
		keep := func(b []byte) (string, bool) {
			stored = append([]byte(nil), b...)
			return "ccrmcpjson01", true
		}
		res := Compact(caps, hookport.ToolOutput{
			Category: core.CategoryOther, VerboseKind: VerboseKindMCP,
			Content: raw, HasContent: true, ExitCode: -1,
		}, Options{Keep: keep, Balanced: true, Session: strings.Repeat("ab", 32)})
		if !res.Replaced || !res.Compacted || res.Family != FamilyMCPTool {
			t.Fatalf("expected MCP JSON pruned: %+v", res)
		}
		if res.KeptID != "ccrmcpjson01" || string(stored) != raw {
			t.Errorf("CCR stored mismatch: id=%q stored=%d raw=%d", res.KeptID, len(stored), len(raw))
		}
		if res.EmittedBytes >= res.RawBytes {
			t.Errorf("did not shrink: %d >= %d", res.EmittedBytes, res.RawBytes)
		}
		if strings.Contains(res.Output.Content, "node_id") || strings.Contains(res.Output.Content, "followers_url") {
			t.Errorf("noisy fields leaked:\n%s", res.Output.Content)
		}
		if !strings.Contains(res.Output.Content, "fix login bug") || !strings.Contains(res.Output.Content, "downshift retrieve --session=abababababab ccrmcpjson01") {
			t.Errorf("core content or retrieve hint missing:\n%s", res.Output.Content)
		}
	})

	t.Run("over 4KB prunable JSON still >4KB after pruning falls back to Balanced", func(t *testing.T) {
		items := make([]map[string]any, 35)
		for i := range items {
			m := makeIssue(i, fmt.Sprintf("detail summary for issue %d", i))
			for f := 0; f < 10; f++ {
				m[fmt.Sprintf("field_%02d", f)] = fmt.Sprintf("value %02d for item %d", f, i)
			}
			items[i] = m
		}
		rawBytes, err := json.MarshalIndent(items, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		raw := string(rawBytes)
		var stored []byte
		keep := func(b []byte) (string, bool) {
			stored = append([]byte(nil), b...)
			return "ccrmcpbig001", true
		}
		res := Compact(caps, hookport.ToolOutput{
			Category: core.CategoryOther, VerboseKind: VerboseKindMCP,
			Content: raw, HasContent: true, ExitCode: -1,
		}, Options{Keep: keep, Balanced: true})
		if !res.Replaced || res.KeptID != "ccrmcpbig001" || string(stored) != raw {
			t.Fatalf("expected pruned+balanced MCP JSON with full raw in CCR: replaced=%v id=%q", res.Replaced, res.KeptID)
		}
		if strings.Contains(res.Output.Content, "node_id") || strings.Contains(res.Output.Content, "followers_url") {
			t.Errorf("JSON pruning did not run before Balanced:\n%s", res.Output.Content)
		}
		if !execwrap.HasMarker(res.Output.Content) || !strings.Contains(res.Output.Content, "lines cut (downshift retrieve)") {
			t.Errorf("expected Balanced cut after JSON pruning, got:\n%s", res.Output.Content)
		}
		if res.EmittedBytes >= res.RawBytes {
			t.Errorf("did not shrink: %d >= %d", res.EmittedBytes, res.RawBytes)
		}
	})

	t.Run("tiny JSON and non-shrinking JSON stay raw without touching CCR", func(t *testing.T) {
		called := false
		keep := func([]byte) (string, bool) {
			called = true
			return "shouldnot", true
		}
		for _, raw := range []string{
			`{"id":1,"node_id":"abc"}`,
			`{"id":1,"title":"` + strings.Repeat("x", 900) + `"}`,
		} {
			res := Compact(caps, hookport.ToolOutput{
				Category: core.CategoryOther, VerboseKind: VerboseKindMCP,
				Content: raw, HasContent: true, ExitCode: -1,
			}, Options{Keep: keep, Balanced: true})
			if res.Replaced || res.Output.Content != raw {
				t.Errorf("expected passthrough for len=%d, got replaced=%v", len(raw), res.Replaced)
			}
		}
		if called {
			t.Error("CCR Keep must not be called when JSON stays raw")
		}
	})
}
