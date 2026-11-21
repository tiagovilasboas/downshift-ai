// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"strings"

	"github.com/tiagovilasboas/downshift-ai/internal/lazyre"
)

// reANSI matches CSI sequences (colour, cursor, erase) and OSC sequences
// (titles, hyperlinks) ended by BEL or ST.
var reANSI = lazyre.New(`\x1b\[[0-?]*[ -/]*[@-~]|\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)`)

// LosslessText is the pass for output no compressor recognises: it strips
// ANSI escapes, keeps only the final state of a carriage-return overwritten
// line and folds runs of identical lines into one plus "(repeated N times)".
// It never drops a line with a danger word (error, fail, panic, warn, ...):
// such a line is kept as is and never folded. It reports whether the result
// is smaller than input; otherwise input is returned unchanged.
func LosslessText(input []byte) ([]byte, bool) {
	if len(input) > MaxProcessableBytes {
		return input, false
	}
	text := reANSI.ReplaceAllString(string(input), "")
	lines := strings.Split(text, "\n")
	lines = collapseCarriageReturns(lines)
	lines = foldIdentical(lines)
	out := strings.Join(lines, "\n")
	if len(out) >= len(input) {
		return input, false
	}
	return []byte(out), true
}

// collapseCarriageReturns keeps the last non-empty segment of a line that a
// "\r" overwrote, plus any earlier segment that holds a danger word.
func collapseCarriageReturns(lines []string) []string {
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		l = strings.TrimSuffix(l, "\r")
		if !strings.Contains(l, "\r") {
			out = append(out, l)
			continue
		}
		segs := strings.Split(l, "\r")
		last := len(segs) - 1
		for last > 0 && segs[last] == "" {
			last--
		}
		for _, s := range segs[:last] {
			if s != "" && genericHasDanger(s) {
				out = append(out, s)
			}
		}
		out = append(out, segs[last])
	}
	return out
}

// foldIdentical folds runs of identical consecutive non-blank lines without
// a danger word into the line and a repeat marker.
func foldIdentical(lines []string) []string {
	out := make([]string, 0, len(lines))
	for i := 0; i < len(lines); {
		j := i + 1
		for j < len(lines) && lines[j] == lines[i] {
			j++
		}
		out = append(out, lines[i])
		if n := j - i; n > 1 && lines[i] != "" && !genericHasDanger(lines[i]) {
			out = append(out, fmt.Sprintf("(repeated %d times)", n))
		} else {
			out = append(out, lines[i+1:j]...)
		}
		i = j
	}
	return out
}
