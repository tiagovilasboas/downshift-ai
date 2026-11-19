// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package kirocrew_test

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/adapters/kirocrew"
)

func TestEventUsedSkills(t *testing.T) {
	cases := []struct {
		name string
		ev   kirocrew.Event
		want []string
	}{
		{
			name: "fs_read operations ponytail SKILL.md",
			ev: kirocrew.Event{
				ToolName:  "fs_read",
				ToolInput: json.RawMessage(`{"operations":[{"mode":"Line","path":"/Users/alice/.kiro/skills/ponytail/SKILL.md"}]}`),
			},
			want: []string{"ponytail"},
		},
		{
			name: "MCP-prefixed read_file downshift-audit SKILL.md",
			ev: kirocrew.Event{
				ToolName:  "@kirocrew-core/read_file",
				ToolInput: json.RawMessage(`{"path":"/Users/alice/.kiro/skills/downshift-audit/SKILL.md"}`),
			},
			want: []string{"downshift-audit"},
		},
		{
			name: "private skill collapses to other",
			ev: kirocrew.Event{
				ToolName:  "read_file",
				ToolInput: json.RawMessage(`{"path":"/Users/alice/.kiro/skills/internal-corp-skill/SKILL.md"}`),
			},
			want: []string{"other"},
		},
		{
			name: "regular file read returns nil",
			ev: kirocrew.Event{
				ToolName:  "fs_read",
				ToolInput: json.RawMessage(`{"operations":[{"mode":"Line","path":"/repo/main.go"}]}`),
			},
			want: nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.ev.UsedSkills(); !slices.Equal(got, tc.want) {
				t.Errorf("UsedSkills() = %v, want %v", got, tc.want)
			}
		})
	}
}
