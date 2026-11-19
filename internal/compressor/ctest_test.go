// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompressCTest_PassingRun(t *testing.T) {
	in, err := os.ReadFile(filepath.Join("testdata", "corpus", "ctest_pass_clean.txt"))
	if err != nil {
		t.Fatal(err)
	}
	res := CompressExit(in, ModeSafe, 0)
	if !res.Applied || res.Format != FormatCTest {
		t.Fatalf("got applied=%v format=%s reason=%q, want applied ctest", res.Applied, res.Format, res.Reason)
	}
	want := "CTest: PASS (12 passed in 0.34s)\n"
	if string(res.Output) != want {
		t.Fatalf("got %q, want %q", res.Output, want)
	}
	if len(res.Output) >= len(in) {
		t.Fatalf("expected smaller output: %d >= %d", len(res.Output), len(in))
	}
	// ShortenPass strips the duration when CCR keeps the raw output.
	short, ok := ShortenPass(res.Output)
	if !ok || string(short) != "CTest: PASS (12 passed)\n" {
		t.Fatalf("ShortenPass(%q) = %q, %v", res.Output, short, ok)
	}
}

func TestCompressCTest_ParallelAndLabels(t *testing.T) {
	in := []byte(`Internal ctest changing into directory: /Users/dev/acme/build
Test project /Users/dev/acme/build
      Start  1: unit_alpha
      Start  2: unit_beta
      Start  3: e2e_gamma
 1/3 Test  #2: unit_beta ........................   Passed    0.02 sec
 2/3 Test  #1: unit_alpha .......................   Passed    0.05 sec
 3/3 Test  #3: e2e_gamma ........................   Passed    0.11 sec

100% tests passed, 0 tests failed out of 3

Label Time Summary:
unit    =   0.07 sec*proc (2 tests)
e2e     =   0.11 sec*proc (1 test)

Total Test time (real) =   0.12 sec
`)
	res := CompressExit(in, ModeSafe, 0)
	if !res.Applied || res.Format != FormatCTest {
		t.Fatalf("got applied=%v format=%s reason=%q", res.Applied, res.Format, res.Reason)
	}
	if string(res.Output) != "CTest: PASS (3 passed in 0.12s)\n" {
		t.Fatalf("got %q", res.Output)
	}
}

func TestCompressCTest_FailingRunPreserved(t *testing.T) {
	in, err := os.ReadFile(filepath.Join("testdata", "corpus", "ctest_fail.txt"))
	if err != nil {
		t.Fatal(err)
	}
	for _, code := range []int{-1, 0, 1, 8} {
		res := CompressExit(in, ModeSafe, code)
		if res.Applied || !bytes.Equal(res.Output, in) {
			t.Fatalf("exit=%d: failing ctest modified: applied=%v reason=%q", code, res.Applied, res.Reason)
		}
		for _, want := range []string{
			"***Failed",
			"Assertion failed: (div_checked(10, 0) == 0)",
			"***Timeout",
			"50% tests passed, 2 tests failed out of 4",
			"The following tests FAILED:",
			"Errors while running CTest",
		} {
			if !strings.Contains(string(res.Output), want) {
				t.Fatalf("exit=%d: lost failure detail %q", code, want)
			}
		}
	}
}

func TestCompressCTest_AmbiguousAndNonPassPreserved(t *testing.T) {
	passRaw, err := os.ReadFile(filepath.Join("testdata", "corpus", "ctest_pass_clean.txt"))
	if err != nil {
		t.Fatal(err)
	}
	// Non-zero exit on otherwise passing transcript must stay raw.
	for _, code := range []int{1, 2, 8} {
		res := CompressExit(passRaw, ModeSafe, code)
		if res.Applied || !bytes.Equal(res.Output, passRaw) || res.Reason != "nonzero_exit_preserved" {
			t.Fatalf("exit=%d: expected raw output, got applied=%v reason=%q", code, res.Applied, res.Reason)
		}
	}

	cases := []struct {
		name string
		in   string
	}{
		{
			name: "skipped test",
			in: `Test project /Users/dev/acme/build
      Start  1: unit_alpha
 1/2 Test  #1: unit_alpha .......................   Passed    0.01 sec
      Start  2: unit_skip
 2/2 Test  #2: unit_skip ........................***Skipped   0.00 sec

100% tests passed, 0 tests failed out of 2

The following tests did not run:
	  2 - unit_skip (Skipped)

Total Test time (real) =   0.02 sec
`,
		},
		{
			name: "no tests found",
			in: `Test project /Users/dev/acme/build
No tests were found!!!
`,
		},
		{
			name: "truncated before total time",
			in: `Test project /Users/dev/acme/build
      Start  1: unit_alpha
 1/1 Test  #1: unit_alpha .......................   Passed    0.01 sec

100% tests passed, 0 tests failed out of 1
`,
		},
		{
			name: "missing pass line",
			in: `Test project /Users/dev/acme/build
      Start  1: unit_alpha
 1/2 Test  #1: unit_alpha .......................   Passed    0.01 sec

100% tests passed, 0 tests failed out of 2

Total Test time (real) =   0.01 sec
`,
		},
		{
			name: "verbose stdout in test",
			in: `Test project /Users/dev/acme/build
      Start  1: unit_alpha
1: Test command: /Users/dev/acme/build/unit_alpha
 1/1 Test  #1: unit_alpha .......................   Passed    0.01 sec

100% tests passed, 0 tests failed out of 1

Total Test time (real) =   0.01 sec
`,
		},
		{
			name: "danger word in test name",
			in: `Test project /Users/dev/acme/build
      Start  1: test_error_handling
 1/1 Test  #1: test_error_handling ..............   Passed    0.01 sec

100% tests passed, 0 tests failed out of 1

Total Test time (real) =   0.01 sec
`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := []byte(tc.in)
			res := CompressExit(in, ModeSafe, 0)
			if res.Applied || !bytes.Equal(res.Output, in) {
				t.Fatalf("%s was compacted unexpectedly: applied=%v out=%q", tc.name, res.Applied, res.Output)
			}
		})
	}
}
