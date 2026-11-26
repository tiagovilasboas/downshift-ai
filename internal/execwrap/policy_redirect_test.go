// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"reflect"
	"testing"
)

func TestDecide_TightPipeAfterStderrRedirect(t *testing.T) {
	const w = "'" + exe + "' compact -- "
	const wd = "'" + exe + "' compact --discard-stderr -- "
	cases := []struct {
		name     string
		command  string
		family   Family
		want     string
		argv     string
		segments []string
		reason   string
	}{
		{
			name:     "tight pipe after 2>&1 with tail",
			command:  "go test ./... 2>&1|tail -15",
			family:   FamilyGoTest,
			want:     w + "go test ./... 2>&1 | tail -15",
			argv:     "go test ./...",
			segments: []string{"tail -15"},
			reason:   ReasonAllowlistedCompound,
		},
		{
			name:     "tight pipe after 2>/dev/null with grep",
			command:  "git diff 2>/dev/null|grep foo",
			family:   FamilyGitDiff,
			want:     wd + "'git' 'diff' | grep foo",
			argv:     "git diff",
			segments: []string{"grep foo"},
			reason:   ReasonAllowlistedQuoted,
		},
		{
			name:     "tight pipe after 2> /dev/null with head",
			command:  "git log -n 5 2> /dev/null|head -5",
			family:   FamilyGitLog,
			want:     wd + "'git' 'log' '-n' '5' | head -5",
			argv:     "git log -n 5",
			segments: []string{"head -5"},
			reason:   ReasonAllowlistedQuoted,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := Decide(tc.command, exe)
			if !d.Rewrite || d.Family != tc.family || d.Command != tc.want || d.Argv != tc.argv || d.Reason != tc.reason || !reflect.DeepEqual(d.Segments, tc.segments) {
				t.Fatalf("Decide(%q) = %+v\nwant rewrite=true family=%s command=%q argv=%q segments=%q reason=%s",
					tc.command, d, tc.family, tc.want, tc.argv, tc.segments, tc.reason)
			}
		})
	}
}

func TestExtractStderrRedirect_TightPipeAndRefusals(t *testing.T) {
	cases := []struct {
		name    string
		command string
		want    string
		kind    stderrRedirect
		ok      bool
	}{
		{"tight pipe after 2>&1", "go test ./... 2>&1|tail -15", "go test ./... |tail -15", redirMerge, true},
		{"tight pipe after 2>/dev/null", "git diff 2>/dev/null|grep foo", "git diff |grep foo", redirDiscard, true},
		{"double pipe after 2>&1 not extracted", "go test ./... 2>&1||true", "go test ./... 2>&1||true", redirNone, true},
		{"double pipe after 2>/dev/null not extracted", "git diff 2>/dev/null||true", "git diff 2>/dev/null||true", redirNone, true},
		{"2>&1 and 2>/dev/null combined refused", "go test ./... 2>&1 2>/dev/null|tail -5", "", redirNone, false},
		{"stdout redirect untouched", "go test ./... >file|tail -5", "go test ./... >file|tail -5", redirNone, true},
		{"stderr file redirect untouched", "go test ./... 2>file|tail -5", "go test ./... 2>file|tail -5", redirNone, true},
		{"stdout to stderr untouched", "go test ./... 1>&2|tail -5", "go test ./... 1>&2|tail -5", redirNone, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, kind, ok := extractStderrRedirect(tc.command)
			if ok != tc.ok || kind != tc.kind || got != tc.want {
				t.Fatalf("extractStderrRedirect(%q) = (%q, %v, %v), want (%q, %v, %v)",
					tc.command, got, kind, ok, tc.want, tc.kind, tc.ok)
			}
		})
	}
}
