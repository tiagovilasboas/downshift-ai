// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package readout

import (
	"github.com/tiagovilasboas/downshift-ai/internal/telemetry"
	"strings"
	"testing"
	"time"
)

func TestReasonWords_EveryCodeHasWords(t *testing.T) {
	codes := []string{
		telemetry.ReasonClassifierConfident, telemetry.ReasonClassifierUnconfident, telemetry.ReasonNoSignal, telemetry.ReasonSemanticBoost, telemetry.ReasonTaskFocus, telemetry.ReasonUnsupportedLanguage,
		telemetry.ReasonRiskCategory, telemetry.ReasonRiskFloor, telemetry.ReasonNBUpshiftApplied,
		telemetry.ReasonNBDownshiftApplied, telemetry.ReasonTierFloor, telemetry.ReasonCreditHeld, telemetry.ReasonGuessHeld, telemetry.ReasonPaused, telemetry.ReasonBaselineFromSettings, telemetry.ReasonAgentPinsModel, telemetry.ReasonAgentTypeHint, telemetry.ReasonAgentDefaultTier, telemetry.ReasonAboveAgentDefault, telemetry.ReasonHeldUnmapped,
	}
	codes = append(codes, telemetry.ReasonHeldUnconfidentDownshift, telemetry.ReasonHeldComplexDownshift, telemetry.ReasonHeldForeignModel, telemetry.ReasonHeldRiskDownshift, telemetry.ReasonHeldUnconfidentUnknownSmall, telemetry.ReasonHeldUnconfidentUnknownFrontier, telemetry.ReasonHeldUnconfidentUpshift, telemetry.ReasonHeldUnconfidentUnknownMid)
	for _, c := range codes {
		if _, ok := reasonWords[c]; !ok {
			t.Errorf("reason code %s has no plain-words entry", c)
		}
	}
	if ReasonWords("SOMETHING_NEW") == "" {
		t.Error("an unknown code still gets a sentence")
	}
}

func TestWhy_HeldDecision(t *testing.T) {
	d := td("d1", sessA, 0, "claude-opus-5-5", telemetry.ReasonClassifierUnconfident, "HELD_UNCONFIDENT_DOWNSHIFT")
	d.Outcome = telemetry.OutcomeAllow
	d.Corrections = []string{"R1_UNCONFIDENT_DOWNSHIFT"}
	d.SafeVerdict = "OK"
	out := Why([]telemetry.Event{d}, sessA)
	for _, want := range []string{
		"subagent 1", "asked for   opus", "ran on      opus", "DOWNSHIFT, held to OK (mid, MEDIUM)",
		"held: spawn left unchanged", "what held it", "HELD_UNCONFIDENT_DOWNSHIFT", "not confident enough to downshift",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if n := len(lines(out)); n > 20 {
		t.Errorf("must fit one screen, got %d lines", n)
	}
}

func TestWhy_LiftedDecisionNamesTheLift(t *testing.T) {
	d := td("d1", sessA, 0, "claude-opus-5-5", telemetry.ReasonClassifierConfident, telemetry.ReasonSemanticBoost, telemetry.ReasonRiskFloor)
	d.Verdict, d.Tier, d.FromModel = "UPSHIFT", "frontier", "claude-sonnet-5-5"
	out := Why([]telemetry.Event{d}, sessA)
	if !strings.Contains(out, "model rewritten to opus") || !strings.Contains(out, "SEMANTIC_BOOST") ||
		!strings.Contains(out, "no guardrail held this decision") {
		t.Fatalf("lifted decision:\n%s", out)
	}
}

func TestWhy_PausedAndLatestDecisionWins(t *testing.T) {
	first := td("d1", sessA, 0, "claude-haiku-5-5", telemetry.ReasonClassifierConfident)
	paused := td("d2", sessA, time.Minute, "claude-opus-5-5", telemetry.ReasonClassifierConfident, telemetry.ReasonPaused)
	paused.Outcome = telemetry.OutcomeBaseline
	res := telemetry.Event{CorrelationID: "r2", Timestamp: at(2 * time.Minute), SessionID: sessA, Outcome: telemetry.OutcomeResolved, LinkedDecision: "d2", ToModel: "claude-opus-5-5"}
	out := Why([]telemetry.Event{first, paused, res}, sessA)
	if !strings.Contains(out, "subagent 2") || !strings.Contains(out, "paused: spawn left unchanged") || !strings.Contains(out, "PAUSED") {
		t.Fatalf("paused:\n%s", out)
	}
}

func TestWhy_OldEventsAndNoDecisions(t *testing.T) {
	old := telemetry.Event{CorrelationID: "d1", Timestamp: at(0), SessionID: sessA, Harness: "claude-code", Outcome: telemetry.OutcomeAllow, Verdict: "OK", Tier: "mid", FromModel: "sonnet", ToModel: "sonnet"}
	if out := Why([]telemetry.Event{old}, sessA); !strings.Contains(out, "predates reason codes") {
		t.Fatalf("old event:\n%s", out)
	}
	if out := Why(nil, sessA); !strings.Contains(out, "no subagent decision") {
		t.Fatalf("empty:\n%s", out)
	}
}
