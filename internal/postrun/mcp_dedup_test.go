// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package postrun

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/core"
	"github.com/tiagovilasboas/downshift-ai/internal/hookport"
	"github.com/tiagovilasboas/downshift-ai/internal/paths"
)

var mcpCaps = core.HarnessCapabilities{
	ReplacesToolOutput: []core.ToolCategory{
		core.CategoryShell, core.CategorySearch, core.CategoryRead, core.CategoryOther,
	},
}

func mcpDocOutput(toolName, args, content string) hookport.ToolOutput {
	return hookport.ToolOutput{
		Category:    core.CategoryOther,
		VerboseKind: VerboseKindMCP,
		ToolName:    toolName,
		ToolInput:   json.RawMessage(args),
		Content:     content,
		HasContent:  true,
		ExitCode:    -1,
	}
}

func TestMCPDocDedup_HitAndMiss(t *testing.T) {
	t.Setenv(paths.EnvStateDir, t.TempDir())
	keep, calls := seenKeep(t)
	o := Options{Keep: keep, Session: sessA, Balanced: true}
	body := strings.Repeat("Datadog skill documentation line with enough bytes.\n", 20) // > 512 B, < 4 KiB
	in := mcpDocOutput("mcp__datadog__load_datadog_skill", `{"skill":"logs","version":"v1"}`, body)

	first := Compact(mcpCaps, in, o)
	if first.Replaced {
		t.Fatalf("first call must miss and pass through: %+v", first)
	}
	if *calls != 1 {
		t.Fatalf("first call Keep calls = %d, want 1", *calls)
	}

	// Second identical call (with reordered JSON keys) must hit without re-writing CCR.
	inReordered := mcpDocOutput("mcp__datadog__load_datadog_skill", `{ "version": "v1", "skill": "logs" }`, body)
	second := Compact(mcpCaps, inReordered, o)
	if !second.Replaced || !second.Compacted || second.KeptID != "seenid123456" {
		t.Fatalf("second identical call must dedup: %+v", second)
	}
	if *calls != 1 {
		t.Fatalf("second call must not re-write CCR: Keep calls = %d, want 1", *calls)
	}
	if !strings.Contains(second.Output.Content, "downshift retrieve --session=") || !strings.Contains(second.Output.Content, "seenid123456") {
		t.Fatalf("pointer missing downshift retrieve hint: %q", second.Output.Content)
	}
	if second.EmittedBytes >= second.RawBytes {
		t.Fatalf("expected byte savings: %+v", second)
	}
}

func TestMCPDocDedup_DifferentArgsAndSessionMiss(t *testing.T) {
	t.Setenv(paths.EnvStateDir, t.TempDir())
	keep, calls := seenKeep(t)
	o := Options{Keep: keep, Session: sessA, Balanced: true}
	body := strings.Repeat("Skill list entry with enough content to exceed min bytes.\n", 20)

	first := Compact(mcpCaps, mcpDocOutput("mcp__plugin_dd_dd__list_datadog_skills", `{"filter":"apm"}`, body), o)
	if first.Replaced {
		t.Fatalf("first call must miss: %+v", first)
	}

	diffArgs := Compact(mcpCaps, mcpDocOutput("mcp__plugin_dd_dd__list_datadog_skills", `{"filter":"logs"}`, body), o)
	if diffArgs.Replaced {
		t.Fatalf("different args must miss: %+v", diffArgs)
	}

	diffSess := Compact(mcpCaps, mcpDocOutput("mcp__plugin_dd_dd__list_datadog_skills", `{"filter":"apm"}`, body), Options{Keep: keep, Session: strings.Repeat("cd", 32), Balanced: true})
	if diffSess.Replaced {
		t.Fatalf("different session must miss: %+v", diffSess)
	}
	if *calls != 3 {
		t.Fatalf("Keep calls = %d, want 3", *calls)
	}
}

func TestMCPDocDedup_TTLExpiry(t *testing.T) {
	dir := t.TempDir()
	keep, calls := seenKeep(t)
	o := Options{Keep: keep, Session: sessA}
	body := strings.Repeat("Static schema reference line.\n", 30)
	in := mcpDocOutput("mcp__datadog__load_datadog_skill", `{"name":"rum"}`, body)
	t0 := time.Unix(1700000000, 0)

	if _, _, changed := dedupMCPDocAt(in, o, sessA, dir, t0); changed {
		t.Fatal("first call must miss")
	}
	if _, id, changed := dedupMCPDocAt(in, o, sessA, dir, t0.Add(30*time.Minute)); !changed || id != "seenid123456" {
		t.Fatalf("within TTL must hit: changed=%v id=%q", changed, id)
	}
	if _, _, changed := dedupMCPDocAt(in, o, sessA, dir, t0.Add(mcpDedupTTL+time.Second)); changed {
		t.Fatal("after TTL expiry must miss")
	}
	if *calls != 2 {
		t.Fatalf("Keep calls = %d, want 2 (initial + post-expiry)", *calls)
	}
}

func TestMCPDocDedup_LiveToolsNeverDedup(t *testing.T) {
	t.Setenv(paths.EnvStateDir, t.TempDir())
	keep, calls := seenKeep(t)
	o := Options{Keep: keep, Session: sessA, Balanced: true}
	body := strings.Repeat("Live query result row that must never be deduped.\n", 20)

	liveTools := []string{
		"mcp__sentry__search_issues",
		"mcp__sentry__search_events",
		"mcp__sandbox__execute_code",
		"mcp__datadog__query_metrics",
		"mcp__github__get_issue",
		"mcp__jira__create_issue",
		"mcp__jira__update_issue",
	}
	for _, tool := range liveTools {
		in := mcpDocOutput(tool, `{"q":"err"}`, body)
		for i := 0; i < 2; i++ {
			res := Compact(mcpCaps, in, o)
			if res.Replaced {
				t.Fatalf("live tool %s must never dedup on call %d: %+v", tool, i+1, res)
			}
		}
	}
	if *calls != 0 {
		t.Fatalf("live tools must not be stored by mcp dedup: Keep calls = %d", *calls)
	}
}

func TestMCPDocDedup_SmallOutputPassesThrough(t *testing.T) {
	t.Setenv(paths.EnvStateDir, t.TempDir())
	keep, calls := seenKeep(t)
	o := Options{Keep: keep, Session: sessA, Balanced: true}
	small := strings.Repeat("x\n", 100) // 200 B < 512 B
	in := mcpDocOutput("mcp__datadog__load_datadog_skill", `{"name":"tiny"}`, small)

	for i := 0; i < 2; i++ {
		if res := Compact(mcpCaps, in, o); res.Replaced {
			t.Fatalf("small output must not dedup: %+v", res)
		}
	}
	if *calls != 0 {
		t.Fatalf("small output must not be stored: Keep calls = %d", *calls)
	}
}
