// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package readout

import (
	"github.com/tiagovilasboas/downshift-ai/internal/telemetry"
	"testing"
)

func TestFoldTaskOutcome(t *testing.T) {
	cases := []struct {
		name     string
		status   telemetry.ChildStatus
		reworked bool
		verif    string
		want     TaskOutcome
	}{
		{"error beats passed", telemetry.ChildError, false, "passed", TaskErrored},
		{"interrupted not redone", telemetry.ChildInterrupted, false, "", TaskErrored},
		{"interrupted plus respawn", telemetry.ChildInterrupted, true, "", TaskReworked},
		{"error plus respawn", telemetry.ChildError, true, "passed", TaskReworked},
		{"check failed", telemetry.ChildCompleted, false, "failed", TaskCheckFailed},
		{"checked", telemetry.ChildCompleted, false, "passed", TaskAcceptedChecked},
		{"verification none", telemetry.ChildCompleted, false, "none", TaskAcceptedUnchecked},
		{"verification missing", telemetry.ChildCompleted, false, "", TaskAcceptedUnchecked},
		{"no end", "", false, "passed", TaskOutcomeUnknown},
	}
	for _, c := range cases {
		if got := FoldTaskOutcome(c.status, c.reworked, c.verif); got != c.want {
			t.Errorf("%s: got %s want %s", c.name, got, c.want)
		}
	}
}

func TestBuildTasksLastVerificationWins(t *testing.T) {
	dec := telemetry.Event{Timestamp: "2026-01-01T00:00:00Z", Harness: "claude-code", SessionID: "s", Outcome: telemetry.OutcomeRewriteEmitted}
	key := telemetry.DecisionKey(dec)
	u1 := telemetry.Event{Timestamp: "2026-01-01T00:01:00Z", Outcome: telemetry.OutcomeUsage, LinkedDecision: key, Verification: "failed"}
	u2 := telemetry.Event{Timestamp: "2026-01-01T00:02:00Z", Outcome: telemetry.OutcomeUsage, LinkedDecision: key, Verification: "passed"}
	tasks := BuildTasks([]telemetry.Event{dec, u1, u2})
	if len(tasks) != 1 || tasks[0].Verification != "passed" || tasks[0].Outcome() != TaskAcceptedChecked {
		t.Fatalf("got %+v", tasks)
	}
	rep := BuildReport([]telemetry.Event{dec, u1, u2}, 0)
	if rep.Outcomes.AcceptedChecked != 1 {
		t.Fatalf("outcomes %+v", rep.Outcomes)
	}
}
