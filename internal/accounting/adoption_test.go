// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package accounting

import (
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/telemetry"
)

func TestAdoptionLine(t *testing.T) {
	scope := Scope{Kind: ScopeAll}
	if got := AdoptionLine(BuildAdoption(nil, scope)); got != "Planner and workers: no calls yet" {
		t.Errorf("zero: %q", got)
	}
	var evs []telemetry.Event
	for i := 0; i < 6; i++ {
		evs = append(evs, telemetry.MCPTool("plan", "", telemetry.ToolCounts{Steps: 2, SpawnSteps: 1, DelegateSteps: 1}))
	}
	evs = append(evs,
		telemetry.MCPTool("delegate", "", telemetry.ToolCounts{Tasks: 4}),
		telemetry.MCPTool("delegate", "workers_off", telemetry.ToolCounts{Tasks: 1}),
		telemetry.WorkerEnded("codex", "done", nil),
	)
	want := "Planner: 6 plans, 12 steps (6 spawn · 6 delegate) · Workers: 4 delegated, 1 done, 1 refused (workers_off) · gateway 0/4"
	if got := AdoptionLine(BuildAdoption(evs, scope)); got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}

func TestAdoptionWithSkills(t *testing.T) {
	scope := Scope{Kind: ScopeAll}
	evs := []telemetry.Event{
		telemetry.SkillUsed("claude-code", "s1", "ponytail", "test"),
		telemetry.SkillUsed("claude-code", "s1", "downshift-audit", "test"),
		telemetry.SkillUsed("claude-code", "s1", "ponytail", "test"),
		telemetry.SkillUsed("claude-code", "s1", "ponytail", "test"),
		telemetry.SkillUsed("claude-code", "s1", "private-skill", "test"),
	}

	got := BuildAdoption(evs, scope)
	if got.Skills["ponytail"] != 3 || got.Skills["downshift-audit"] != 1 || got.Skills["other"] != 1 {
		t.Fatalf("Skills = %+v", got.Skills)
	}

	wantOnlySkills := "Planner and workers: no calls yet · Skills: downshift-audit 1, other 1, ponytail 3"
	if line := AdoptionLine(got); line != wantOnlySkills {
		t.Errorf("skills only line = %q, want %q", line, wantOnlySkills)
	}

	withPlanner := append(evs, telemetry.MCPTool("plan", "", telemetry.ToolCounts{Steps: 2, SpawnSteps: 1, DelegateSteps: 1}))
	wantCombined := "Planner: 1 plans, 2 steps (1 spawn · 1 delegate) · Workers: 0 delegated, 0 done · gateway 0/0 · Skills: downshift-audit 1, other 1, ponytail 3"
	if line := AdoptionLine(BuildAdoption(withPlanner, scope)); line != wantCombined {
		t.Errorf("combined line = %q, want %q", line, wantCombined)
	}
}
