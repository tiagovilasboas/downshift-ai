// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core_test

import (
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/core"
)

// Only the Claude Code adapter fills ChildEnd.Verification (from the child
// transcript); every other adapter leaves it empty.
func TestReportsChildVerificationPerHarness(t *testing.T) {
	want := map[string]bool{
		"claude-code": true, "cursor": false, "codex": false, "kiro": false,
		"kirocrew": false, "antigravity": false, "opencode": false,
	}
	for _, n := range core.AllHarnessCaps() {
		w, ok := want[n.Harness]
		if !ok {
			t.Fatalf("%s: add it to the verification table", n.Harness)
		}
		if n.Caps.ReportsChildVerification != w {
			t.Errorf("%s: ReportsChildVerification = %v, want %v", n.Harness, n.Caps.ReportsChildVerification, w)
		}
		if n.Caps.ReportsChildVerification && n.Caps.ChildEndEvent == "" {
			t.Errorf("%s: verification needs a child-end event", n.Harness)
		}
	}
}
