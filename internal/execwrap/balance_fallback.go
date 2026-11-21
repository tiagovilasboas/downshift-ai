// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import "github.com/tiagovilasboas/downshift-ai/internal/compressor"

// DetailFallbackCut marks a run whose family compressor passed (unrecognized
// output, or a failed run it did not fold) and whose output the byte cut of
// fallbackCut shortened instead. It is a bounded code, never output text.
const DetailFallbackCut = "fallback_cut"

// fallbackCut is the last resort for a family that has a compressor which let
// the output through whole: the same balanced cut unclassified output gets,
// stored in CCR first so every dropped line can be retrieved. A failed run
// keeps its failure lines and a long tail (compressor.BalanceFailed); a clean
// one takes the shorter of the line cut and the byte cut. It reports false,
// and the caller prints the output whole, when there is no store, the family
// is never cut with loss or stored, the output is within the threshold, a
// diff carries a danger word (its hunks are read verbatim), or the cut saves
// less than a quarter. A panic counts as false.
func fallbackCut(keep func([]byte) (string, bool), family Family, argv []string, exitCode int, raw []byte, detail string, cfg balanceConfig) (res []byte, ok bool) {
	defer func() {
		if recover() != nil {
			res, ok = nil, false
		}
	}()
	if keep == nil || family == FamilyNone || balanceRefused[family] || rawStoreRefused[family] || len(raw) <= cfg.limit() {
		return nil, false
	}
	kind := balanceKindOf(family, argv)
	if kind == compressor.BalanceDiff && detail == DetailDangerWord {
		return nil, false
	}
	if isListing(family, argv) && (exitCode != 0 || argvWithinLimit(argv, raw)) {
		return nil, false // a failed listing, or lines the agent sized itself
	}
	var cut []byte
	if exitCode != 0 {
		cut, ok = compressor.BalanceFailed(raw)
	} else {
		cut, ok = compressor.BalanceBytesScaled(raw)
		if lc, lok := compressor.BalanceWith(kind, raw, cfg.limits); lok && (!ok || len(lc) < len(cut)) {
			cut, ok = lc, true
		}
	}
	if !ok {
		return nil, false
	}
	id, stored := keep(append([]byte(nil), raw...))
	if !stored {
		return nil, false
	}
	return append(append([]byte{}, cut...), MarkerWithID(len(raw), len(cut), id)...), true
}
