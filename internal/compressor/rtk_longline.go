// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"strings"
)

const (
	// longLineMax is the length above which a line is capped.
	longLineMax = 300
	// longLineKeep is how many bytes of a capped line stay.
	longLineKeep = 200
	// rtkMinGain is the least share of bytes a lossy text pass must save.
	rtkMinGain = 0.10
	// rtkMinBytes is the smallest output the lossy text passes touch.
	rtkMinBytes = 1024
)

// CapLongLines cuts each line over 300 bytes to its first 200 plus a
// "… (K bytes cut)" marker. A line with a danger word stays whole. It is lossy:
// the caller runs it only once the original is stored, and never on a file
// read (markdown paragraphs are legitimately long). It returns false when
// nothing is cut or the gain is under 10%.
func CapLongLines(data []byte) ([]byte, bool) {
	if len(data) < rtkMinBytes || len(data) > MaxProcessableBytes {
		return data, false
	}
	lines := strings.Split(string(data), "\n")
	changed := false
	for i, l := range lines {
		if len(l) <= longLineMax || genericHasDanger(l) {
			continue
		}
		cut := longLineKeep
		for cut > 0 && !runeStart(l[cut]) { // never split a UTF-8 sequence
			cut--
		}
		lines[i] = l[:cut] + fmt.Sprintf(" … (%d bytes cut)", len(l)-cut)
		changed = true
	}
	if !changed {
		return data, false
	}
	out := strings.Join(lines, "\n")
	if float64(len(data)-len(out)) < rtkMinGain*float64(len(data)) {
		return data, false
	}
	return []byte(out), true
}

func runeStart(b byte) bool { return b&0xC0 != 0x80 }
