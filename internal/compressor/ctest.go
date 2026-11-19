// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/tiagovilasboas/downshift-ai/internal/lazyre"
)

var (
	reCTestProject         = lazyre.New(`(?m)^Test project \S`)
	reCTestChangingDir     = lazyre.New(`^Internal ctest changing into directory: \S.*$`)
	reCTestProjectLine     = lazyre.New(`^Test project \S.*$`)
	reCTestStartLine       = lazyre.New(`^\s*Start\s+\d+:\s+\S.*$`)
	reCTestTestLine        = lazyre.New(`(?m)^\s*\d+/\d+\s+Test\s+#\d+:\s+`)
	reCTestPassLine        = lazyre.New(`^\s*(\d+)/(\d+)\s+Test\s+#\d+:\s+\S.*?(?:\s+\.+)?\s+Passed\s+\d+(?:\.\d+)?\s+sec$`)
	reCTestSummary         = lazyre.New(`(?m)^(\d+)% tests passed, (\d+) tests? failed out of (\d+)$`)
	reCTestPassSummaryLine = lazyre.New(`^100% tests passed, 0 tests? failed out of \d+$`)
	reCTestLabelHeader     = lazyre.New(`^Label Time Summary:$`)
	reCTestLabelLine       = lazyre.New(`^\S+(?:\s+\S+)*\s+=\s+\d+(?:\.\d+)?\s+sec\*proc\s+\(\d+\s+tests?\)$`)
	reCTestTotalTime       = lazyre.New(`^Total Test time \(real\)\s*=\s*(\d+(?:\.\d+)?)\s+sec$`)
	reCTestFailHeader      = lazyre.New(`(?m)^(?:The following tests FAILED:|Errors while running CTest)$`)
	reCTestFailure         = lazyre.New(`(?m)(?:\*\*\*(?:Failed|Timeout|Exception|Not Run|Skipped)|^The following tests (?:FAILED|did not run):|^Errors while running CTest|^No tests were found|^\d+% tests passed, [1-9]\d* tests? failed|^(?:[0-9]|[1-9][0-9])% tests passed,)`)
)

// isCTest reports whether input looks like a CMake CTest transcript.
func isCTest(input []byte) bool {
	return reCTestSummary.Match(input) ||
		(reCTestProject.Match(input) && reCTestTestLine.Match(input)) ||
		reCTestFailHeader.Match(input)
}

// compressCTest compacts only a complete, unambiguous, all-passing CTest run
// to a single summary line. Any failing, skipped, incomplete, or unrecognised
// output is returned unchanged.
func compressCTest(input []byte) ([]byte, bool, string) {
	if len(input) == 0 || input[len(input)-1] != '\n' {
		return input, false, "ctest_incomplete_bytes_preserved"
	}
	if reCTestFailure.Match(input) || linesHaveDangerWord(input, reCTestPassSummaryLine) {
		return input, false, "ctest_failure_or_danger_preserved"
	}
	total, duration, reason := parseCTestRun(string(input))
	if reason != "" {
		return input, false, reason
	}
	out := fmt.Sprintf("CTest: PASS (%d passed in %ss)\n", total, duration)
	if len(out) >= len(input) {
		return input, false, "no_compression_gain"
	}
	return []byte(out), true, fmt.Sprintf("compressed ctest summary (%d passed in %ss)", total, duration)
}

// parseCTestRun validates every line against the clean CTest grammar and
// returns the total test count and wall-clock duration.
func parseCTestRun(text string) (int, string, string) {
	var (
		changingDirs     int
		headers          int
		passedLines      int
		expectedTotal    int
		summaryCount     int
		summaryTotal     int
		totalTime        string
		seenLabelSummary bool
		inLabelSummary   bool
	)
	for _, line := range strings.Split(strings.TrimSuffix(text, "\n"), "\n") {
		if line == "" {
			inLabelSummary = false
			continue
		}
		if summaryCount == 0 {
			switch {
			case reCTestChangingDir.MatchString(line):
				if headers > 0 || changingDirs > 0 || passedLines > 0 {
					return 0, "", "ctest_unrecognised_line_preserved"
				}
				changingDirs++
			case reCTestProjectLine.MatchString(line):
				if headers > 0 || passedLines > 0 {
					return 0, "", "ctest_multiple_runs_preserved"
				}
				headers++
			case reCTestStartLine.MatchString(line):
			case reCTestPassLine.MatchString(line):
				m := reCTestPassLine.FindStringSubmatch(line)
				total, err := strconv.Atoi(m[2])
				if err != nil || total <= 0 {
					return 0, "", "ctest_unrecognised_line_preserved"
				}
				if expectedTotal == 0 {
					expectedTotal = total
				} else if expectedTotal != total {
					return 0, "", "ctest_inconsistent_counts_preserved"
				}
				passedLines++
			case reCTestPassSummaryLine.MatchString(line):
				m := reCTestSummary.FindStringSubmatch(line)
				if m == nil {
					return 0, "", "ctest_unrecognised_line_preserved"
				}
				total, err := strconv.Atoi(m[3])
				if err != nil || total <= 0 {
					return 0, "", "ctest_unrecognised_line_preserved"
				}
				summaryCount++
				summaryTotal = total
			default:
				return 0, "", "ctest_unrecognised_line_preserved"
			}
			continue
		}
		switch {
		case totalTime == "" && !seenLabelSummary && reCTestLabelHeader.MatchString(line):
			seenLabelSummary = true
			inLabelSummary = true
		case inLabelSummary && reCTestLabelLine.MatchString(line):
		case totalTime == "" && reCTestTotalTime.MatchString(line):
			inLabelSummary = false
			totalTime = reCTestTotalTime.FindStringSubmatch(line)[1]
		default:
			return 0, "", "ctest_unrecognised_line_preserved"
		}
	}
	if summaryCount != 1 || totalTime == "" || summaryTotal <= 0 ||
		passedLines != summaryTotal || expectedTotal != summaryTotal {
		return 0, "", "ctest_incomplete_or_inconsistent_preserved"
	}
	return summaryTotal, totalTime, ""
}
