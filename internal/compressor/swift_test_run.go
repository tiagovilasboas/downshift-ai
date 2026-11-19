// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"github.com/tiagovilasboas/downshift-ai/internal/lazyre"
)

// Swift test output patterns covering both XCTest (Darwin/Linux) and Swift Testing.
var (
	reSwiftSuiteHeader  = lazyre.New(`(?m)^Test Suite '[^']+' (?:started|passed|failed) at `)
	reSwiftCaseHeader   = lazyre.New(`(?m)^Test Case '[^']+' (?:started(?:\.| at )|(?:passed|failed|skipped) \()`)
	reSwiftTestingStart = lazyre.New(`(?m)^◇ Test run started\.$`)
	reSwiftTestingEnd   = lazyre.New(`(?m)^[✔✘] Test run with \d+ tests?(?: in \d+ suites?)? (?:passed|failed) after \S+ seconds\.$`)

	reSwiftTestFailure = lazyre.New(`(?m)(?:^Test Suite '[^']+' failed at |^Test Case '[^']+' failed \(|^\s*Executed \d+ tests?, with (?:\d+ tests? skipped and )?[1-9]\d* failures?|^\s*✘ |recorded an issue at |Expectation failed:|XCTAssert|:\d+(?::\d+)?: (?:error|warning):|(?i)\b(?:fatal error|error:|warning:|panic:|exception)\b)`)

	reSwiftBuildStep    = lazyre.New(`^(?:\[\d+/\d+\] (?:Planning build|Write .+|Compiling .+|Emitting module .+|Linking .+|Copying .+|Generating .+|Merging .+|Wrapping .+|Applying .+)|Building for (?:debugging|release)\.\.\.|Build(?: of product '[^']+')? complete! \(\S+\))$`)
	reSwiftXCSuiteStart = lazyre.New(`^Test Suite '[^']+' started at .+$`)
	reSwiftXCSuitePass  = lazyre.New(`^Test Suite '([^']+)' passed at .+$`)
	reSwiftXCCaseStart  = lazyre.New(`^Test Case '[^']+' started(?:\.| at .+)?$`)
	reSwiftXCCasePass   = lazyre.New(`^Test Case '[^']+' passed \(\S+ seconds\)\.?$`)
	reSwiftXCCaseSkip   = lazyre.New(`^Test Case '[^']+' skipped \(\S+ seconds\)\.?$`)
	reSwiftXCSummary    = lazyre.New(`^\s*Executed (\d+) tests?, with (?:(\d+) tests? skipped and )?0 failures? \(0 unexpected\) in (\S+)(?: \((\S+)\))? seconds\.?$`)

	reSwiftSTLine    = lazyre.New(`^(?:◇ Test run started\.|↳ (?:Testing Library Version|Target Platform): .+|◇ (?:Suite|Test) .+ started\.|✔ (?:Suite|Test) .+ passed after \S+ seconds(?: with \d+ test cases?)?\.)$`)
	reSwiftSTSummary = lazyre.New(`^✔ Test run with (\d+) tests?(?: in \d+ suites?)? passed after (\S+) seconds\.$`)

	reSwiftDangerWord = lazyre.New(`(?i)error|fail|warning|panic|fatal|exception`)
)

// isSwiftTest reports whether input carries an unambiguous swift test anchor.
func isSwiftTest(input []byte) bool {
	return reSwiftSuiteHeader.Match(input) || reSwiftCaseHeader.Match(input) ||
		reSwiftTestingStart.Match(input) || reSwiftTestingEnd.Match(input)
}

type swiftTestTotals struct {
	xcAllDone, xcSuiteDone, stDone bool
	xcExec, xcSkip                 int
	suiteExec, suiteSkip           int
	stPassed                       int
	xcDurStr, suiteDurStr, stDur   string
	xcDur, suiteDur, stDurVal      float64
	pendingSuite                   string
}

func (s *swiftTestTotals) acceptLine(line string) bool {
	if line == "" {
		return true
	}
	if m := reSwiftXCSummary.FindStringSubmatch(line); m != nil {
		return s.acceptXCSummary(m)
	}
	// Every other line rejects any danger word so injected errors or warnings
	// stay raw.
	if reSwiftDangerWord.MatchString(line) || s.pendingSuite != "" {
		return false
	}
	if m := reSwiftXCSuitePass.FindStringSubmatch(line); m != nil {
		if m[1] == "All tests" && s.xcAllDone {
			return false
		}
		s.pendingSuite = m[1]
		return true
	}
	if m := reSwiftSTSummary.FindStringSubmatch(line); m != nil {
		if s.stDone {
			return false
		}
		p, err := strconv.Atoi(m[1])
		d, derr := strconv.ParseFloat(m[2], 64)
		if err != nil || derr != nil {
			return false
		}
		s.stDone, s.stPassed, s.stDur, s.stDurVal = true, p, m[2], d
		return true
	}
	// Reject further XCTest lines once Swift Testing has concluded.
	if s.stDone {
		return false
	}
	return reSwiftBuildStep.MatchString(line) || reSwiftXCSuiteStart.MatchString(line) ||
		reSwiftXCCaseStart.MatchString(line) || reSwiftXCCasePass.MatchString(line) ||
		reSwiftXCCaseSkip.MatchString(line) || reSwiftSTLine.MatchString(line)
}

func (s *swiftTestTotals) acceptXCSummary(m []string) bool {
	if s.pendingSuite == "" {
		return false
	}
	execN, err := strconv.Atoi(m[1])
	if err != nil {
		return false
	}
	skipN := 0
	if m[2] != "" {
		if skipN, err = strconv.Atoi(m[2]); err != nil || skipN > execN {
			return false
		}
	}
	durStr := m[3]
	if m[4] != "" {
		durStr = m[4]
	}
	dur, err := strconv.ParseFloat(durStr, 64)
	if err != nil {
		return false
	}
	if s.pendingSuite == "All tests" {
		s.xcAllDone, s.xcExec, s.xcSkip, s.xcDurStr, s.xcDur = true, execN, skipN, durStr, dur
	} else {
		s.xcSuiteDone, s.suiteExec, s.suiteSkip, s.suiteDurStr, s.suiteDur = true, execN, skipN, durStr, dur
	}
	s.pendingSuite = ""
	return true
}

func (s *swiftTestTotals) summary() (passed, skipped int, dur string, ok bool) {
	if s.pendingSuite != "" || (!s.xcAllDone && !s.xcSuiteDone && !s.stDone) {
		return 0, 0, "", false
	}
	xcExec, xcSkip, xcDurStr, xcDur := s.suiteExec, s.suiteSkip, s.suiteDurStr, s.suiteDur
	hasXC := s.xcSuiteDone
	if s.xcAllDone {
		xcExec, xcSkip, xcDurStr, xcDur, hasXC = s.xcExec, s.xcSkip, s.xcDurStr, s.xcDur, true
	}
	passed = (xcExec - xcSkip) + s.stPassed
	skipped = xcSkip
	if passed <= 0 && skipped == 0 {
		return 0, 0, "", false
	}
	switch {
	case hasXC && s.stDone:
		if xcExec == 0 {
			dur = s.stDur + "s"
		} else {
			dur = fmt.Sprintf("%.3fs", xcDur+s.stDurVal)
		}
	case hasXC:
		dur = xcDurStr + "s"
	default:
		dur = s.stDur + "s"
	}
	return passed, skipped, dur, true
}

// compressSwiftTest compacts clean swift test output (XCTest, Swift Testing,
// or both) to a single PASS summary line. Failures, warnings, unrecognised
// lines, and truncated bytes are returned unchanged.
func compressSwiftTest(input []byte) ([]byte, bool, string) {
	if len(input) == 0 || input[len(input)-1] != '\n' ||
		bytes.IndexByte(input, '\r') != -1 || bytes.IndexByte(input, 0x1b) != -1 {
		return input, false, "swift_test_incomplete_bytes_preserved"
	}
	if reSwiftTestFailure.Match(input) {
		return input, false, "swift_test_failure_preserved"
	}
	var totals swiftTestTotals
	lines := strings.Split(strings.TrimSuffix(string(input), "\n"), "\n")
	for _, line := range lines {
		if !totals.acceptLine(line) {
			return input, false, "swift_test_unrecognised_line_preserved"
		}
	}
	passed, skipped, dur, ok := totals.summary()
	if !ok {
		return input, false, "swift_test_missing_summary_preserved"
	}
	var out string
	if skipped > 0 {
		out = fmt.Sprintf("Swift test: PASS (%d passed, %d skipped in %s)\n", passed, skipped, dur)
	} else {
		out = fmt.Sprintf("Swift test: PASS (%d passed in %s)\n", passed, dur)
	}
	if len(out) >= len(input) {
		return input, false, "no_compression_gain"
	}
	return []byte(out), true, fmt.Sprintf("compressed swift test summary (%d passed in %s)", passed, dur)
}
