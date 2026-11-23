// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry_test

import (
	"github.com/tiagovilasboas/downshift-ai/internal/telemetry"
	"github.com/tiagovilasboas/downshift-ai/internal/telemetry/readout"
	"testing"
	"time"
)

func TestSkillNameLabel(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"ponytail", "ponytail"},
		{"downshift:ponytail", "ponytail"},
		{"downshift-audit", "downshift-audit"},
		{"downshift:downshift-setup", "downshift-setup"},
		{"superpowers:brainstorming", "superpowers:brainstorming"},
		{"superpowers:test-driven-development", "superpowers:test-driven-development"},
		{"private-acme-deploy", "other"},
		{"/Users/secret/path/skill", "other"},
		{"superpowers:bad skill with spaces", "other"},
		{"sk-ant-SECRETVALUE123", "other"},
	}
	for _, tc := range cases {
		if got := telemetry.SkillNameLabel(tc.in); got != tc.want {
			t.Errorf("SkillNameLabel(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestSkillUsedIsIgnoredAsDecisionAndCost(t *testing.T) {
	now := time.Now().UTC()
	dec := telemetry.Event{
		CorrelationID: "d1", Timestamp: now.Add(-2 * time.Minute).Format(time.RFC3339Nano),
		Harness: "claude-code", SessionID: "s1", Outcome: telemetry.OutcomeAllow, Verdict: "OK",
	}
	childErr := telemetry.Event{
		CorrelationID: "e1", Timestamp: now.Add(-time.Minute).Format(time.RFC3339Nano),
		Harness: "claude-code", SessionID: "s1", Outcome: telemetry.OutcomeChildEnded,
		LinkedDecision: "d1", ChildStatus: telemetry.ChildError,
	}
	skill := telemetry.SkillUsed("claude-code", "s1", "ponytail", "v1")
	skill.Timestamp = now.Format(time.RFC3339Nano)

	if !telemetry.IsObservationOutcome(telemetry.OutcomeSkillUsed) {
		t.Fatal("OutcomeSkillUsed must be an observation outcome")
	}
	if !telemetry.IsCostOnlyOutcome(telemetry.OutcomeSkillUsed) {
		t.Fatal("OutcomeSkillUsed must be cost-only/observation excluded from decisions")
	}
	if telemetry.IsDecisionRow(skill) {
		t.Fatal("OutcomeSkillUsed must not be treated as a decision row")
	}
	if readout.IsTaskDecision(skill) {
		t.Fatal("OutcomeSkillUsed must not be treated as a task decision")
	}
	s := readout.Aggregate([]telemetry.Event{dec, skill})
	if s.Total != 1 || s.UsageRecords != 0 || s.UsageObserved != 0 {
		t.Fatalf("Aggregate counted skill_used as decision or usage: %+v", s)
	}
	tasks := readout.BuildTasks([]telemetry.Event{dec, childErr, skill})
	if len(tasks) != 1 {
		t.Fatalf("BuildTasks returned %d tasks, want 1: %+v", len(tasks), tasks)
	}
	if retries := readout.RespawnAfterFailure([]telemetry.Event{dec, childErr, skill}, 10*time.Minute); len(retries) != 0 {
		t.Fatalf("RespawnAfterFailure counted skill_used as respawn: %+v", retries)
	}
}
