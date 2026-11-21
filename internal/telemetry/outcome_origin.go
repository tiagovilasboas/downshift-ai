// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

// SmallSampleN is the task count below which an origin row is marked small.
const SmallSampleN = 30

// OriginOutcome is the outcome matrix folded to one spawn origin, across tier
// and agent type. Observational: origins are not randomized.
type OriginOutcome struct {
	Origin   SpawnOrigin
	N        int
	Outcomes OutcomeCounts
}

// Accepted is accepted_checked plus accepted_unchecked.
func (o OriginOutcome) Accepted() int {
	return o.Outcomes.AcceptedChecked + o.Outcomes.AcceptedUnchecked
}

// CheckedShare is accepted_checked over accepted; ok is false with no accepted task.
func (o OriginOutcome) CheckedShare() (share float64, ok bool) {
	if a := o.Accepted(); a > 0 {
		return float64(o.Outcomes.AcceptedChecked) / float64(a), true
	}
	return 0, false
}

// ReworkedOrErrored is reworked plus errored.
func (o OriginOutcome) ReworkedOrErrored() int {
	return o.Outcomes.Reworked + o.Outcomes.Errored
}

// SmallSample reports whether the row has fewer than SmallSampleN tasks.
func (o OriginOutcome) SmallSample() bool { return o.N < SmallSampleN }

var originOrder = []SpawnOrigin{
	OriginDownshiftRewrite, OriginOrchestratorBelow, OriginOrchestratorSession, OriginUnattributed,
}

// OutcomesByOrigin sums the matrix rows per origin, in a fixed origin order,
// leaving out origins with no task.
func OutcomesByOrigin(rows []OutcomeMatrixRow) []OriginOutcome {
	by := map[SpawnOrigin]*OriginOutcome{}
	for _, r := range rows {
		o := by[r.Origin]
		if o == nil {
			o = &OriginOutcome{Origin: r.Origin}
			by[r.Origin] = o
		}
		o.N += r.N
		c := &o.Outcomes
		c.Errored += r.Outcomes.Errored
		c.Reworked += r.Outcomes.Reworked
		c.CheckFailed += r.Outcomes.CheckFailed
		c.AcceptedChecked += r.Outcomes.AcceptedChecked
		c.AcceptedUnchecked += r.Outcomes.AcceptedUnchecked
		c.Unknown += r.Outcomes.Unknown
	}
	var out []OriginOutcome
	for _, k := range originOrder {
		if o := by[k]; o != nil && o.N > 0 {
			out = append(out, *o)
		}
	}
	return out
}
