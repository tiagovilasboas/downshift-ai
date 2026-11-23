// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package readout

import "github.com/tiagovilasboas/downshift-ai/internal/telemetry"

// TaskOutcome is the one verdict on how a spawned task ended, folded from the
// child's end status, the rework flag and the child's own check result.
type TaskOutcome string

// Outcomes, strongest first. TaskAcceptedUnchecked is never a pass: the child
// ended normally but nothing verified the work.
const (
	TaskErrored           TaskOutcome = "errored"
	TaskReworked          TaskOutcome = "reworked"
	TaskCheckFailed       TaskOutcome = "check_failed"
	TaskAcceptedChecked   TaskOutcome = "accepted_checked"
	TaskAcceptedUnchecked TaskOutcome = "accepted_unchecked"
	TaskOutcomeUnknown    TaskOutcome = "unknown"
)

// FoldTaskOutcome applies the precedence errored > reworked > check_failed >
// accepted_checked > accepted_unchecked > unknown. A failed end that a later
// spawn redid is reworked; one nobody redid is errored. A task with no
// observed normal end is unknown.
func FoldTaskOutcome(status telemetry.ChildStatus, reworked bool, verification string) TaskOutcome {
	switch {
	case failedEnd(status) && !reworked:
		return TaskErrored
	case reworked:
		return TaskReworked
	case status != telemetry.ChildCompleted:
		return TaskOutcomeUnknown
	case verification == "failed":
		return TaskCheckFailed
	case verification == "passed":
		return TaskAcceptedChecked
	}
	return TaskAcceptedUnchecked
}

// Outcome is the folded outcome of the task.
func (t TaskRecord) Outcome() TaskOutcome {
	return FoldTaskOutcome(t.Status, t.Reworked, t.Verification)
}

// verificationMeasurable is false for a harness that cannot report a check.
func verificationMeasurable(harness string) bool {
	return telemetry.VerificationReported == nil || telemetry.VerificationReported(harness)
}

// OutcomeCounts holds task counts per outcome. A pass is only
// AcceptedChecked; AcceptedUnchecked is never added to it.
type OutcomeCounts struct {
	Errored           int `json:"errored"`
	Reworked          int `json:"reworked"`
	CheckFailed       int `json:"check_failed"`
	AcceptedChecked   int `json:"accepted_checked"`
	AcceptedUnchecked int `json:"accepted_unchecked"`
	// AcceptedNotMeasurable counts normal ends on a harness that cannot report
	// a child's check, so no check could have been seen. Never a pass.
	AcceptedNotMeasurable int `json:"accepted_not_measurable,omitempty"`
	Unknown               int `json:"unknown"`
}

// addTask counts a task. An accepted_unchecked end on a harness that cannot
// report verification goes to AcceptedNotMeasurable instead.
func (c *OutcomeCounts) addTask(t TaskRecord) {
	o := t.Outcome()
	if o == TaskAcceptedUnchecked && !verificationMeasurable(t.Harness) {
		c.AcceptedNotMeasurable++
		return
	}
	c.add(o)
}

func (c *OutcomeCounts) add(o TaskOutcome) {
	switch o {
	case TaskErrored:
		c.Errored++
	case TaskReworked:
		c.Reworked++
	case TaskCheckFailed:
		c.CheckFailed++
	case TaskAcceptedChecked:
		c.AcceptedChecked++
	case TaskAcceptedUnchecked:
		c.AcceptedUnchecked++
	default:
		c.Unknown++
	}
}
