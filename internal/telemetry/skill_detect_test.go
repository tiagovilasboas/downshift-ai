// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"slices"
	"testing"
)

func TestSkillsFromToolCall(t *testing.T) {
	cases := []struct {
		name  string
		tool  string
		input map[string]any
		want  []string
	}{
		{
			name:  "claude Skill tool ponytail",
			tool:  "Skill",
			input: map[string]any{"skill": "ponytail", "args": "secret args"},
			want:  []string{"ponytail"},
		},
		{
			name:  "opencode lowercase skill tool name field",
			tool:  "skill",
			input: map[string]any{"name": "downshift-audit"},
			want:  []string{"downshift-audit"},
		},
		{
			name:  "Skill tool private skill collapses to other",
			tool:  "Skill",
			input: map[string]any{"skill": "acme-internal-deploy"},
			want:  []string{"other"},
		},
		{
			name:  "read_file on kiro ponytail SKILL.md",
			tool:  "read_file",
			input: map[string]any{"path": "/Users/alice/.kiro/skills/ponytail/SKILL.md"},
			want:  []string{"ponytail"},
		},
		{
			name:  "read on opencode downshift-effort SKILL.md via filePath",
			tool:  "read",
			input: map[string]any{"filePath": "/home/alice/.config/opencode/skills/downshift-effort/SKILL.md"},
			want:  []string{"downshift-effort"},
		},
		{
			name: "fs_read operations array deduplicates duplicate skill reads",
			tool: "fs_read",
			input: map[string]any{
				"operations": []any{
					map[string]any{"mode": "Line", "path": `C:\Users\alice\.kiro\skills\ponytail\SKILL.md`},
					map[string]any{"mode": "Line", "path": "/Users/alice/.kiro/skills/ponytail/SKILL.md"},
					map[string]any{"mode": "Line", "path": "/Users/alice/.kiro/skills/secret-skill/SKILL.md"},
				},
			},
			want: []string{"ponytail", "other"},
		},
		{
			name:  "Read on non-skill file returns nil",
			tool:  "Read",
			input: map[string]any{"file_path": "/Users/alice/repo/README.md"},
			want:  nil,
		},
		{
			name:  "Read on SKILL.md outside skills directory returns nil",
			tool:  "Read",
			input: map[string]any{"file_path": "/Users/alice/repo/docs/SKILL.md"},
			want:  nil,
		},
		{
			name:  "Bash tool ignored",
			tool:  "Bash",
			input: map[string]any{"command": "cat ~/.kiro/skills/ponytail/SKILL.md"},
			want:  nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := SkillsFromToolCall(tc.tool, tc.input)
			if !slices.Equal(got, tc.want) {
				t.Errorf("SkillsFromToolCall(%q, %v) = %v, want %v", tc.tool, tc.input, got, tc.want)
			}
		})
	}
}

func TestSkillsFromPrompt(t *testing.T) {
	cases := []struct {
		name   string
		prompt string
		want   []string
	}{
		{
			name:   "slash ponytail alone",
			prompt: "/ponytail",
			want:   []string{"ponytail"},
		},
		{
			name:   "dollar ponytail mid-sentence",
			prompt: "Please use $ponytail to keep this diff small.",
			want:   []string{"ponytail"},
		},
		{
			name:   "multiple skills deduplicated",
			prompt: "/downshift:ponytail and /downshift-audit and /ponytail and /superpowers:brainstorming!",
			want:   []string{"ponytail", "downshift-audit", "superpowers:brainstorming"},
		},
		{
			name:   "slash commands and env vars are not emitted as other",
			prompt: "/clear /help /model $HOME $PATH $100 /private-secret-skill",
			want:   nil,
		},
		{
			name:   "paths and URLs containing ponytail are ignored",
			prompt: "read /Users/alice/.cursor/skills/ponytail/SKILL.md or https://example.com/ponytail",
			want:   nil,
		},
		{
			name:   "adjacent tokens separated by single space",
			prompt: "/ponytail /downshift-setup",
			want:   []string{"ponytail", "downshift-setup"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := SkillsFromPrompt(tc.prompt)
			if !slices.Equal(got, tc.want) {
				t.Errorf("SkillsFromPrompt(%q) = %v, want %v", tc.prompt, got, tc.want)
			}
		})
	}
}
