// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"bytes"
	"strings"
	"testing"
)

func TestMixTest_CompactsCleanRuns(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "standard run",
			in: "Compiling 3 files (.ex)\n" +
				"Generated ledger app\n" +
				"Running ExUnit with seed: 482913, max_cases: 16\n\n" +
				"..........................................\n\n" +
				"Finished in 0.42 seconds (0.12s async, 0.30s sync)\n" +
				"42 tests, 0 failures\n",
			want: "Mix test: PASS (42 tests, 0 failures in 0.42s)\n",
		},
		{
			name: "doctests properties excluded skipped",
			in: "Running ExUnit with seed: 102938, max_cases: 8\n" +
				"Excluding tags: [:pending]\n\n" +
				"..*....................\n\n" +
				"Finished in 0.15 seconds (0.05s async, 0.10s sync)\n" +
				"1 doctest, 2 properties, 20 tests, 0 failures, 1 excluded, 3 skipped\n",
			want: "Mix test: PASS (1 doctest, 2 properties, 20 tests, 0 failures, 1 excluded, 3 skipped in 0.15s)\n",
		},
		{
			name: "legacy randomized seed and minutes",
			in: "........................................\n\n" +
				"Finished in 1 minute, 2.3 seconds\n" +
				"40 tests, 0 failures\n\n" +
				"Randomized with seed 99123\n",
			want: "Mix test: PASS (40 tests, 0 failures in 62.30s)\n",
		},
		{
			name: "umbrella multi-app run",
			in: "==> app_one\n" +
				"Running ExUnit with seed: 111, max_cases: 8\n\n" +
				"............\n\n" +
				"Finished in 0.12 seconds (0.08s async, 0.04s sync)\n" +
				"12 tests, 0 failures\n\n" +
				"==> app_two\n" +
				"Running ExUnit with seed: 111, max_cases: 8\n\n" +
				"..................\n\n" +
				"Finished in 0.18 seconds (0.10s async, 0.08s sync)\n" +
				"18 tests, 0 failures\n",
			want: "Mix test: PASS (30 tests, 0 failures in 0.30s)\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := CompressExit([]byte(tc.in), ModeSafe, 0)
			if !res.Applied || res.Format != FormatMixTest {
				t.Fatalf("got applied=%v format=%s reason=%q", res.Applied, res.Format, res.Reason)
			}
			if string(res.Output) != tc.want {
				t.Fatalf("got %q, want %q", res.Output, tc.want)
			}
		})
	}
}

func TestMixTest_CompactsANSI(t *testing.T) {
	in := "\x1b[32mRunning ExUnit with seed: 482913, max_cases: 16\x1b[0m\n\n" +
		"\x1b[32m..........................................\x1b[0m\n\n" +
		"Finished in 0.42 seconds (0.12s async, 0.30s sync)\n" +
		"\x1b[32m42 tests, 0 failures\x1b[0m\n"
	res := CompressExit([]byte(in), ModeSafe, 0)
	if !res.Applied || res.Format != FormatMixTest {
		t.Fatalf("ANSI clean mix test not compacted: applied=%v format=%s reason=%q", res.Applied, res.Format, res.Reason)
	}
	want := "Mix test: PASS (42 tests, 0 failures in 0.42s)\n"
	if string(res.Output) != want {
		t.Fatalf("got %q, want %q", res.Output, want)
	}
}

func TestMixTest_PreservesFailuresAndUnrecognised(t *testing.T) {
	const green = "Running ExUnit with seed: 482913, max_cases: 16\n\n" +
		"..........................................\n\n" +
		"Finished in 0.42 seconds (0.12s async, 0.30s sync)\n" +
		"42 tests, 0 failures\n"

	cases := map[string]string{
		"test failure": "Running ExUnit with seed: 482913, max_cases: 16\n\n.F..\n\n" +
			"  1) test adds line items to total (Ledger.InvoiceTest)\n" +
			"     test/ledger/invoice_test.exs:14\n" +
			"     Assertion with == failed\n" +
			"     code:  assert Invoice.total(invoice) == 110\n" +
			"     left:  100\n" +
			"     right: 110\n" +
			"     stacktrace:\n" +
			"       test/ledger/invoice_test.exs:16: (test)\n\n" +
			"Finished in 0.19 seconds (0.05s async, 0.14s sync)\n" +
			"4 tests, 1 failure\n",
		"compiler warning": "Compiling 1 file (.ex)\n" +
			"warning: variable \"x\" is unused\n" +
			"  lib/ledger/invoice.ex:12: Ledger.Invoice.total/1\n\n" + green,
		"debug IO.puts": "Running ExUnit with seed: 482913, max_cases: 16\n\n" +
			"debug value: 42\n..\n\n" +
			"Finished in 0.12 seconds (0.05s async, 0.07s sync)\n" +
			"2 tests, 0 failures\n",
		"invalid test": "Running ExUnit with seed: 482913, max_cases: 16\n\n" +
			"..\n\nFinished in 0.12 seconds (0.05s async, 0.07s sync)\n" +
			"2 tests, 0 failures, 1 invalid\n",
		"zero tests": "Running ExUnit with seed: 482913, max_cases: 16\n\n" +
			"Finished in 0.00 seconds (0.00s async, 0.00s sync)\n" +
			"0 tests, 0 failures\n",
		"duplicate summary": green + "42 tests, 0 failures\n",
		"missing newline":   strings.TrimSuffix(green, "\n"),
		"truncated tail":    "Running ExUnit with seed: 482913, max_cases: 16\n\n....................\n",
	}

	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			for _, code := range []int{-1, 0, 1} {
				res := CompressExit([]byte(in), ModeSafe, code)
				if res.Applied || !bytes.Equal(res.Output, []byte(in)) {
					t.Fatalf("exit=%d: expected raw preservation, got applied=%v out=%q", code, res.Applied, res.Output)
				}
			}
		})
	}
}

func TestMixTest_DetectionDoesNotClaimRspec(t *testing.T) {
	rspec := "Randomized with seed 41873\n......\n\nFinished in 3.13 seconds (files took 1.21 seconds to load)\n6 examples, 0 failures\n"
	if f, _ := detectFormat([]byte(rspec)); f != FormatRspec {
		t.Fatalf("rspec misdetected as %s", f)
	}
	mix := "Running ExUnit with seed: 482913, max_cases: 16\n\n......\n\nFinished in 0.42 seconds (0.12s async, 0.30s sync)\n6 tests, 0 failures\n"
	if f, isErr := detectFormat([]byte(mix)); f != FormatMixTest || isErr {
		t.Fatalf("mix test misdetected as (%s, %v)", f, isErr)
	}
}
