// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package readout

import (
	"github.com/tiagovilasboas/downshift-ai/internal/telemetry"
	"testing"
)

func TestAddTaskNotMeasurable(t *testing.T) {
	old := telemetry.VerificationReported
	defer func() { telemetry.VerificationReported = old }()
	telemetry.VerificationReported = func(h string) bool { return h == "claude-code" }
	cases := []struct {
		name string
		task TaskRecord
		want OutcomeCounts
	}{
		{"unchecked where measurable", TaskRecord{Harness: "claude-code", Status: telemetry.ChildCompleted}, OutcomeCounts{AcceptedUnchecked: 1}},
		{"unchecked where not", TaskRecord{Harness: "codex", Status: telemetry.ChildCompleted}, OutcomeCounts{AcceptedNotMeasurable: 1}},
		{"checked stays", TaskRecord{Harness: "codex", Status: telemetry.ChildCompleted, Verification: "passed"}, OutcomeCounts{AcceptedChecked: 1}},
		{"unknown stays", TaskRecord{Harness: "codex", Status: telemetry.ChildLaunched}, OutcomeCounts{Unknown: 1}},
	}
	for _, c := range cases {
		var got OutcomeCounts
		got.addTask(c.task)
		if got != c.want {
			t.Errorf("%s: got %+v, want %+v", c.name, got, c.want)
		}
	}
	telemetry.VerificationReported = nil
	var got OutcomeCounts
	got.addTask(TaskRecord{Harness: "codex", Status: telemetry.ChildCompleted})
	if got.AcceptedUnchecked != 1 {
		t.Errorf("nil hook must not reclassify: %+v", got)
	}
}
