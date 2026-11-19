// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import "strings"

type stderrRedirect uint8

const (
	redirNone stderrRedirect = iota
	redirMerge
	redirDiscard
)

// extractStderrRedirect finds at most one standalone stderr-only redirect
// token (`2>&1`, `2>/dev/null`, or `2> /dev/null`) outside quotes in command
// and returns command with that token removed and surrounding spaces joined.
// Finding more than one stderr redirect returns ok=false. Any other redirect
// (`>`, `>>`, `<`, `1>&2`, `&>/dev/null`, `2>file`) is left in place so the
// safe-byte gate and lexer refuse it.
func extractStderrRedirect(command string) (core string, kind stderrRedirect, ok bool) {
	core, kind, _, _, ok = extractStderrRedirectSpan(command)
	return core, kind, ok
}

func extractStderrRedirectSpan(command string) (core string, kind stderrRedirect, spanStart, delta int, ok bool) {
	var quote byte
	spanStart, spanEnd := -1, -1
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
		if i > 0 && command[i-1] != ' ' {
			continue
		}
		end, found := matchStderrRedirect(command, i)
		if found == redirNone {
			continue
		}
		if kind != redirNone {
			return "", redirNone, -1, 0, false
		}
		kind, spanStart, spanEnd = found, i, end
		i = end - 1
	}
	if kind == redirNone {
		return command, redirNone, -1, 0, true
	}
	left := strings.TrimRight(command[:spanStart], " ")
	right := strings.TrimLeft(command[spanEnd:], " ")
	switch {
	case left == "":
		core = right
	case right == "":
		core = left
	default:
		core = left + " " + right
	}
	return core, kind, len(left), len(command) - len(core), true
}

func matchStderrRedirect(command string, i int) (end int, kind stderrRedirect) {
	rest := command[i:]
	if strings.HasPrefix(rest, "2>&1") && wordEnds(command, i+4) {
		return i + 4, redirMerge
	}
	if !strings.HasPrefix(rest, "2>") {
		return 0, redirNone
	}
	j := i + 2
	for j < len(command) && command[j] == ' ' {
		j++
	}
	const devNull = "/dev/null"
	if strings.HasPrefix(command[j:], devNull) && wordEnds(command, j+len(devNull)) {
		return j + len(devNull), redirDiscard
	}
	return 0, redirNone
}

func wordEnds(s string, i int) bool {
	return i == len(s) || s[i] == ' '
}
