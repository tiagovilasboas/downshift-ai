// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"strings"
)

// nearRepeatMinRun is the shortest run of near-identical lines that folds.
const nearRepeatMinRun = 3

// nearRepeatKey is a line with every digit run replaced by "#", so lines that
// differ only by a timestamp, id or counter share a key.
func nearRepeatKey(l string) string {
	var b strings.Builder
	inDigits := false
	for i := 0; i < len(l); i++ {
		if c := l[i]; c >= '0' && c <= '9' {
			if !inDigits {
				b.WriteByte('#')
			}
			inDigits = true
			continue
		}
		inDigits = false
		b.WriteByte(l[i])
	}
	return b.String()
}

// FoldNearRepeats folds a run of 3 or more consecutive lines that are equal
// once digit runs are masked (timestamps, ids, counters) into the first line,
// a "(N similar lines)" marker and the last line. Blank lines and lines with a
// danger word are never folded; exact repeats are LosslessText's job. It is
// lossy: the caller runs it only once the original is stored. It returns false
// when nothing folds or the gain is under 10%.
func FoldNearRepeats(data []byte) ([]byte, bool) {
	if len(data) < rtkMinBytes || len(data) > MaxProcessableBytes {
		return data, false
	}
	lines := strings.Split(string(data), "\n")
	out := make([]string, 0, len(lines))
	folded := false
	for i := 0; i < len(lines); {
		j := i + 1
		if foldable(lines[i]) {
			key := nearRepeatKey(lines[i])
			for j < len(lines) && foldable(lines[j]) && nearRepeatKey(lines[j]) == key {
				j++
			}
		}
		if j-i >= nearRepeatMinRun {
			out = append(out, lines[i], fmt.Sprintf("(%d similar lines)", j-i-2), lines[j-1])
			folded = true
		} else {
			out = append(out, lines[i:j]...)
		}
		i = j
	}
	if !folded {
		return data, false
	}
	res := strings.Join(out, "\n")
	if float64(len(data)-len(res)) < rtkMinGain*float64(len(data)) {
		return data, false
	}
	return []byte(res), true
}

func foldable(l string) bool {
	return strings.TrimSpace(l) != "" && !genericHasDanger(l)
}
