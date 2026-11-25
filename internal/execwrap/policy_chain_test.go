// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"reflect"
	"strings"
	"testing"
)

func TestDecide_Chains(t *testing.T) {
	const w = "'/opt/ds/downshift' compact -- "
	cases := []struct {
		name, command string
		rewrite       bool
		want          string
		argv          string
		segments      []string
		reason        string
	}{
		// The benchmark commands.
		{"gofmt then go test", "gofmt -l internal/telemetry/; go test ./internal/telemetry/", true,
			"gofmt -l internal/telemetry/; " + w + "go test ./internal/telemetry/",
			"go test ./internal/telemetry/", []string{"gofmt -l internal/telemetry/"}, ReasonAllowlistedChain},
		{"diff, gofmt, go test", "git diff; gofmt -l cmd internal; go test ./cmd/downshift/ ./internal/telemetry/ ./internal/adapters/cursor/", true,
			w + "git diff; gofmt -l cmd internal; " + w + "go test ./cmd/downshift/ ./internal/telemetry/ ./internal/adapters/cursor/",
			"git diff", []string{"gofmt -l cmd internal", "go test ./cmd/downshift/ ./internal/telemetry/ ./internal/adapters/cursor/"}, ReasonAllowlistedChain},
		{"gofmt then go test with tail", "gofmt -l cmd/downshift/doctor.go; go test ./cmd/downshift/ 2>&1 | tail -15", true,
			"gofmt -l cmd/downshift/doctor.go; " + w + "go test ./cmd/downshift/ 2>&1 | tail -15",
			"go test ./cmd/downshift/", []string{"gofmt -l cmd/downshift/doctor.go", "tail -15"}, ReasonAllowlistedChain},
		{"grep pipe wraps safely", "git diff --stat && git diff | grep '^[+-]'", true,
			w + "git diff --stat && " + w + "git diff | 'grep' '^[+-]'",
			"git diff --stat", []string{"grep '^[+-]'", "grep ^[+-]", "git diff"}, ReasonAllowlistedQuoted},
		{"grep pipe with file operand refused", "git diff --stat && git diff | grep '^[+-]' /etc/passwd", false, "", "", nil, ReasonUnsafeChar},

		// Shapes.
		{"and list", "go build ./... && go test ./...", true, w + "go build ./... && " + w + "go test ./...", "go build ./...", []string{"go test ./..."}, ReasonAllowlistedChain},
		{"mixed separators", "cd x && go vet ./...; git status", true, "cd x && " + w + "go vet ./...; " + w + "git status", "go vet ./...", []string{"cd x", "git status"}, ReasonAllowlistedChain},
		{"no space before semicolon", "git status;go test", true, w + "git status;" + w + "go test", "git status", []string{"go test"}, ReasonAllowlistedChain},
		{"kept segment byte-identical", "gofmt  -l   x   ; go test", true, "gofmt  -l   x   ; " + w + "go test", "go test", []string{"gofmt -l x"}, ReasonAllowlistedChain},
		{"not allowlisted after", "go test ./... && echo ok", true, w + "go test ./... && echo ok", "go test ./...", []string{"echo ok"}, ReasonAllowlistedChain},
		{"cd option kept", "cd -P x && go test", true, "cd -P x && " + w + "go test", "go test", []string{"cd -P x"}, ReasonAllowlistedChain},
		{"compound not allowlisted kept", "sudo ls | tail -5; go test", true, "sudo ls | tail -5; " + w + "go test", "go test", []string{"sudo ls | tail -5"}, ReasonAllowlistedChain},

		// Refusals: one unproven segment refuses the whole command.
		{"nothing allowlisted", "gofmt -l x; echo ok", false, "", "", nil, ReasonNotAllowlisted},
		{"or list", "gofmt -l x; go test || true", false, "", "", nil, ReasonUnsafeChar},
		{"background", "go test & git status; go vet", false, "", "", nil, ReasonUnsafeChar},
		{"triple ampersand", "go test &&& git status", false, "", "", nil, ReasonUnsafeChar},
		{"quoted separator", "git commit -m 'a; b'; go test", true,
			w + "'git' 'commit' '-m' 'a; b'; " + w + "go test",
			"git commit -m 'a; b'", []string{"git commit -m a; b", "go test"}, ReasonAllowlistedQuoted},
		{"double quoted separator in a chain", `go vet ./...; go test -run "A && B"`, true,
			w + `go vet ./...; ` + w + `'go' 'test' '-run' 'A && B'`,
			"go vet ./...", []string{`go test -run A && B`, `go test -run "A && B"`}, ReasonAllowlistedQuoted},
		{"and list with single quoted and", "git status && git commit -m 'a && b'", true,
			w + "git status && " + w + "'git' 'commit' '-m' 'a && b'",
			"git status", []string{"git commit -m a && b", "git commit -m 'a && b'"}, ReasonAllowlistedQuoted},
		{"unclosed single quote in chain", "git commit -m 'a; go test", false, "", "", nil, ReasonUnsafeChar},
		{"unclosed double quote in chain", `go vet ./...; go test -run "A && B`, false, "", "", nil, ReasonUnsafeChar},
		{"escaped quote inside double quotes in chain", `go vet ./...; go test -run "A\"B && C"`, false, "", "", nil, ReasonUnsafeChar},
		{"subshell", "(go test); git status", false, "", "", nil, ReasonUnsafeChar},
		{"substitution", "echo $(id); go test", false, "", "", nil, ReasonUnsafeChar},
		{"backtick", "go test `pwd`; git status", false, "", "", nil, ReasonUnsafeChar},
		{"heredoc", "cat <<EOF; go test", false, "", "", nil, ReasonUnsafeChar},
		{"redirect", "go test > out.txt; git status", false, "", "", nil, ReasonUnsafeChar},
		{"glob", "gofmt -l *.go; go test", false, "", "", nil, ReasonUnsafeChar},
		{"newline", "gofmt -l x\ngo test; git status", false, "", "", nil, ReasonUnsafeChar},
		{"empty segment", "git status;; go test", false, "", "", nil, ReasonUnsafeChar},
		{"trailing semicolon", "git status; go test;", false, "", "", nil, ReasonUnsafeChar},
		{"leading and", "&& go test", false, "", "", nil, ReasonUnsafeChar},
		{"already wrapped segment", "downshift compact -- go test; git status", false, "", "", nil, ReasonAlreadyWrapped},
		{"already wrapped quoted", "'/x/downshift' compact -- go test; git status", false, "", "", nil, ReasonAlreadyWrapped},
		{"git global option", "gofmt -l x; git -C y status", false, "", "", nil, ReasonNotAllowlisted},
		{"too long", "git status; go test " + strings.Repeat("a", 4096), false, "", "", nil, ReasonTooLong},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := Decide(tc.command, exe)
			if d.Rewrite != tc.rewrite || d.Command != tc.want || d.Argv != tc.argv || d.Reason != tc.reason || !reflect.DeepEqual(d.Segments, tc.segments) {
				t.Fatalf("Decide(%q) = %+v, want rewrite=%v command=%q argv=%q segments=%q reason=%s", tc.command, d, tc.rewrite, tc.want, tc.argv, tc.segments, tc.reason)
			}
		})
	}
}

// A wrapped chain is never wrapped again.
func TestDecide_ChainIdempotent(t *testing.T) {
	d := Decide("git diff; gofmt -l x; go test ./... 2>&1 | tail -5", exe)
	if !d.Rewrite {
		t.Fatalf("first pass not rewritten: %+v", d)
	}
	for _, c := range []string{d.Command, WithSession(d.Command, hashA)} {
		if again := Decide(c, exe); again.Rewrite {
			t.Fatalf("wrapped chain rewritten again: %q -> %q", c, again.Command)
		}
	}
}

// Every wrapped segment gets the session tag once; kept segments are not
// touched, even one that reads like a wrapped tail.
func TestWithSession_Chain(t *testing.T) {
	d := Decide("git diff; gofmt -l compact -- x; go test ./... 2>&1 | tail -5", exe)
	got := WithSession(d.Command, hashA)
	tag := "'/opt/ds/downshift' compact --session=" + hashA + " -- "
	want := tag + "git diff; gofmt -l compact -- x; " + tag + "go test ./... 2>&1 | tail -5"
	if got != want {
		t.Fatalf("WithSession = %q, want %q", got, want)
	}
	if WithSession(got, hashA) != got {
		t.Fatalf("WithSession tagged a tagged chain twice")
	}
}

func TestSplitChain_QuoteAware(t *testing.T) {
	cases := []struct {
		name     string
		command  string
		wantOK   bool
		wantSegs []string
		wantSeps []string
	}{
		{"single quoted semicolon", "git commit -m 'a; b'; go test", true,
			[]string{"git commit -m 'a; b'", "go test"}, []string{"; "}},
		{"double quoted &&", `go vet ./...; go test -run "A && B"`, true,
			[]string{"go vet ./...", `go test -run "A && B"`}, []string{"; "}},
		{"escaped quote inside double quotes", `echo "a\"; b" && go test`, true,
			[]string{`echo "a\"; b"`, "go test"}, []string{" && "}},
		{"escaped backslash before closing double quote", `echo "a\\" ; go test`, true,
			[]string{`echo "a\\"`, "go test"}, []string{" ; "}},
		{"backslash literal inside single quotes", `echo 'a\' ; go test`, true,
			[]string{`echo 'a\'`, "go test"}, []string{" ; "}},
		{"escaped semicolon outside quotes", `echo a\; b; go test`, true,
			[]string{`echo a\; b`, "go test"}, []string{"; "}},
		{"unclosed single quote", "git commit -m 'a; go test", false, nil, nil},
		{"unclosed double quote", `go test -run "a && b; git status`, false, nil, nil},
		{"trailing backslash in double quote", `echo "abc\`, false, nil, nil},
		{"only quoted separator no outer chain", "git commit -m 'a; b'", false, nil, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			segs, seps, ok := splitChain(tc.command)
			if ok != tc.wantOK || !reflect.DeepEqual(segs, tc.wantSegs) || !reflect.DeepEqual(seps, tc.wantSeps) {
				t.Fatalf("splitChain(%q) = (%q, %q, %v), want (%q, %q, %v)",
					tc.command, segs, seps, ok, tc.wantSegs, tc.wantSeps, tc.wantOK)
			}
		})
	}
}
