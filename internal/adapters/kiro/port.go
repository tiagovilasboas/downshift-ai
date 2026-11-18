// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package kiro

import (
	"github.com/tiagovilasboas/downshift-ai/internal/core"
	"github.com/tiagovilasboas/downshift-ai/internal/hookport"
)

// Port returns the observation port for Kiro IDE post-spawn events.
func Port() hookport.Port {
	return hookport.Port{
		ID: harnessID,
		Honor: func(raw []byte, binaryVersion string, r core.Resolver) (any, string) {
			stdout, note, _ := HandlePostToolUse(raw, binaryVersion, r)
			return stdout, note
		},
		ParseChildEnd: ParseChildEnd,
	}
}
