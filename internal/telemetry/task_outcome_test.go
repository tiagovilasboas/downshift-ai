// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import "testing"

func TestFoldTaskOutcome(t *testing.T) {
	cases := []struct {
		name     string
		status   ChildStatus
		reworked bool
		verif    string
		want     TaskOutcome
	}{
		{"error beats passed", ChildError, false, "passed", TaskErrored},
		{"interrupted not redone", ChildInterrupted, false, "", TaskErrored},
		{"interrupted plus respawn", ChildInterrupted, true, "", TaskReworked},
		{"error plus respawn", ChildError, true, "passed", TaskReworked},
		{"check failed", ChildCompleted, false, "failed", TaskCheckFailed},
		{"checked", ChildCompleted, false, "passed", TaskAcceptedChecked},
		{"verification none", ChildCompleted, false, "none", TaskAcceptedUnchecked},
		{"verification missing", ChildCompleted, false, "", TaskAcceptedUnchecked},
		{"no end", "", false, "passed", TaskOutcomeUnknown},
	}
	for _, c := range cases {
		if got := FoldTaskOutcome(c.status, c.reworked, c.verif); got != c.want {
			t.Errorf("%s: got %s want %s", c.name, got, c.want)
		}
	}
}

func TestBuildTasksLastVerificationWins(t *testing.T) {
	dec := Event{Timestamp: "2026-01-01T00:00:00Z", Harness: "claude-code", SessionID: "s", Outcome: OutcomeRewriteEmitted}
	key := DecisionKey(dec)
	u1 := Event{Timestamp: "2026-01-01T00:01:00Z", Outcome: OutcomeUsage, LinkedDecision: key, Verification: "failed"}
	u2 := Event{Timestamp: "2026-01-01T00:02:00Z", Outcome: OutcomeUsage, LinkedDecision: key, Verification: "passed"}
	tasks := BuildTasks([]Event{dec, u1, u2})
	if len(tasks) != 1 || tasks[0].Verification != "passed" || tasks[0].Outcome() != TaskAcceptedChecked {
		t.Fatalf("got %+v", tasks)
	}
	rep := BuildReport([]Event{dec, u1, u2}, 0)
	if rep.Outcomes.AcceptedChecked != 1 {
		t.Fatalf("outcomes %+v", rep.Outcomes)
	}
}
