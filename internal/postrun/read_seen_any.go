// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package postrun

import (
	"fmt"

	"github.com/tiagovilasboas/downshift-ai/internal/execwrap"
)

// seenAnyMaxStderr bounds the stderr a repeated output may carry: a command
// that also complains is not the same output the model can skip.
const seenAnyMaxStderr = 1 << 10

// pointerFunc renders the short stand-in for a repeated text; arg is the
// ready retrieve argument (session flag and handle).
type pointerFunc func(raw, arg string) string

func filePointer(raw, arg string) string {
	return fmt.Sprintf("[downshift: unchanged, same %d lines (%d B) already read in this session; downshift retrieve %s]\n",
		countLines(raw), len(raw), arg)
}

func outputPointer(raw, arg string) string {
	return fmt.Sprintf("[downshift: same output as an earlier run in this session (%d lines, %d B); downshift retrieve %s]\n",
		countLines(raw), len(raw), arg)
}

// dedupShellAny dedups the stdout of an exit-0 shell command that no family
// compacts (FamilyNone) when the same bytes were already sent in this session.
// A command whose raw output must not be stored (credential-bearing compounds)
// is never deduped. Classified commands keep their own compaction.
func dedupShellAny(command, stdout, stderr string, o Options, payloadSession string) (string, string, bool) {
	if len(stderr) > seenAnyMaxStderr || execwrap.CompoundRefusesRawStore(command) {
		return stdout, "", false
	}
	if fam, _ := execwrap.SimpleFamily(command); fam != execwrap.FamilyNone {
		return stdout, "", false
	}
	if fam, _ := extractPostRunFamily(command, 0); fam != execwrap.FamilyNone {
		return stdout, "", false
	}
	return seenReadWith(stdout, o, seenSession(o, payloadSession), outputPointer)
}
