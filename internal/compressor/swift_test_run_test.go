// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/compressor"
)

func TestCompressSwiftTest_PassXCTest(t *testing.T) {
	in := []byte(strings.Join([]string{
		"Test Suite 'All tests' started at 2026-11-19 10:00:00.000.",
		"Test Suite 'DemoPackageTests.xctest' started at 2026-11-19 10:00:00.002.",
		"Test Suite 'MathTests' started at 2026-11-19 10:00:00.003.",
		"Test Case '-[DemoTests.MathTests testAdd]' started.",
		"Test Case '-[DemoTests.MathTests testAdd]' passed (0.002 seconds).",
		"Test Case '-[DemoTests.MathTests testSub]' started.",
		"Test Case '-[DemoTests.MathTests testSub]' passed (0.001 seconds).",
		"Test Suite 'MathTests' passed at 2026-11-19 10:00:00.007.",
		"\t Executed 2 tests, with 0 failures (0 unexpected) in 0.003 (0.004) seconds",
		"Test Suite 'DemoPackageTests.xctest' passed at 2026-11-19 10:00:00.008.",
		"\t Executed 2 tests, with 0 failures (0 unexpected) in 0.003 (0.005) seconds",
		"Test Suite 'All tests' passed at 2026-11-19 10:00:00.009.",
		"\t Executed 2 tests, with 0 failures (0 unexpected) in 0.003 (0.009) seconds",
		"",
	}, "\n"))

	res := compressor.CompressExit(in, compressor.ModeSafe, 0)
	if !res.Applied || res.Format != compressor.FormatSwiftTest {
		t.Fatalf("expected swift_test compaction, got applied=%v format=%v reason=%q", res.Applied, res.Format, res.Reason)
	}
	want := "Swift test: PASS (2 passed in 0.009s)\n"
	if string(res.Output) != want {
		t.Fatalf("got %q, want %q", res.Output, want)
	}
	short, ok := compressor.ShortenPass(res.Output)
	if !ok || string(short) != "Swift test: PASS (2 passed)\n" {
		t.Fatalf("ShortenPass = %q (%v), want 'Swift test: PASS (2 passed)\\n'", short, ok)
	}
}

func TestCompressSwiftTest_PassLinuxAndSkippedAndBuild(t *testing.T) {
	in := []byte(strings.Join([]string{
		"Building for debugging...",
		"[1/3] Compiling DemoTests MathTests.swift",
		"[2/3] Emitting module DemoTests",
		"[3/3] Linking DemoPackageTests.xctest",
		"Build complete! (1.25s)",
		"Test Suite 'All tests' started at 2026-11-19 10:00:00.000",
		"Test Suite 'MathTests' started at 2026-11-19 10:00:00.001",
		"Test Case 'MathTests.testAdd' started at 2026-11-19 10:00:00.001",
		"Test Case 'MathTests.testAdd' passed (0.002 seconds)",
		"Test Case 'MathTests.testSkip' started at 2026-11-19 10:00:00.003",
		"Test Case 'MathTests.testSkip' skipped (0.001 seconds)",
		"Test Suite 'MathTests' passed at 2026-11-19 10:00:00.004",
		"\t Executed 2 tests, with 1 test skipped and 0 failures (0 unexpected) in 0.003 (0.004) seconds",
		"Test Suite 'All tests' passed at 2026-11-19 10:00:00.005",
		"\t Executed 2 tests, with 1 test skipped and 0 failures (0 unexpected) in 0.003 (0.005) seconds",
		"",
	}, "\n"))

	res := compressor.CompressExit(in, compressor.ModeSafe, 0)
	if !res.Applied || res.Format != compressor.FormatSwiftTest {
		t.Fatalf("expected swift_test compaction, got applied=%v format=%v reason=%q", res.Applied, res.Format, res.Reason)
	}
	want := "Swift test: PASS (1 passed, 1 skipped in 0.005s)\n"
	if string(res.Output) != want {
		t.Fatalf("got %q, want %q", res.Output, want)
	}
}

func TestCompressSwiftTest_PassSwiftTestingAndCombined(t *testing.T) {
	in := []byte(strings.Join([]string{
		"Test Suite 'All tests' started at 2026-11-19 10:46:38.130.",
		"Test Suite 'All tests' passed at 2026-11-19 10:46:38.133.",
		"\t Executed 0 tests, with 0 failures (0 unexpected) in 0.000 (0.003) seconds",
		"◇ Test run started.",
		"↳ Testing Library Version: 124",
		"↳ Target Platform: arm64e-apple-macos14.0",
		"◇ Suite \"LedgerTests\" started.",
		"◇ Test \"entryBalance()\" started.",
		"◇ Test \"reconcileBatch()\" started.",
		"✔ Test \"entryBalance()\" passed after 0.001 seconds.",
		"✔ Test \"reconcileBatch()\" passed after 0.002 seconds.",
		"✔ Suite \"LedgerTests\" passed after 0.003 seconds.",
		"✔ Test run with 2 tests in 1 suite passed after 0.014 seconds.",
		"",
	}, "\n"))

	res := compressor.CompressExit(in, compressor.ModeSafe, 0)
	if !res.Applied || res.Format != compressor.FormatSwiftTest {
		t.Fatalf("expected swift_test compaction, got applied=%v format=%v reason=%q", res.Applied, res.Format, res.Reason)
	}
	want := "Swift test: PASS (2 passed in 0.014s)\n"
	if string(res.Output) != want {
		t.Fatalf("got %q, want %q", res.Output, want)
	}
}

func TestCompressSwiftTest_FailsKeepEveryFailureLine(t *testing.T) {
	xcFail := readFixture(t, "swift_test_fail.txt")
	stFail := []byte(strings.Join([]string{
		"◇ Test run started.",
		"↳ Testing Library Version: 124",
		"◇ Test passOne() started.",
		"◇ Test failOne() started.",
		"✔ Test passOne() passed after 0.001 seconds.",
		"✘ Test failOne() recorded an issue at DemoTests.swift:10:5: Expectation failed: (1 + 1 → 2) == 3",
		"✘ Test failOne() failed after 0.001 seconds with 1 issue.",
		"✘ Test run with 2 tests failed after 0.001 seconds with 1 issue.",
		"",
	}, "\n"))

	for name, in := range map[string][]byte{
		"xctest":        xcFail,
		"swift_testing": stFail,
	} {
		t.Run(name, func(t *testing.T) {
			for _, code := range append([]int{-1, 0}, nonZeroExits...) {
				res := compressor.CompressExit(in, compressor.ModeSafe, code)
				if res.Applied || !bytes.Equal(res.Output, in) {
					t.Fatalf("exit=%d modified failing output: applied=%v reason=%q", code, res.Applied, res.Reason)
				}
			}
			res := compressor.CompressFailed(in, compressor.FormatSwiftTest)
			if res.Applied || !bytes.Equal(res.Output, in) {
				t.Fatalf("CompressFailed modified failing output: applied=%v reason=%q", res.Applied, res.Reason)
			}
		})
	}
}

func TestCompressSwiftTest_PreservesAmbiguousOrCorrupt(t *testing.T) {
	clean := readFixture(t, "swift_test_pass_clean.txt")
	cases := map[string][]byte{
		"no trailing newline": bytes.TrimSuffix(clean, []byte("\n")),
		"crlf":                bytes.ReplaceAll(clean, []byte("\n"), []byte("\r\n")),
		"unrecognised line":   []byte("print(\"debug\")\n" + string(clean)),
		"danger in test name": []byte("Test Suite 'All tests' started at 2026-11-19 10:00:00.000.\nTest Case '-[Pkg.Tests testErrorCase]' passed (0.001 seconds).\nTest Suite 'All tests' passed at 2026-11-19 10:00:00.001.\n\t Executed 1 test, with 0 failures (0 unexpected) in 0.001 (0.001) seconds\n"),
		"zero tests only":     []byte("Test Suite 'All tests' started at 2026-11-19 10:00:00.000.\nTest Suite 'All tests' passed at 2026-11-19 10:00:00.001.\n\t Executed 0 tests, with 0 failures (0 unexpected) in 0.000 (0.001) seconds\n"),
		"duplicate summary":   append(append([]byte{}, clean...), []byte("✔ Test run with 2 tests in 1 suite passed after 0.019 seconds.\n")...),
		"truncated":           clean[:len(clean)/2],
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			res := compressor.CompressExit(in, compressor.ModeSafe, 0)
			if res.Applied || !bytes.Equal(res.Output, in) {
				t.Fatalf("%s was unexpectedly compacted: applied=%v reason=%q out=%q", name, res.Applied, res.Reason, res.Output)
			}
		})
	}
}
