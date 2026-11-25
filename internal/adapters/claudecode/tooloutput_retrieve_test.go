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

func TestParseToolOutput_PopulatesToolNameForMCPRetrieve(t *testing.T) {
	largeBody := strings.Repeat("retrieved line of output\n", 800) // > 16 KiB
	raw, err := json.Marshal(map[string]any{
		"hook_event_name": "PostToolUse",
		"tool_name":       "mcp__downshift__retrieve",
		"tool_input":      map[string]any{"id": "0123456789ab"},
		"tool_response":   largeBody,
	})
	if err != nil {
		t.Fatal(err)
	}
	parsed, ok := claudecode.ParseToolOutput(raw)
	if !ok {
		t.Fatal("ParseToolOutput returned ok=false")
	}
	if parsed.ToolName != "mcp__downshift__retrieve" {
		t.Fatalf("ToolName = %q, want %q", parsed.ToolName, "mcp__downshift__retrieve")
	}
	if parsed.Category != core.CategoryOther || parsed.VerboseKind != "mcp" {
		t.Fatalf("Category=%q VerboseKind=%q, want other/mcp", parsed.Category, parsed.VerboseKind)
	}
	if parsed.Content != largeBody {
		t.Fatalf("Content length = %d, want %d", len(parsed.Content), len(largeBody))
	}
}
