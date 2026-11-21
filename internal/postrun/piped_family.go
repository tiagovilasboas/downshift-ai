// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package postrun

import (
	"strings"

	"github.com/tiagovilasboas/downshift-ai/internal/execwrap"
)

// A piped family command is one family producer plus harmless trailing
// stages: `2>&1`, `| cat`, `| head -N`, `| tail -N`, and, only when the
// family prints one record per line, `| grep [-v -i -E] pat`, `| sort`,
// `| uniq`. Anything else (xargs, tee, awk, a redirect, a substitution)
// keeps the command unclassified. A command led by a search program stays
// with execwrap.CutSearch, which already honours the agent's head/tail limit.

// lineRecordFamilies print one record per line, so grep/sort/uniq keep the
// shape the family compressor reads.
var lineRecordFamilies = map[execwrap.Family]bool{
	execwrap.FamilyLs:         true,
	execwrap.FamilyFind:       true,
	execwrap.FamilyGitBranch:  true,
	execwrap.FamilyGitLsFiles: true,
}

// extractPipedPostRunFamily is extractPostRunFamily that also accepts one
// family producer followed by harmless pipe stages.
func extractPipedPostRunFamily(command string, exit int) (execwrap.Family, []string) {
	return extractFamily(command, exit, true)
}

// splitPipeStages cuts command at its first unquoted `|` (never `||`) into
// the producer part and the trailing stages. piped is false when no pipe
// exists; ok is false when a pipe is malformed.
func splitPipeStages(command string) (head string, stages []string, piped, ok bool) {
	var quote byte
	var cuts []int
	for i := 0; i < len(command); i++ {
		c := command[i]
		switch {
		case c == '\\' && quote != '\'':
			i++
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case c == '\'' || c == '"':
			quote = c
		case c == '|':
			if i+1 < len(command) && command[i+1] == '|' {
				return "", nil, true, false
			}
			cuts = append(cuts, i)
		}
	}
	if quote != 0 {
		return "", nil, len(cuts) > 0, false
	}
	if len(cuts) == 0 {
		return command, nil, false, true
	}
	head = strings.TrimSpace(command[:cuts[0]])
	for n, at := range cuts {
		end := len(command)
		if n+1 < len(cuts) {
			end = cuts[n+1]
		}
		stages = append(stages, strings.TrimSpace(command[at+1:end]))
	}
	return head, stages, true, head != ""
}

// pipeStagesHarmless reports whether every stage is allowed for family.
func pipeStagesHarmless(stages []string, family execwrap.Family) bool {
	for _, st := range stages {
		segs, ok := splitSafeChain(st)
		if !ok || len(segs) != 1 {
			return false
		}
		words, ok := lexShellWords(segs[0])
		if !ok || len(words) == 0 {
			return false
		}
		switch words[0] {
		case "cat":
			if len(words) != 1 {
				return false
			}
		case "head", "tail":
			if !execwrap.IsLineFilter(words) {
				return false
			}
		case "sort", "uniq":
			if len(words) != 1 || !lineRecordFamilies[family] {
				return false
			}
		case "grep":
			if !lineRecordFamilies[family] || !simpleGrepStage(words[1:]) {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// simpleGrepStage accepts `[-v|-i|-E ...] pattern`: short flags from vEi only
// and exactly one pattern word.
func simpleGrepStage(args []string) bool {
	pattern := false
	for _, a := range args {
		if !pattern && len(a) > 1 && a[0] == '-' {
			for _, f := range a[1:] {
				if f != 'v' && f != 'i' && f != 'E' {
					return false
				}
			}
			continue
		}
		if pattern {
			return false
		}
		pattern = true
	}
	return pattern
}

// pipeAllowed gates a piped command: only on allowPipes, exit 0, not led by a
// search, and never a command whose raw output must not be stored.
func pipeAllowed(command string, exit int, allowPipes bool) bool {
	return allowPipes && exit == 0 && !execwrap.LedBySearch(command) && !execwrap.CompoundRefusesRawStore(command)
}

// keepsLimitedRows reports that command is a listing the agent already cut
// with head/tail and stdout fits that limit: the family compressor would drop
// rows the agent asked for, so the command stays unclassified (no byte cut).
func keepsLimitedRows(family execwrap.Family, command, stdout string) bool {
	return lineRecordFamilies[family] && execwrap.WithinRequestedLimit(command, []byte(stdout))
}
