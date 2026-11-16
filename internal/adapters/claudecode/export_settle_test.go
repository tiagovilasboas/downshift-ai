// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package claudecode

import "time"

// The production settle window waits a quarter second on every SubagentStop.
// Tests that do not exercise the wait use a short one so the suite stays fast;
// the ones that do set their own with SetTranscriptSettle.
func init() {
	transcriptSettle = settleWindow{poll: 5 * time.Millisecond, quiet: 20 * time.Millisecond, deadline: 200 * time.Millisecond}
}

// SetTranscriptSettle swaps the SubagentStop settle window for one test and
// returns the restore function.
func SetTranscriptSettle(poll, quiet, deadline time.Duration) func() {
	prev := transcriptSettle
	transcriptSettle = settleWindow{poll: poll, quiet: quiet, deadline: deadline}
	return func() { transcriptSettle = prev }
}
