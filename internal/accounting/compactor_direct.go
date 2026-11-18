// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package accounting

import (
	"fmt"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/execwrap"
	"github.com/tiagovilasboas/downshift-ai/internal/telemetry"
)

// DirectLabel names the runs that carry no session tag: a direct
// `downshift compactor -- ...` by an agent or a script.
const DirectLabel = "direct calls, no session"

// DirectCompactor totals the exec runs of the window that arrived as direct
// invocations (`entry == "direct"` or legacy rows with empty `entry` and empty
// `session`). Hook and post-tool-use rows are excluded even when `session` is
// empty. Only a day window applies: a session scope has no direct runs by
// definition, so it returns zero.
func DirectCompactor(execEvents []execwrap.Event, scope Scope, now time.Time) Compactor {
	if scope.Kind == ScopeSession {
		return Compactor{}
	}
	var untagged []execwrap.Event
	for _, ev := range scopeExec(execEvents, scope, now) {
		if isDirectExec(ev) {
			untagged = append(untagged, ev)
		}
	}
	return compactorTotals(untagged)
}

func isDirectExec(ev execwrap.Event) bool {
	if ev.Entry == "direct" {
		return true
	}
	return ev.Entry == "" && ev.Session == ""
}

// DirectLine is "direct calls, no session: 3, ~1.2k tokens saved"; the tokens
// are the saved_bytes / 4 estimate.
func DirectLine(c Compactor) string {
	return fmt.Sprintf("%s: %d, %s saved", DirectLabel, c.Commands, "~"+telemetry.TokensShort(c.Tokens)+" tokens")
}
