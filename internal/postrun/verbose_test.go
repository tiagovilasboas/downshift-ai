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

	t.Run("sentry and datadog MCP outputs under 4KB are pruned and stored in CCR", func(t *testing.T) {
		fixtures := []struct {
			toolName string
			raw      string
			preserve string
		}{
			{
				toolName: "mcp__sentry__search_issues",
				raw:      `{"result": "{\"issues\":[{\"id\":\"9812345\",\"shortId\":\"PAY-412\",\"title\":\"NullPointerException\",\"culprit\":\"ChargeService.capture\",\"level\":\"error\",\"status\":\"unresolved\",\"count\":\"148\",\"userCount\":42,\"firstSeen\":\"2026-11-20T09:15:00Z\",\"lastSeen\":\"2026-11-25T08:40:00Z\",\"permalink\":\"https://sentry.io/organizations/acme-corp/issues/9812345/?referrer=mcp\",\"assignedTo\":null,\"annotations\":[],\"activity\":[]}]}"}`,
				preserve: "PAY-412",
			},
			{
				toolName: "mcp__sentry__search_events",
				raw:      `{"events":[{"eventID":"ev_01abcdef","title":"TypeError","culprit":"confirm.ts","level":"error","status":"unresolved","timestamp":"2026-11-25T08:12:33Z","trace_id":"4bf92f3577b34da6","span_id":"00f067aa","permalink":"https://sentry.io/organizations/acme/issues/1/events/ev_01abcdef/?referrer=mcp","sdk":{"name":"sentry.node","version":"8.12.0"},"breadcrumbs":[],"cookies":{}}]}`,
				preserve: "4bf92f3577b34da6",
			},
			{
				toolName: "mcp__datadog__execute_code",
				raw:      `{"stdout": "{\n  \"status\": \"error\",\n  \"level\": \"error\",\n  \"timestamp\": \"2026-11-25T08:30:05Z\",\n  \"trace_id\": \"dd-trace-778899\",\n  \"span_id\": \"dd-span-3344\",\n  \"error\": \"Upstream 502\",\n  \"permalink\": \"https://app.datadoghq.com/logs?query=service%3Apayment-api&event=AgAAAZN123456\",\n  \"tags\": []\n}", "stderr": ""}`,
				preserve: "dd-trace-778899",
			},
		}
		for _, fx := range fixtures {
			if len(fx.raw) >= PostRunBalanceThreshold {
				t.Fatalf("%s fixture %d >= 4 KiB", fx.toolName, len(fx.raw))
			}
			var stored string
			keep := func(b []byte) (string, bool) {
				stored = string(b)
				return "ccrmcpprune1", true
			}
			res := Compact(caps, hookport.ToolOutput{
				Category: core.CategoryOther, VerboseKind: VerboseKindMCP, ToolName: fx.toolName,
				Content: fx.raw, HasContent: true, ExitCode: -1,
			}, Options{Keep: keep, Balanced: true})
			if !res.Replaced || !res.Compacted || res.KeptID != "ccrmcpprune1" || stored != fx.raw {
				t.Fatalf("%s: expected pruned+CCR, got replaced=%v compacted=%v id=%q", fx.toolName, res.Replaced, res.Compacted, res.KeptID)
			}
			if res.EmittedBytes >= res.RawBytes {
				t.Errorf("%s: did not shrink (%d >= %d)", fx.toolName, res.EmittedBytes, res.RawBytes)
			}
			if !strings.Contains(res.Output.Content, fx.preserve) {
				t.Errorf("%s: missing %q in:\n%s", fx.toolName, fx.preserve, res.Output.Content)
			}
		}
	})
}

func TestCompact_VerboseShortLogBalanceBytesScaledFallback(t *testing.T) {
	caps := core.HarnessCapabilities{ReplacesToolOutput: []core.ToolCategory{core.CategoryOther}}

	// 60 lines (~6.3 KiB > 4 KiB, <= 140 lines): Balance(BalanceLog) returns
	// (nil, false), so BalanceBytesScaled must handle it and store raw in CCR.
	var b strings.Builder
	for i := 0; i < 60; i++ {
		fmt.Fprintf(&b, "section %02d: %s\n", i, strings.Repeat("content ", 12))
	}
	raw := b.String()
	if len(raw) <= PostRunBalanceThreshold {
		t.Fatalf("fixture %d <= %d", len(raw), PostRunBalanceThreshold)
	}

	for _, kind := range []string{VerboseKindFetch, VerboseKindMCP} {
		t.Run(kind, func(t *testing.T) {
			var stored []byte
			keep := func(data []byte) (string, bool) {
				stored = append([]byte(nil), data...)
				return "ccrscaled001", true
			}
			res := Compact(caps, hookport.ToolOutput{
				Category:    core.CategoryOther,
				VerboseKind: kind,
				Content:     raw,
				HasContent:  true,
				ExitCode:    -1,
			}, Options{Keep: keep, Balanced: true})
			if !res.Replaced || !res.Compacted || res.KeptID != "ccrscaled001" {
				t.Fatalf("%s: expected BalanceBytesScaled compaction, got %+v", kind, res)
			}
			if string(stored) != raw {
				t.Errorf("%s: CCR raw mismatch: stored=%d raw=%d", kind, len(stored), len(raw))
			}
			if res.EmittedBytes >= res.RawBytes || !execwrap.HasMarker(res.Output.Content) {
				t.Errorf("%s: did not shrink or missing marker: raw=%d emitted=%d", kind, res.RawBytes, res.EmittedBytes)
			}
		})
	}
}
