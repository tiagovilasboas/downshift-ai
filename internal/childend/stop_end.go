// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package childend

import (
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/hookport"
	"github.com/tiagovilasboas/downshift-ai/internal/telemetry"
)

// hasEnd reports whether a stop event carries a failed end (error or cancel).
// A stop alone never proves completion, so no other status is recorded.
func hasEnd(ce hookport.ChildEnd) bool {
	return ce.Status == telemetry.ChildError || ce.Status == telemetry.ChildInterrupted
}

// recordStopEnd records how a child ended from its stop event (a transcript
// read: an API error or a cancel). A background child reports only "launched"
// at spawn time and an end without usage leaves no other trace, so without
// this its task folds to unknown. The agent id joins the stop to the "resolved"
// observation made at launch, and through it to the decision. It records
// nothing when there is no such observation, or when the same agent already
// has this status (SubagentStop can fire more than once).
func recordStopEnd(ce hookport.ChildEnd, agentHash, binaryVersion string, prior []telemetry.Event, now time.Time) {
	if !hasEnd(ce) || agentHash == "" {
		return
	}
	var resolved *telemetry.Event
	for i := range prior {
		ev := &prior[i]
		if ev.AgentHash != agentHash {
			continue
		}
		if ev.Outcome == telemetry.OutcomeChildEnded && ev.ChildStatus == ce.Status {
			return
		}
		if ev.Outcome == telemetry.OutcomeResolved {
			resolved = ev
		}
	}
	if resolved == nil {
		return
	}
	var target *telemetry.Event
	for i := range prior {
		if !telemetry.IsObservationOutcome(prior[i].Outcome) && decisionKey(prior[i]) == resolved.LinkedDecision {
			target = &prior[i]
		}
	}
	if target == nil {
		return
	}
	ev := childEndEvent(ce, target, agentHash, binaryVersion, now)
	ev.ToModel = resolved.ToModel // the model the harness resolved at launch
	telemetry.Record(ev)
}
