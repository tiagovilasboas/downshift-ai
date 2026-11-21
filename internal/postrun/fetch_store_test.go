// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package postrun

import (
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/core"
	"github.com/tiagovilasboas/downshift-ai/internal/hookport"
)

// A compound command led by curl, wget or glab may carry a credential or a
// private response: CCR must never store its raw output, and the output must
// stay raw rather than be cut without a store.
func TestCompactShell_FetchCompoundNeverStored(t *testing.T) {
	log := bigLog()
	for _, tc := range []struct {
		cmd       string
		wantStore bool
	}{
		{"curl https://example.test/x | head", false},
		{"curl -s https://example.test/x | head -n 500", false},
		{"wget -O- https://example.test/x | grep foo", false},
		{"glab mr view 482 | head", false},
		{"git log | head", true},
	} {
		writes := 0
		keep := func(raw []byte) (string, bool) { writes++; return "0123456789ab", true }
		in := hookport.ToolOutput{Category: core.CategoryShell, Command: tc.cmd, Stdout: log, HasStdout: true, ExitCode: -1}
		res := Compact(allCaps, in, Options{Keep: keep, Balanced: true})
		if tc.wantStore {
			if writes == 0 {
				t.Errorf("%q: expected a CCR store", tc.cmd)
			}
			continue
		}
		if writes != 0 || res.Replaced || res.Output.Stdout != log || res.KeptID != "" {
			t.Errorf("%q: writes=%d replaced=%v kept=%q, want raw with no store", tc.cmd, writes, res.Replaced, res.KeptID)
		}
	}
}
