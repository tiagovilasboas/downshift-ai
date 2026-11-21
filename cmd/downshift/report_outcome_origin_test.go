// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/telemetry"
)

func TestOutcomeOriginMarkdown(t *testing.T) {
	m := []telemetry.OutcomeMatrixRow{
		{Origin: telemetry.OriginDownshiftRewrite, N: 40, Outcomes: telemetry.OutcomeCounts{AcceptedChecked: 20, AcceptedUnchecked: 10, Reworked: 5, Errored: 1, CheckFailed: 2, Unknown: 2}},
		{Origin: telemetry.OriginOrchestratorSession, N: 3, Outcomes: telemetry.OutcomeCounts{AcceptedUnchecked: 3}},
	}
	var b bytes.Buffer
	printOutcomeOriginMarkdown(&b, m)
	s := b.String()
	row := strings.Join(strings.Split("| downshift_rewrite;40;30;67%;2;6;2 |", ";"), " | ")
	wants := strings.Split("(observational);not randomized;orchestrator_session (small sample, n<30);"+row, ";")
	for _, want := range wants {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in:\n%s", want, s)
		}
	}
	b.Reset()
	printOutcomeOriginMarkdown(&b, nil)
	printOutcomeOriginText(&b, nil)
	if b.Len() != 0 {
		t.Errorf("empty matrix must print nothing: %q", b.String())
	}
}
