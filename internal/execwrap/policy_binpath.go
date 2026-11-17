// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"path/filepath"
	"strings"
)

// knownBinPrefixes lists standard system and package-manager binary
// directories whose direct children may be resolved to their basename for
// allowlist matching. Arbitrary user or project directories (for example
// /tmp/evil/git or /Users/someone/bin/gh) are intentionally excluded so an
// untrusted binary path is never auto-wrapped.
var knownBinPrefixes = []string{
	"/bin/",
	"/usr/bin/",
	"/usr/local/bin/",
	"/opt/homebrew/bin/",
	"/opt/local/bin/",
	"/home/linuxbrew/.linuxbrew/bin/",
	"/snap/bin/",
	"/run/current-system/sw/bin/",
	"/usr/local/go/bin/",
}

// isKnownBinPath reports whether prog is a direct child of a known system or
// package-manager binary directory (with no traversal or nested subdirectories).
func isKnownBinPath(prog string) bool {
	slash := filepath.ToSlash(prog)
	for _, prefix := range knownBinPrefixes {
		if rest, ok := strings.CutPrefix(slash, prefix); ok {
			return validLocalToolName(rest)
		}
	}
	return false
}

// resolveBinaryBase returns the basename of prog via filepath.Base if prog is
// a known binary absolute path, or prog unchanged otherwise.
func resolveBinaryBase(prog string) (string, bool) {
	if !isKnownBinPath(prog) {
		return prog, false
	}
	base := filepath.Base(prog)
	if base == "" || base == "." || base == ".." {
		return prog, false
	}
	return base, true
}

// normalizeBinaryPath returns a copy of argv with argv[0] replaced by its
// basename when argv[0] is a known binary absolute path, and reports whether
// argv[0] was resolved.
func normalizeBinaryPath(argv []string) ([]string, bool) {
	if len(argv) == 0 {
		return argv, false
	}
	base, ok := resolveBinaryBase(argv[0])
	if !ok {
		return argv, false
	}
	out := make([]string, len(argv))
	out[0] = base
	copy(out[1:], argv[1:])
	return out, true
}

// isDownshiftBinary reports whether prog names the downshift binary itself,
// either bare or via a known binary path.
func isDownshiftBinary(prog string) bool {
	if prog == "downshift" {
		return true
	}
	base, ok := resolveBinaryBase(prog)
	return ok && base == "downshift"
}
