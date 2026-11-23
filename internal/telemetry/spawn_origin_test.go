// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import "testing"

func TestOriginOf(t *testing.T) {
	cases := []struct {
		name                        string
		onSession, below, rewritten bool
		want                        SpawnOrigin
	}{
		{"applied rewrite below session", true, true, true, OriginDownshiftRewrite},
		{"refuted rewrite falls to orchestrator below", true, true, false, OriginOrchestratorBelow},
		{"rewrite that did not end below stays session", true, false, true, OriginOrchestratorSession},
		{"on session", true, false, false, OriginOrchestratorSession},
		{"no session model", false, false, false, OriginUnattributed},
		{"rewrite but no session model", false, false, true, OriginUnattributed},
	}
	for _, c := range cases {
		if got := OriginOf(c.onSession, c.below, c.rewritten); got != c.want {
			t.Error(c.name, "got", got, "want", c.want)
		}
	}
}
