// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"bytes"
	"strconv"
	"strings"

	"github.com/tiagovilasboas/downshift-ai/internal/compressor"
)

// ════════════════════════════════════════════════════════════════════════════
// DOMAIN: Balanced cut of long listings — ls, find, git list outputs
// ════════════════════════════════════════════════════════════════════════════

// listingFamilies are the families whose output, once their own compressor
// ran, may take the byte-budget head/tail cut when it is still over the
// balanced threshold: listings of names (ls, find,
// git ls-files, log, branch, tag and the other git subcommands). Their raw
// output is stored in CCR and the marker names the retrieve id.
var listingFamilies = map[Family]bool{
	FamilyLs: true, FamilyFind: true, FamilyGitLsFiles: true,
	FamilyGitLog: true, FamilyGitBranch: true, FamilyGitTag: true, FamilyGitOther: true,
}

// nameListFlags are the git diff options that print a list of names, not a
// patch: the output is cut as a list.
var nameListFlags = map[string]bool{"--name-only": true, "--name-status": true, "--stat": true}

// isNameListDiff reports whether argv is a git diff that prints names.
func isNameListDiff(family Family, argv []string) bool {
	if family != FamilyGitDiff {
		return false
	}
	for _, a := range argv {
		if nameListFlags[a] {
			return true
		}
	}
	return false
}

// isListing reports whether a run's output is a list of names (not the
// unclassified family): it is cut only on a clean exit and when the agent did
// not size the output itself with -n N.
func isListing(family Family, argv []string) bool {
	return (family != FamilyNone && listingFamilies[family]) || isNameListDiff(family, argv)
}

// cutBody is the balanced cut of body, the text a run would print. Unclassified
// output takes the line cut, or the byte cut when the lines are too few. A
// listing takes the shorter of the two, and only when its exit is 0 and the
// agent's own limit does not already hold it. Any other family takes the line
// cut of its shape.
func cutBody(family Family, argv []string, exitCode int, raw, body []byte, lim compressor.BalanceLimits) ([]byte, bool) {
	cut, applied := compressor.BalanceWith(balanceKindOf(family, argv), body, lim)
	switch {
	case family == FamilyNone:
		if !applied {
			cut, applied = compressor.BalanceBytesScaled(body)
		}
	case isListing(family, argv):
		if exitCode != 0 || argvWithinLimit(argv, raw) {
			return nil, false
		}
		if bc, ok := compressor.BalanceBytes(body); ok && (!applied || len(bc) < len(cut)) {
			cut, applied = bc, true
		}
	}
	return cut, applied
}

// argvWithinLimit reports whether argv asked for at most N lines (-n N, -nN,
// --max-count=N, -N) and data holds no more than N: the agent chose them.
func argvWithinLimit(argv []string, data []byte) bool {
	n, ok := argvLineLimit(argv)
	if !ok {
		return false
	}
	lines := bytes.Count(data, []byte("\n"))
	if len(data) > 0 && data[len(data)-1] != '\n' {
		lines++
	}
	return lines <= n
}

// argvLineLimit reads the line limit a command line carries.
func argvLineLimit(argv []string) (int, bool) {
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		var val string
		switch {
		case (a == "-n" || a == "--max-count") && i+1 < len(argv):
			val = argv[i+1]
		case strings.HasPrefix(a, "--max-count="):
			val = a[len("--max-count="):]
		case strings.HasPrefix(a, "-n") && len(a) > 2:
			val = a[2:]
		case len(a) > 1 && a[0] == '-' && a[1] >= '0' && a[1] <= '9':
			val = a[1:]
		default:
			continue
		}
		if n, err := strconv.Atoi(val); err == nil && n >= 0 {
			return n, true
		}
	}
	return 0, false
}
