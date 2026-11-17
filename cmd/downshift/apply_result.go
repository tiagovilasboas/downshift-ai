// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"slices"
	"strings"

	"github.com/tiagovilasboas/downshift-ai/internal/core"
	"github.com/tiagovilasboas/downshift-ai/internal/routing/contract"
)

func applyResult(in ApplyInput, d core.Decision, note contract.Note, known, policyOK bool) ApplyResult {
	res := ApplyResult{
		Harness: in.Harness, RequestedModel: in.RequestedModel,
		SelectedModel: in.RequestedModel, Effort: in.Effort,
		Tier: string(note.TierTo), Verdict: string(note.Verdict),
		Note: noteMap(note),
	}
	moved := (d.Verdict == core.VerdictDownshift || d.Verdict == core.VerdictUpshift) &&
		d.Model.ID != "" && d.Model.ID != in.RequestedModel
	if moved {
		res.SelectedModel, res.Effort, res.Applied = d.Model.ID, d.Effort.String(), true
	}
	res.Reason = applyReason(note, moved, known, in.RequestedModel != "", policyOK)
	return res
}

// applyReason is one sentence built from enums and declared ids only.
func applyReason(n contract.Note, moved, known, named, policyOK bool) string {
	switch {
	case moved && n.RuleID != "":
		return "declared rule " + string(n.RuleID) + " applied to the requested model"
	case moved && n.Profile != "":
		return "declared profile " + string(n.Profile) + " applied to the requested model"
	case moved && n.Verdict == contract.VerdictHintDown:
		return "tier hint applied: moved down from the requested model"
	case moved && n.Verdict == contract.VerdictClass:
		return "task class applied: a decisive trivial or simple task moved down to small"
	case moved && n.Verdict == contract.VerdictClassUp:
		return "routing.raise_complex applied: a decisive complex task moved up from small to mid"
	case moved:
		return "declared policy applied (" + string(n.Verdict) + ")"
	case !named:
		return "no requested model: nothing to apply"
	case !policyOK:
		return "the project policy could not be read or is invalid: the requested model is kept (doubt keeps)"
	case !known:
		return "requested model is not in the catalog for this harness: kept (doubt keeps)"
	case n.Reason != "":
		return "requested model kept: " + string(n.Reason)
	default:
		return "requested model kept: no declared policy or hint moved it"
	}
}

// noteMap turns the note into a map through its own JSON tags, so a field
// added to the note reaches the surface without a second list to maintain.
func noteMap(n contract.Note) map[string]any {
	raw, err := json.Marshal(n)
	m := map[string]any{}
	if err == nil {
		_ = json.Unmarshal(raw, &m)
	}
	return m
}

// effortNames are the reasoning-effort names the catalog uses.
var effortNames = []string{"low", "medium", "high", "xhigh", "max"}

// ApplyInputError is a refusal of the input itself. Its message is a fixed
// sentence that never repeats the value it refuses.
type ApplyInputError struct{ Msg string }

func (e *ApplyInputError) Error() string { return e.Msg }

// validateApplyInput refuses a tier hint or effort that names no known value,
// the same for the command and the MCP tool.
func validateApplyInput(in ApplyInput) error {
	if in.TierHint != "" && !contract.Tier(in.TierHint).Valid() {
		return &ApplyInputError{Msg: "tier must be one of: small, mid, frontier"}
	}
	if in.Effort != "" && !slices.Contains(effortNames, in.Effort) {
		return &ApplyInputError{Msg: "effort must be one of: " + strings.Join(effortNames, ", ")}
	}
	return nil
}
