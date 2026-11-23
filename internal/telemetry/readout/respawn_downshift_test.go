// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package readout

import (
	"github.com/tiagovilasboas/downshift-ai/internal/telemetry"
	"testing"
	"time"
)

func TestFailedDownshiftWithin(t *testing.T) {
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	at := func(ago time.Duration) string { return now.Add(-ago).Format(time.RFC3339Nano) }
	moved := telemetry.Event{CorrelationID: "d1", SessionID: "s", Outcome: telemetry.OutcomeRewriteEmitted, Verdict: "DOWNSHIFT", Timestamp: at(5 * time.Minute)}
	kept := telemetry.Event{CorrelationID: "d2", SessionID: "s", Outcome: telemetry.OutcomeAllow, Verdict: "OK", Timestamp: at(5 * time.Minute)}
	end := func(id string, st telemetry.ChildStatus, ago time.Duration) telemetry.Event {
		return telemetry.Event{CorrelationID: "e-" + id, SessionID: "s", Outcome: telemetry.OutcomeChildEnded, LinkedDecision: id, ChildStatus: st, Timestamp: at(ago)}
	}
	cases := []struct {
		name   string
		events []telemetry.Event
		want   bool
	}{
		{"moved down then failed", []telemetry.Event{moved, end("d1", telemetry.ChildError, 2*time.Minute)}, true},
		{"moved down then interrupted", []telemetry.Event{moved, end("d1", telemetry.ChildInterrupted, 2*time.Minute)}, true},
		{"moved down then completed", []telemetry.Event{moved, end("d1", telemetry.ChildCompleted, 2*time.Minute)}, false},
		{"kept then failed", []telemetry.Event{kept, end("d2", telemetry.ChildError, 2*time.Minute)}, false},
		{"failure older than the window", []telemetry.Event{moved, end("d1", telemetry.ChildError, 11*time.Minute)}, false},
	}
	for _, c := range cases {
		if got := FailedDownshiftWithin(c.events, "s", now); got != c.want {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
	if FailedDownshiftWithin([]telemetry.Event{moved, end("d1", telemetry.ChildError, time.Minute)}, "other", now) {
		t.Error("another session must not match")
	}
	if FailedDownshiftWithin([]telemetry.Event{moved, end("d1", telemetry.ChildError, time.Minute)}, "", now) {
		t.Error("an empty session must not match")
	}
}
