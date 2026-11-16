// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

// detectFormatExpected is the detection seam for CompressExitExpected. It
// returns exactly what detectFormat returns, whatever the caller expects.
//
// A fast path that tried the expected detectors first changed precedence: a
// later detector in detectFormat's order would claim output an earlier one
// owns (a go test run read as gh_run_log, jest as vitest, a gh issue view as
// gh_pr), so a family compacted output the full scan kept raw. The corpus
// parity test (detect_expected_corpus_test.go) found those divergences. Only
// running every earlier detector makes a match trustworthy, and that is the
// full scan, which already stops at its first match; so there is no cheaper
// correct order. The expected list stays in the API for the caller's
// acceptance check (execwrap formatAccepted).
func detectFormatExpected(input []byte, expected []Format) (Format, bool) {
	_ = expected
	return detectFormat(input)
}
