// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import "bytes"

// screenText returns the text that format detection and compaction read, or
// the reason the raw bytes are the only faithful answer.
//
// Test runners and git hooks print SGR colour sequences (ESC [ ... m) even
// when stdout is not a terminal: vitest does, and so do the lefthook lines a
// git commit or push prints. When colour is the only escape sequence present,
// the text is the input without it, so a coloured clean run compacts like a
// plain one and its summary is plain text. Any other escape (cursor movement,
// line erase, a hyperlink) means the bytes are a terminal rendering, not
// text, and stay raw.
//
// Single-line output with no newline at all may be an interactive prompt or
// cut-off fragment and stays raw. Multi-line output whose only missing newline
// is at EOF is normalized with a trailing newline so line-oriented compressors
// can read the final line cleanly. CRLF output would be rewritten with stray
// carriage returns and stays raw.
// The Flutter and Dart reporters rewrite one status line with carriage
// returns even without a TTY; compressFlutterTest validates them exactly.
func screenText(input []byte) (text []byte, reason string) {
	text = input
	if bytes.IndexByte(input, 0x1b) != -1 {
		plain, ok := stripColour(input)
		if !ok {
			return nil, "terminal_escape_preserved"
		}
		text = plain
	}
	if len(text) == 0 {
		return nil, "no_final_newline_preserved"
	}
	if text[len(text)-1] != '\n' {
		if bytes.IndexByte(text, '\n') == -1 {
			return nil, "no_final_newline_preserved"
		}
		norm := make([]byte, len(text)+1)
		copy(norm, text)
		norm[len(text)] = '\n'
		text = norm
	}
	if bytes.IndexByte(text, '\r') != -1 && !isFlutterTest(text) {
		return nil, "carriage_return_preserved"
	}
	return text, ""
}

// stripColour removes every SGR sequence: ESC, '[', parameter bytes made of
// digits, ';' and ':', then 'm'. It reports false when any other escape
// sequence is present.
func stripColour(input []byte) ([]byte, bool) {
	out := make([]byte, 0, len(input))
	for i := 0; i < len(input); i++ {
		if input[i] != 0x1b {
			out = append(out, input[i])
			continue
		}
		j := i + 1
		if j >= len(input) || input[j] != '[' {
			return nil, false
		}
		for j++; j < len(input) && (input[j] >= '0' && input[j] <= '9' || input[j] == ';' || input[j] == ':'); j++ {
		}
		if j >= len(input) || input[j] != 'm' {
			return nil, false
		}
		i = j
	}
	return out, true
}
