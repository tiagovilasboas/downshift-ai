// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import "strings"

// unquotedPipes returns the byte indices of every single unquoted `|` in
// command, ignoring `||` and any `|` inside single or double quotes.
func unquotedPipes(command string) []int {
	var pipes []int
	var quote byte
	for i := 0; i < len(command); i++ {
		c := command[i]
		if quote != 0 {
			if c == quote {
				quote = 0
			}
			continue
		}
		if c == '\'' || c == '"' {
			quote = c
			continue
		}
		if c == '|' {
			if i+1 < len(command) && command[i+1] == '|' {
				i++
				continue
			}
			pipes = append(pipes, i)
		}
	}
	return pipes
}

// parsePipeStages validates 1 or 2 pipe tail stages (e.g. `tail -15`,
// `grep FAIL | head -20`, `sort | uniq -c`). Every stage must pass either
// lineFilter or parsePipeFilter.
func parsePipeStages(stages []string) (out string, segs []string, kind pipeKindFlags, ok bool) {
	if len(stages) == 0 || len(stages) > 2 {
		return "", nil, pipeKindFlags{}, false
	}
	outs := make([]string, len(stages))
	for i, stage := range stages {
		stageOut, stageSegs, stageKind, stageOK := parsePipeStage(stage)
		if !stageOK {
			return "", nil, pipeKindFlags{}, false
		}
		outs[i] = stageOut
		segs = append(segs, stageSegs...)
		kind.filter = kind.filter || stageKind.filter
		kind.quoted = kind.quoted || stageKind.quoted
	}
	return strings.Join(outs, pipeSep), segs, kind, true
}

func parsePipeStage(stage string) (out string, segs []string, kind pipeKindFlags, ok bool) {
	trimmed := strings.TrimSpace(stage)
	if trimmed == "" {
		return "", nil, pipeKindFlags{}, false
	}
	words := strings.Fields(trimmed)
	if allSafe(trimmed) && lineFilter(words) {
		seg := strings.Join(words, " ")
		return seg, []string{seg}, pipeKindFlags{}, true
	}
	pipeOut, segs, quoted, filterOK := parsePipeFilter(trimmed)
	if !filterOK {
		return "", nil, pipeKindFlags{}, false
	}
	return pipeOut, segs, pipeKindFlags{filter: true, quoted: quoted}, true
}
