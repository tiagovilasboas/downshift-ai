// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

// IsLineFilter reports whether words are a `head` or `tail` stage with a short
// line count (`-N`, `-n N`, `-nN`), the same rule decideCompound applies
// before the run.
func IsLineFilter(words []string) bool { return lineFilter(words) }

// WithinRequestedLimit reports whether data holds no more lines than the head
// or tail that ends command asked for: those lines are the agent's choice, so
// no byte cut may drop one.
func WithinRequestedLimit(command string, data []byte) bool {
	if len(data) == 0 {
		return false
	}
	return withinRequestedLimit(command, data)
}
