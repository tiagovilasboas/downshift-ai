// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package accounting

// entryPostToolUse mirrors postrun.Entry; accounting may not import postrun.
const entryPostToolUse = "post_tool_use"

// Reason codes for a row filed under the unknown harness. Fixed codes, derived
// from fields the row already carries; never text.
const (
	// UnknownNoHarnessEnv: `downshift compact` ran directly, outside any hook,
	// so no harness environment was there to read.
	UnknownNoHarnessEnv = "no_harness_env"
	// UnknownPostToolUse: the post-run entry (postrun.Entry) wrote the row and
	// could not resolve a harness.
	UnknownPostToolUse = "post_tool_use"
	// UnknownHookNoHarness: a pre-run hook wrapped the command but the harness
	// was not identified.
	UnknownHookNoHarness = "hook_no_harness"
	// UnknownLegacyRow: written before the entry field existed, so the origin
	// cannot be told.
	UnknownLegacyRow = "legacy_row"
	// UnknownSkipRow: a hook skip row, which records a reason but no entry.
	UnknownSkipRow = "skip_row"
)

// UnknownHarnessReason says why an exec row has no harness, from its entry.
func UnknownHarnessReason(entry string) string {
	switch entry {
	case "direct":
		return UnknownNoHarnessEnv
	case entryPostToolUse:
		return UnknownPostToolUse
	case "hook":
		return UnknownHookNoHarness
	}
	return UnknownLegacyRow
}

// isUnknownHarness reports a row that names no harness.
func isUnknownHarness(h string) bool { return h == "" || h == UnknownHarness }

func (a *detailAcc) addUnknown(reason string) {
	if a.unknownWhy == nil {
		a.unknownWhy = map[string]int64{}
	}
	a.unknownWhy[reason]++
}
