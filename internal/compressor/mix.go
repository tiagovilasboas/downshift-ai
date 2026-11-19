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

var (
	reMixTestRunning       = lazyre.New(`(?m)^Running ExUnit with seed: \d+, max_cases: \d+$`)
	reMixTestFinished      = lazyre.New(`(?m)^Finished in (?:(\d+) minutes?, )?(\d+(?:\.\d+)?) seconds?(?: \([^)]*\))?$`)
	reMixTestFinishedAsync = lazyre.New(`(?m)^Finished in (?:\d+ minutes?, )?\d+(?:\.\d+)? seconds? \([^)]*\basync\b[^)]*\)$`)
	reMixTestSummary       = lazyre.New(`(?m)^(?:(\d+) doctests?, )?(?:(\d+) propert(?:y|ies), )?(\d+) tests?, (\d+) failures?(?:, (\d+) excluded)?(?:, (\d+) skipped)?(?:, (\d+) invalid)?$`)
	reMixTestFailure       = lazyre.New(`(?m)(?:^\s*\d+\)\s+|^\s*\*\*\s*\(|(?:^|, )[1-9]\d*\s+(?:failures?|invalid)\b|\bAssertion\b|\bFAILED\b|(?i)warning:|(?i)error:)`)
	reMixTestDanger        = lazyre.New(`(?i)error|fail|warning|panic`)
	reMixTestKnownLine     = lazyre.New(`^(?:==> [A-Za-z0-9_.-]+|Compiling \d+ files? \([^)]+\)|Generated [A-Za-z0-9_.-]+ app|Running ExUnit with seed: \d+, max_cases: \d+|(?:Ex|In)cluding tags: \[[^\]]*\]|Randomized with seed \d+|[.*]+)$`)
)

type mixTestStats struct {
	doctests   int
	properties int
	tests      int
	excluded   int
	skipped    int
	seconds    float64
	timeRaw    string
	summaries  int
}

func isMixTest(input []byte) bool {
	return reMixTestRunning.Match(input) ||
		reMixTestSummary.Match(input) ||
		reMixTestFinishedAsync.Match(input)
}

// compressMixTest compacts clean `mix test` (ExUnit) output to a one-line
// summary. Failures, warnings, debug output and unrecognised shapes stay raw.
func compressMixTest(input []byte) ([]byte, bool, string) {
	if len(input) == 0 || input[len(input)-1] != '\n' || bytes.IndexByte(input, '\r') != -1 {
		return input, false, "mix_test_incomplete_bytes_preserved"
	}
	if reMixTestFailure.Match(input) {
		return input, false, "mix_test_failure_preserved"
	}
	stats, reason := parseMixTest(input)
	if reason != "" {
		return input, false, reason
	}
	out := formatMixTestSummary(stats)
	if len(out) >= len(input) {
		return input, false, "no_compression_gain"
	}
	return []byte(out), true, fmt.Sprintf("compressed mix test summary (%d tests)", stats.tests)
}

func parseMixTest(input []byte) (mixTestStats, string) {
	var stats mixTestStats
	sawFinished := false
	for _, line := range bytes.Split(input[:len(input)-1], []byte("\n")) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		if m := reMixTestFinished.FindSubmatch(line); m != nil {
			if sawFinished {
				return stats, "mix_test_unrecognised_line_preserved"
			}
			var mins int
			if len(m[1]) > 0 {
				mins, _ = strconv.Atoi(string(m[1]))
			}
			secs, err := strconv.ParseFloat(string(m[2]), 64)
			if err != nil {
				return stats, "mix_test_unrecognised_line_preserved"
			}
			stats.seconds += float64(mins*60) + secs
			if stats.summaries == 0 && len(m[1]) == 0 {
				stats.timeRaw = string(m[2]) + "s"
			}
			sawFinished = true
			continue
		}
		if m := reMixTestSummary.FindSubmatch(line); m != nil {
			if !sawFinished {
				return stats, "mix_test_duplicate_summary_preserved"
			}
			failures, _ := strconv.Atoi(string(m[4]))
			invalid, _ := strconv.Atoi(string(m[7]))
			if failures != 0 || invalid != 0 {
				return stats, "mix_test_failure_preserved"
			}
			doctests, _ := strconv.Atoi(string(m[1]))
			properties, _ := strconv.Atoi(string(m[2]))
			tests, _ := strconv.Atoi(string(m[3]))
			excluded, _ := strconv.Atoi(string(m[5]))
			skipped, _ := strconv.Atoi(string(m[6]))
			stats.doctests += doctests
			stats.properties += properties
			stats.tests += tests
			stats.excluded += excluded
			stats.skipped += skipped
			stats.summaries++
			sawFinished = false
			continue
		}
		if reMixTestDanger.Match(line) {
			return stats, "mix_test_danger_word_preserved"
		}
		if !reMixTestKnownLine.Match(line) {
			return stats, "mix_test_unrecognised_line_preserved"
		}
	}
	if sawFinished || stats.summaries == 0 {
		return stats, "mix_test_incomplete_summary_preserved"
	}
	if stats.tests+stats.doctests+stats.properties == 0 {
		return stats, "mix_test_zero_tests_preserved"
	}
	return stats, ""
}

func formatMixTestSummary(s mixTestStats) string {
	var parts []string
	if s.doctests > 0 {
		parts = append(parts, fmt.Sprintf("%d %s", s.doctests, pluralUnit(s.doctests, "doctest", "doctests")))
	}
	if s.properties > 0 {
		parts = append(parts, fmt.Sprintf("%d %s", s.properties, pluralUnit(s.properties, "property", "properties")))
	}
	parts = append(parts, fmt.Sprintf("%d %s", s.tests, pluralUnit(s.tests, "test", "tests")))
	parts = append(parts, "0 failures")
	if s.excluded > 0 {
		parts = append(parts, fmt.Sprintf("%d excluded", s.excluded))
	}
	if s.skipped > 0 {
		parts = append(parts, fmt.Sprintf("%d skipped", s.skipped))
	}
	dur := s.timeRaw
	if s.summaries > 1 || dur == "" {
		dur = fmt.Sprintf("%.2fs", s.seconds)
	}
	return fmt.Sprintf("Mix test: PASS (%s in %s)\n", strings.Join(parts, ", "), dur)
}

func pluralUnit(n int, singular, plural string) string {
	if n == 1 {
		return singular
	}
	return plural
}
