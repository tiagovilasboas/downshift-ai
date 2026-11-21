// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import "testing"

func TestOutcomesByOriginSumsAcrossTierAndAgent(t *testing.T) {
	rows := []OutcomeMatrixRow{
		{Origin: OriginUnattributed, N: 1, Outcomes: OutcomeCounts{Unknown: 1}},
		{Origin: OriginDownshiftRewrite, Tier: "small", AgentType: "a", N: 20,
			Outcomes: OutcomeCounts{AcceptedChecked: 10, AcceptedUnchecked: 5, Reworked: 3, Errored: 2}},
		{Origin: OriginDownshiftRewrite, Tier: "mid", AgentType: "b", N: 15,
			Outcomes: OutcomeCounts{AcceptedChecked: 5, CheckFailed: 4, Unknown: 6}},
	}
	got := OutcomesByOrigin(rows)
	if len(got) != 2 || got[0].Origin != OriginDownshiftRewrite || got[1].Origin != OriginUnattributed {
		t.Fatalf("order/origins wrong: %+v", got)
	}
	d := got[0]
	if d.N != 35 || d.Accepted() != 20 || d.ReworkedOrErrored() != 5 || d.Outcomes.CheckFailed != 4 || d.Outcomes.Unknown != 6 {
		t.Fatalf("sums wrong: %+v", d)
	}
	if s, ok := d.CheckedShare(); !ok || s != 0.75 {
		t.Fatalf("checked share = %v %v", s, ok)
	}
	if d.SmallSample() || !got[1].SmallSample() {
		t.Fatalf("small sample flags wrong")
	}
	if _, ok := got[1].CheckedShare(); ok {
		t.Fatal("no accepted task must give no share")
	}
}

func TestOutcomesByOriginEmpty(t *testing.T) {
	if got := OutcomesByOrigin(nil); len(got) != 0 {
		t.Fatalf("got %+v", got)
	}
}
