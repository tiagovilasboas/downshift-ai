// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

// Byte budget of the cut of a failed run's output: a failure is read from its
// error lines and from the end, so the tail and the failure share are larger
// than the cut of a clean run.
const (
	failedHeadBytes = 1024
	failedTailBytes = 8192
	failedKeepBytes = 16384
)

// Share of the text the head and the tail may take, in percent. The failure
// lines are never scaled: when the cut then saves under a quarter, the gate
// leaves the output whole.
const (
	failedHeadPct = 10
	failedTailPct = 35
)

// BalanceFailed cuts the output of a run that exited non-zero: a short head,
// every failure line (with the lines right after it) up to a byte budget, and
// a long tail. Budgets scale down for text only a little over the threshold.
// It reports false when the cut would not save a quarter of the text; the
// caller must only use the result when the raw text is stored (retrievable).
func BalanceFailed(text []byte) ([]byte, bool) {
	n := len(text)
	return balanceBytes(text,
		min(failedHeadBytes, n*failedHeadPct/100),
		min(failedTailBytes, n*failedTailPct/100),
		failedKeepBytes) // never scaled: a dropped error line costs a rerun
}
