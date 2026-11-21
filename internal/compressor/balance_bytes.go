// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// Byte budget of the balanced cut for output with too few lines for the line
// cut to help (a few huge lines: minified JSON, long log lines, base64).
const (
	byteBudgetHead = 1536 // bytes kept from the start
	byteBudgetTail = 2048 // bytes kept from the end
	byteBudgetKeep = 2048 // bytes of failure lines kept from the middle
	byteLineMax    = 400  // a longer line is cut in the middle
	byteLineHead   = 250  // bytes a cut line keeps from its start
	byteLineTail   = 100  // bytes a cut line keeps from its end
)

// BalanceBytes cuts text by bytes: a head, a tail, every failure line between
// them (up to a budget) and a middle cut of any line over byteLineMax, with
// whole lines kept where the budget allows. It reports false when the cut
// would not save at least a quarter of the text; the caller keeps what it had
// and must only use the result when the raw text is stored (retrievable).
func BalanceBytes(text []byte) ([]byte, bool) {
	lines := strings.Split(strings.TrimSuffix(string(text), "\n"), "\n")
	for i, l := range lines {
		lines[i] = midCut(l)
	}
	head, used := 0, 0
	for head < len(lines) && used+len(lines[head])+1 <= byteBudgetHead {
		used += len(lines[head]) + 1
		head++
	}
	tail, used := len(lines), 0
	for tail > head && used+len(lines[tail-1])+1 <= byteBudgetTail {
		used += len(lines[tail-1]) + 1
		tail--
	}
	out := append([]string{}, lines[:head]...)
	gap, keep, after := 0, 0, 0
	for _, l := range lines[head:tail] {
		failure := reKeepLine.MatchString(l) && !reBalanceScaffold.MatchString(l)
		if failure {
			after = balanceLogAfter
		} else if after > 0 {
			after-- // its assertion, its location
			failure = true
		}
		if failure && keep+len(l)+1 <= byteBudgetKeep {
			if gap > 0 {
				out = append(out, fmt.Sprintf("… %d lines cut (downshift retrieve)", gap))
				gap = 0
			}
			keep += len(l) + 1
			out = append(out, l)
			continue
		}
		gap++
	}
	if gap > 0 {
		out = append(out, fmt.Sprintf("… %d lines cut (downshift retrieve)", gap))
	}
	out = append(out, lines[tail:]...)
	res := []byte(strings.Join(out, "\n") + "\n")
	if len(res)*4 > len(text)*3 {
		return text, false
	}
	return res, true
}

// midCut keeps the start and the end of a line over byteLineMax.
func midCut(l string) string {
	if len(l) <= byteLineMax {
		return l
	}
	n := byteLineHead
	for n > 0 && !utf8.RuneStart(l[n]) {
		n--
	}
	m := len(l) - byteLineTail
	for m < len(l) && !utf8.RuneStart(l[m]) {
		m++
	}
	return l[:n] + fmt.Sprintf(" … (%d bytes cut) … ", m-n) + l[m:]
}
