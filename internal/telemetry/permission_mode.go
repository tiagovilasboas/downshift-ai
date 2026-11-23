// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

// builtinPermissionModes are the permission mode names Claude Code ships. They
// say nothing about the user's work, so they are safe to store by name.
var builtinPermissionModes = map[string]bool{
	"default":           true,
	"acceptEdits":       true,
	"bypassPermissions": true,
	"plan":              true,
	"auto":              true,
	"dontAsk":           true,
}

// PermissionModeLabel turns a harness permission mode into what may be stored.
// Only the built-in names are kept; any other value is "other", so a value the
// harness adds later never reaches the event log verbatim. An absent mode stays
// empty, which keeps the field out of the record.
func PermissionModeLabel(raw string) string {
	switch {
	case raw == "":
		return ""
	case builtinPermissionModes[raw]:
		return raw
	default:
		return "other"
	}
}
