// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package debugreverse

// Reverse returns s reversed rune-by-rune.
// BUG: the current implementation reverses raw bytes, corrupting multi-byte UTF-8.
func Reverse(s string) string {
	b := []byte(s)
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}
	return string(b)
}
