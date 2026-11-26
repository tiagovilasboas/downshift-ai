// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"reflect"
	"testing"
)

func TestDecide_TightPipesAndTwoStageTails(t *testing.T) {
	const w = "'" + exe + "' compact -- "
	const wd = "'" + exe + "' compact --discard-stderr -- "
	cases := []struct {
		name     string
		command  string
		rewrite  bool
		want     string
		segments []string
		reason   string
	}{
		{
			name:     "tight pipe tail",
			command:  "go test ./...|tail -15",
			rewrite:  true,
			want:     w + "go test ./... | tail -15",
			segments: []string{"tail -15"},
			reason:   ReasonAllowlistedCompound,
		},
		{
			name:     "tight pipe grep",
			command:  "git diff|grep foo",
			rewrite:  true,
			want:     w + "git diff | grep foo",
			segments: []string{"grep foo"},
			reason:   ReasonAllowlistedCompound,
		},
		{
			name:     "tight pipe after 2>/dev/null",
			command:  "go test ./... 2>/dev/null|tail -10",
			rewrite:  true,
			want:     wd + "'go' 'test' './...' | tail -10",
			segments: []string{"tail -10"},
			reason:   ReasonAllowlistedQuoted,
		},
		{
			name:     "tight pipe after 2>&1",
			command:  "go test ./... 2>&1|tail -10",
			rewrite:  true,
			want:     w + "go test ./... 2>&1 | tail -10",
			segments: []string{"tail -10"},
			reason:   ReasonAllowlistedCompound,
		},
		{
			name:     "two-stage grep then head",
			command:  "go test ./... | grep FAIL | head -20",
			rewrite:  true,
			want:     w + "go test ./... | grep FAIL | head -20",
			segments: []string{"grep FAIL", "head -20"},
			reason:   ReasonAllowlistedCompound,
		},
		{
			name:     "two-stage tight rg then tail",
			command:  "go test ./... 2>&1|rg -n FAIL|tail -5",
			rewrite:  true,
			want:     w + "go test ./... 2>&1 | rg -n FAIL | tail -5",
			segments: []string{"rg -n FAIL", "tail -5"},
			reason:   ReasonAllowlistedCompound,
		},
		{
			name:     "two-stage sort then uniq",
			command:  "git log --oneline | sort | uniq -c",
			rewrite:  true,
			want:     w + "git log --oneline | sort | uniq -c",
			segments: []string{"sort", "uniq -c"},
			reason:   ReasonAllowlistedCompound,
		},
		{
			name:     "two-stage quoted grep then head",
			command:  `go test -run 'A|B' ./... | grep -E "FAIL|panic" | head -5`,
			rewrite:  true,
			want:     w + `'go' 'test' '-run' 'A|B' './...' | 'grep' '-E' 'FAIL|panic' | head -5`,
			segments: []string{"go test -run A|B ./...", `grep -E "FAIL|panic"`, "grep -E FAIL|panic", "head -5"},
			reason:   ReasonAllowlistedQuoted,
		},
		{
			name:     "two-stage 2>/dev/null before pipes",
			command:  "go test ./... 2>/dev/null | grep FAIL | tail -5",
			rewrite:  true,
			want:     wd + "'go' 'test' './...' | grep FAIL | tail -5",
			segments: []string{"grep FAIL", "tail -5"},
			reason:   ReasonAllowlistedQuoted,
		},
		{
			name:    "or-list never split as pipe",
			command: "go test || false",
			rewrite: false,
			reason:  ReasonUnsafeChar,
		},
		{
			name:    "three-stage pipe refused",
			command: "go test ./... | grep FAIL | sort | head -5",
			rewrite: false,
			reason:  ReasonUnsafeChar,
		},
		{
			name:    "two-stage with file operand refused",
			command: "go test ./... | grep FAIL | head -5 /etc/passwd",
			rewrite: false,
			reason:  ReasonUnsafeChar,
		},
		{
			name:    "empty pipe stage refused",
			command: "go test ./... | | tail -5",
			rewrite: false,
			reason:  ReasonUnsafeChar,
		},
		{
			name:    "file-read with two-stage filter refused",
			command: "cat main.go | grep func | head -5",
			rewrite: false,
			reason:  ReasonUnsafeChar,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := Decide(tc.command, exe)
			if d.Rewrite != tc.rewrite || d.Command != tc.want || d.Reason != tc.reason || !reflect.DeepEqual(d.Segments, tc.segments) {
				t.Fatalf("Decide(%q) = %+v, want rewrite=%v command=%q segments=%q reason=%s",
					tc.command, d, tc.rewrite, tc.want, tc.segments, tc.reason)
			}
		})
	}
}
