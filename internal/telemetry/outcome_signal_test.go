// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import "testing"

func TestAddTaskNotMeasurable(t *testing.T) {
	old := VerificationReported
	defer func() { VerificationReported = old }()
	VerificationReported = func(h string) bool { return h == "claude-code" }
	cases := []struct {
		name string
		task TaskRecord
		want OutcomeCounts
	}{
		{"unchecked where measurable", TaskRecord{Harness: "claude-code", Status: ChildCompleted}, OutcomeCounts{AcceptedUnchecked: 1}},
		{"unchecked where not", TaskRecord{Harness: "codex", Status: ChildCompleted}, OutcomeCounts{AcceptedNotMeasurable: 1}},
		{"checked stays", TaskRecord{Harness: "codex", Status: ChildCompleted, Verification: "passed"}, OutcomeCounts{AcceptedChecked: 1}},
		{"unknown stays", TaskRecord{Harness: "codex", Status: ChildLaunched}, OutcomeCounts{Unknown: 1}},
	}
	for _, c := range cases {
		var got OutcomeCounts
		got.addTask(c.task)
		if got != c.want {
			t.Errorf("%s: got %+v, want %+v", c.name, got, c.want)
		}
	}
	VerificationReported = nil
	var got OutcomeCounts
	got.addTask(TaskRecord{Harness: "codex", Status: ChildCompleted})
	if got.AcceptedUnchecked != 1 {
		t.Errorf("nil hook must not reclassify: %+v", got)
	}
}
