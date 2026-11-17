// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package postrun

import (
	"fmt"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/core"
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
