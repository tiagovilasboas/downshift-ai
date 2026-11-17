// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package claudecode_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/adapters/claudecode"
	"github.com/tiagovilasboas/downshift-ai/internal/core"
)

func TestParseAndReplaceToolOutput_NonBashTools(t *testing.T) {
	largeBody := strings.Repeat("line of verbose output\n", 300)

	cases := []struct {
		name            string
		toolName        string
		toolResp        any
		wantCategory    core.ToolCategory
		wantVerboseKind string
		wantHasContent  bool
		wantHasPaths    bool
		wantOpaque      bool
		wantExitCode    int
		wantMCPField    bool
	}{
		{
			name:            "mcp string response",
			toolName:        "mcp__github__get_file",
			toolResp:        largeBody,
			wantCategory:    core.CategoryOther,
			wantVerboseKind: "mcp",
			wantHasContent:  true,
			wantExitCode:    -1,
			wantMCPField:    true,
		},
		{
			name:     "mcp top-level content array",
			toolName: "mcp__jira__search",
			toolResp: []any{
				map[string]any{"type": "text", "text": largeBody},
			},
			wantCategory:    core.CategoryOther,
			wantVerboseKind: "mcp",
			wantHasContent:  true,
			wantExitCode:    -1,
			wantMCPField:    true,
		},
		{
			name:     "mcp map with content array",
			toolName: "mcp__postgres__query",
			toolResp: map[string]any{
				"content": []any{map[string]any{"type": "text", "text": largeBody}},
				"isError": false,
			},
			wantCategory:    core.CategoryOther,
			wantVerboseKind: "mcp",
			wantHasContent:  true,
			wantExitCode:    -1,
			wantMCPField:    true,
		},
		{
			name:            "mcp map with text field",
			toolName:        "mcp__docs__fetch",
			toolResp:        map[string]any{"text": largeBody, "source": "kb"},
			wantCategory:    core.CategoryOther,
			wantVerboseKind: "mcp",
			wantHasContent:  true,
			wantExitCode:    -1,
			wantMCPField:    true,
		},
		{
			name:            "mcp arbitrary JSON object",
			toolName:        "mcp__api__list",
			toolResp:        map[string]any{"items": []any{"a", "b", "c"}, "total": 3},
			wantCategory:    core.CategoryOther,
			wantVerboseKind: "mcp",
			wantHasContent:  true,
			wantExitCode:    -1,
			wantMCPField:    true,
		},
		{
			name:     "mcp image block is opaque",
			toolName: "mcp__browser__screenshot",
			toolResp: []any{
				map[string]any{"type": "image", "data": "base64"},
			},
			wantCategory:    core.CategoryOther,
			wantVerboseKind: "mcp",
			wantOpaque:      true,
			wantExitCode:    -1,
		},
		{
			name:     "mcp error sets exit code 1",
			toolName: "mcp__jira__search",
			toolResp: map[string]any{
				"content": []any{map[string]any{"type": "text", "text": largeBody}},
				"isError": true,
			},
			wantCategory:    core.CategoryOther,
			wantVerboseKind: "mcp",
			wantHasContent:  true,
			wantExitCode:    1,
			wantMCPField:    true,
		},
		{
			name:     "webfetch result map",
			toolName: "WebFetch",
			toolResp: map[string]any{
				"url":      "https://example.com",
				"code":     200,
				"codeText": "OK",
				"result":   largeBody,
			},
			wantCategory:    core.CategoryOther,
			wantVerboseKind: "fetch",
			wantHasContent:  true,
			wantExitCode:    -1,
			wantMCPField:    false,
		},
		{
			name:           "grep content map",
			toolName:       "Grep",
			toolResp:       map[string]any{"mode": "content", "content": largeBody, "numLines": 300},
			wantCategory:   core.CategorySearch,
			wantHasContent: true,
			wantExitCode:   -1,
			wantMCPField:   false,
		},
		{
			name:         "glob filenames map",
			toolName:     "Glob",
			toolResp:     map[string]any{"filenames": []any{"a.go", "b.go"}, "numFiles": 2},
			wantCategory: core.CategorySearch,
			wantHasPaths: true,
			wantExitCode: -1,
			wantMCPField: false,
		},
		{
			name:           "read nested file untouched",
			toolName:       "Read",
			toolResp:       map[string]any{"type": "text", "file": map[string]any{"content": largeBody}},
			wantCategory:   core.CategoryRead,
			wantHasContent: true,
			wantExitCode:   -1,
			wantMCPField:   false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := json.Marshal(map[string]any{
				"hook_event_name": "PostToolUse",
				"session_id":      "sess-1",
				"tool_name":       tc.toolName,
				"tool_response":   tc.toolResp,
			})
			if err != nil {
				t.Fatal(err)
			}
			parsed, ok := claudecode.ParseToolOutput(raw)
			if !ok {
				t.Fatalf("ParseToolOutput returned ok=false")
			}
			if parsed.Category != tc.wantCategory {
				t.Errorf("Category = %q, want %q", parsed.Category, tc.wantCategory)
			}
			if parsed.VerboseKind != tc.wantVerboseKind {
				t.Errorf("VerboseKind = %q, want %q", parsed.VerboseKind, tc.wantVerboseKind)
			}
			if parsed.HasContent != tc.wantHasContent {
				t.Errorf("HasContent = %v, want %v", parsed.HasContent, tc.wantHasContent)
			}
			if parsed.HasPaths != tc.wantHasPaths {
				t.Errorf("HasPaths = %v, want %v", parsed.HasPaths, tc.wantHasPaths)
			}
			if parsed.Opaque != tc.wantOpaque {
				t.Errorf("Opaque = %v, want %v", parsed.Opaque, tc.wantOpaque)
			}
			if parsed.ExitCode != tc.wantExitCode {
				t.Errorf("ExitCode = %d, want %d", parsed.ExitCode, tc.wantExitCode)
			}
			if tc.wantOpaque {
				return
			}

			if parsed.HasContent {
				parsed.Content = "compacted"
			}
			if parsed.HasPaths {
				parsed.Paths = []string{"a.go"}
			}
			ans, ok := claudecode.ReplaceToolOutput(claudecode.Output{}, parsed).(claudecode.Output)
			if !ok || ans.HookSpecificOutput == nil {
				t.Fatalf("ReplaceToolOutput did not set HookSpecificOutput")
			}
			if tc.wantMCPField {
				if len(ans.HookSpecificOutput.UpdatedMCPToolOutput) == 0 {
					t.Errorf("expected UpdatedMCPToolOutput to be set")
				}
				if len(ans.HookSpecificOutput.UpdatedToolOutput) != 0 {
					t.Errorf("unexpected UpdatedToolOutput for MCP tool: %s", ans.HookSpecificOutput.UpdatedToolOutput)
				}
			} else {
				if len(ans.HookSpecificOutput.UpdatedToolOutput) == 0 {
					t.Errorf("expected UpdatedToolOutput to be set")
				}
				if len(ans.HookSpecificOutput.UpdatedMCPToolOutput) != 0 {
					t.Errorf("unexpected UpdatedMCPToolOutput for built-in tool: %s", ans.HookSpecificOutput.UpdatedMCPToolOutput)
				}
			}
		})
	}
}
