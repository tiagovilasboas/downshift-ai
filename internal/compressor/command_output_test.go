// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"strings"
	"testing"
)

func TestCompressCommandOutputGuards(t *testing.T) {
	const findOut = "internal/a.go\ninternal/b.go\ninternal/c.go\n"
	cases := []struct {
		name       string
		input      string
		exit       int
		wantReason string
	}{
		{"non-zero exit stays raw", findOut, 1, "nonzero_exit_preserved"},
		{"empty output stays raw", "", 0, "empty_input"},
		{"NUL byte stays raw", "internal/\x00a.go\n", 0, "binary_or_invalid_utf8"},
		{"invalid UTF-8 stays raw", "internal/\xff.go\n", 0, "binary_or_invalid_utf8"},
		{"single line without newline stays raw", "internal/a.go", 0, "no_final_newline_preserved"},
		{"carriage return stays raw", "internal/a.go\r\ninternal/b.go\n", 0, "carriage_return_preserved"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := CompressCommandOutput([]byte(tc.input), FormatFind, tc.exit)
			if res.Applied || res.Reason != tc.wantReason || string(res.Output) != tc.input {
				t.Fatalf("applied=%v reason=%q, want raw with reason %q", res.Applied, res.Reason, tc.wantReason)
			}
			if res.Format != FormatFind {
				t.Fatalf("format=%q, want %q", res.Format, FormatFind)
			}
		})
	}
}

func TestCompressCommandOutputNewlineHandling(t *testing.T) {
	cases := []struct {
		name        string
		input       string
		wantApplied bool
		wantReason  string
	}{
		{
			name:        "single line plain without newline stays raw",
			input:       "internal/compressor/a.go",
			wantApplied: false,
			wantReason:  "no_final_newline_preserved",
		},
		{
			name:        "single line coloured without newline stays raw",
			input:       "\x1b[32minternal/compressor/a.go\x1b[0m",
			wantApplied: false,
			wantReason:  "no_final_newline_preserved",
		},
		{
			name:        "multi-line without final newline compacts cleanly",
			input:       "internal/compressor/a.go\ninternal/compressor/b.go\ninternal/compressor/c.go",
			wantApplied: true,
		},
		{
			name:        "multi-line coloured without final newline compacts cleanly",
			input:       "\x1b[32minternal/compressor/a.go\x1b[0m\ninternal/compressor/b.go\n\x1b[32minternal/compressor/c.go\x1b[0m",
			wantApplied: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := CompressCommandOutput([]byte(tc.input), FormatFind, 0)
			if res.Applied != tc.wantApplied {
				t.Fatalf("applied=%v reason=%q, want applied=%v", res.Applied, res.Reason, tc.wantApplied)
			}
			if !tc.wantApplied {
				if res.Reason != tc.wantReason || string(res.Output) != tc.input {
					t.Fatalf("reason=%q output=%q, want reason=%q raw=%q", res.Reason, res.Output, tc.wantReason, tc.input)
				}
				return
			}
			if res.Format != FormatFind || res.OriginalBytes != len(tc.input) || res.ReducedBytes != len(res.Output) {
				t.Fatalf("unexpected result metadata: %+v", res)
			}
			if strings.Contains(string(res.Output), "\x1b[") {
				t.Fatalf("colour escape kept in output: %q", res.Output)
			}
		})
	}
}

func TestCompressCommandOutputAppliesAndStripsColour(t *testing.T) {
	in := "\x1b[32minternal/compressor/a.go\x1b[0m\ninternal/compressor/b.go\ninternal/compressor/c.go\n"
	res := CompressCommandOutput([]byte(in), FormatFind, 0)
	if !res.Applied || res.Format != FormatFind {
		t.Fatalf("applied=%v format=%q reason=%q", res.Applied, res.Format, res.Reason)
	}
	if res.ReducedBytes != len(res.Output) || res.OriginalBytes != len(in) {
		t.Fatalf("byte accounting: original=%d reduced=%d", res.OriginalBytes, res.ReducedBytes)
	}
	if strings.Contains(string(res.Output), "\x1b[") {
		t.Fatalf("colour escape kept in output: %q", res.Output)
	}
}
