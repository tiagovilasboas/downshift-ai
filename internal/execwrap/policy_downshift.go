// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

// FamilyDownshift is the allowlisted family for read-only downshift CLI
// subcommands. Mutating and wrapper subcommands (compact, exec, compactor,
// setup, enable, uninstall, retrieve) are excluded so the wrapper never
// recursively wraps itself or mutates configuration.
const FamilyDownshift Family = "downshift"

var downshiftReadOnlySubcommands = map[string]bool{
	"stats":    true,
	"discover": true,
	"report":   true,
	"doctor":   true,
	"version":  true,
	"help":     true,
	"session":  true,
	"models":   true,
}

// downshiftFamily detects read-only downshift CLI subcommands.
func downshiftFamily(argv []string) Family {
	if len(argv) < 2 || argv[0] != "downshift" {
		return FamilyNone
	}
	if downshiftReadOnlySubcommands[argv[1]] {
		return FamilyDownshift
	}
	return FamilyNone
}

// isWrappedDownshift reports whether argv is already a downshift compactor
// invocation (compact, exec, compactor), which must never be wrapped again.
func isWrappedDownshift(argv []string) bool {
	if len(argv) < 2 || argv[0] != "downshift" {
		return false
	}
	switch argv[1] {
	case "compact", "exec", "compactor":
		return true
	}
	return false
}
