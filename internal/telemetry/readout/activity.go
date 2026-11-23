// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package readout

import (
	"fmt"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/telemetry"
)

// ActivityLine says whether Downshift is working for the session, from its
// event log: the last spawn's state and age, or that nothing was routed yet.
func ActivityLine(events []telemetry.Event, sessionHash string, now time.Time) string {
	var last *telemetry.Event
	for _, ev := range sessionEvents(events, sessionHash) {
		if telemetry.IsDecisionRow(ev) {
			e := ev
			last = &e
		}
	}
	if last == nil {
		return "○ downshift on · no subagent routed yet"
	}
	age := now.Sub(evTime(*last)).Round(time.Second)
	if age < 0 {
		age = 0
	}
	rec := telemetry.NewSpawnRecord(*last)
	if age < telemetry.ActiveWindow {
		return fmt.Sprintf("● downshift active · last spawn %s ago: %s", age, rec.Line())
	}
	return fmt.Sprintf("○ downshift idle · last spawn %s ago: %s", age, rec.Line())
}
