// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestDecide_SafeTextFilterPipes(t *testing.T) {
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
			name:     "pipe sort bare",
			command:  "git diff --name-only | sort",
			family:   FamilyGitDiff,
			want:     w + "git diff --name-only | sort",
			argv:     "git diff --name-only",
			segments: []string{"sort"},
			reason:   ReasonAllowlistedCompound,
		},
		{
			name:     "pipe sort flags and key",
			command:  "git log --oneline | sort -rnk2 -t: -u",
			family:   FamilyGitLog,
			want:     w + "git log --oneline | sort -rnk2 -t: -u",
			argv:     "git log --oneline",
			segments: []string{"sort -rnk2 -t: -u"},
			reason:   ReasonAllowlistedCompound,
		},
		{
			name:     "pipe sort quoted separator",
			command:  `git status --short | sort -t ' ' -k 2,2 -V --check=quiet`,
			family:   FamilyGitStatus,
			want:     w + `git status --short | 'sort' '-t' ' ' '-k' '2,2' '-V' '--check=quiet'`,
			argv:     "git status --short",
			segments: []string{`sort -t ' ' -k 2,2 -V --check=quiet`, "sort -t   -k 2,2 -V --check=quiet"},
			reason:   ReasonAllowlistedQuoted,
		},
		{
			name:     "pipe sort long flags",
			command:  "git branch | sort --unique --reverse --ignore-case --key=1,1 --field-separator=:",
			family:   FamilyGitBranch,
			want:     w + "git branch | sort --unique --reverse --ignore-case --key=1,1 --field-separator=:",
			argv:     "git branch",
			segments: []string{"sort --unique --reverse --ignore-case --key=1,1 --field-separator=:"},
			reason:   ReasonAllowlistedCompound,
		},
		{
			name:     "pipe uniq bare",
			command:  "git diff --name-only | uniq",
			family:   FamilyGitDiff,
			want:     w + "git diff --name-only | uniq",
			argv:     "git diff --name-only",
			segments: []string{"uniq"},
			reason:   ReasonAllowlistedCompound,
		},
		{
			name:     "pipe uniq flags and counts",
			command:  "git log --oneline | uniq -c -i -f 1 -s 2 -w 10",
			family:   FamilyGitLog,
			want:     w + "git log --oneline | uniq -c -i -f 1 -s 2 -w 10",
			argv:     "git log --oneline",
			segments: []string{"uniq -c -i -f 1 -s 2 -w 10"},
			reason:   ReasonAllowlistedCompound,
		},
		{
			name:     "pipe uniq clustered and long",
			command:  "git branch | uniq -cf2 --skip-chars=1 --check-chars 8",
			family:   FamilyGitBranch,
			want:     w + "git branch | uniq -cf2 --skip-chars=1 --check-chars 8",
			argv:     "git branch",
			segments: []string{"uniq -cf2 --skip-chars=1 --check-chars 8"},
			reason:   ReasonAllowlistedCompound,
		},
		{
			name:     "pipe wc bare and flags",
			command:  "go test ./... | wc",
			family:   FamilyGoTest,
			want:     w + "go test ./... | wc",
			argv:     "go test ./...",
			segments: []string{"wc"},
			reason:   ReasonAllowlistedCompound,
		},
		{
			name:     "pipe wc clustered and long",
			command:  "git diff --stat | wc -lw --bytes -m -L",
			family:   FamilyGitDiff,
			want:     w + "git diff --stat | wc -lw --bytes -m -L",
			argv:     "git diff --stat",
			segments: []string{"wc -lw --bytes -m -L"},
			reason:   ReasonAllowlistedCompound,
		},
		{
			name:     "pipe cut quoted delimiter",
			command:  `git log --oneline | cut -d ' ' -f 1`,
			family:   FamilyGitLog,
			want:     w + `git log --oneline | 'cut' '-d' ' ' '-f' '1'`,
			argv:     "git log --oneline",
			segments: []string{`cut -d ' ' -f 1`, "cut -d   -f 1"},
			reason:   ReasonAllowlistedQuoted,
		},
		{
			name:     "pipe cut output delimiter holding pipe char",
			command:  `git log --oneline | cut -s -d: -f1,2 --complement --output-delimiter='|'`,
			family:   FamilyGitLog,
			want:     w + `git log --oneline | 'cut' '-s' '-d:' '-f1,2' '--complement' '--output-delimiter=|'`,
			argv:     "git log --oneline",
			segments: []string{`cut -s -d: -f1,2 --complement --output-delimiter='|'`, "cut -s -d: -f1,2 --complement --output-delimiter=|"},
			reason:   ReasonAllowlistedQuoted,
		},
		{
			name:     "pipe cut chars and bytes",
			command:  "git status --short | cut -c1-10",
			family:   FamilyGitStatus,
			want:     w + "git status --short | cut -c1-10",
			argv:     "git status --short",
			segments: []string{"cut -c1-10"},
			reason:   ReasonAllowlistedCompound,
		},
		{
			name:     "cd, 2>&1, and pipe wc",
			command:  "cd internal && go test ./... 2>&1 | wc -l",
			family:   FamilyGoTest,
			want:     "cd internal && " + w + "go test ./... 2>&1 | wc -l",
			argv:     "go test ./...",
			segments: []string{"cd internal", "wc -l"},
			reason:   ReasonAllowlistedCompound,
		},
		{
			name:     "quoted producer with 2>/dev/null and pipe sort",
			command:  `find . -name "*.go" 2>/dev/null | sort -u`,
			family:   FamilyFind,
			want:     wd + `'find' '.' '-name' '*.go' | sort -u`,
			argv:     `find . -name "*.go"`,
			segments: []string{"find . -name *.go", "sort -u"},
			reason:   ReasonAllowlistedQuoted,
		},
		{
			name:     "chain with text filter pipe",
			command:  "git status && git diff --name-only | sort -u",
			family:   FamilyGitStatus,
			want:     w + "git status && " + w + "git diff --name-only | sort -u",
			argv:     "git status",
			segments: []string{"sort -u", "git diff --name-only"},
			reason:   ReasonAllowlistedChain,
		},
		{
			name:     "pipe text filter with trailing 2>/dev/null",
			command:  "go test ./... | wc -l 2>/dev/null",
			family:   FamilyGoTest,
			want:     w + "go test ./... | wc -l 2>/dev/null",
			argv:     "go test ./...",
			segments: []string{"wc -l", "wc -l 2>/dev/null"},
			reason:   ReasonAllowlistedCompound,
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

func TestDecide_RefusesUnsafeTextFilterPipes(t *testing.T) {
	cases := []struct {
		name    string
		command string
	}{
		// Positional file operands must always be refused.
		{"sort with file operand", "git status | sort /etc/passwd"},
		{"sort flag then file operand", "git status | sort -u /etc/passwd"},
		{"sort double-dash file operand", "git status | sort -- /etc/passwd"},
		{"sort dash operand", "git status | sort -"},
		{"uniq with file operand", "git log --oneline | uniq /etc/passwd"},
		{"uniq with input and output file", "git log --oneline | uniq in.txt out.txt"},
		{"uniq dash operand", "git log --oneline | uniq -"},
		{"wc with file operand", "go test ./... | wc -l /etc/passwd"},
		{"wc dash operand", "go test ./... | wc -"},
		{"cut with file operand", "git log --oneline | cut -f1 /etc/passwd"},
		{"cut dash operand", "git log --oneline | cut -f1 -"},

		// Dangerous or unsupported sort flags.
		{"sort -o output file", "git status | sort -o /tmp/out"},
		{"sort clustered -uo output file", "git status | sort -uo /tmp/out"},
		{"sort --output=", "git status | sort --output=/tmp/out"},
		{"sort --output space", "git status | sort --output /tmp/out"},
		{"sort --files0-from", "git status | sort --files0-from=/etc/passwd"},
		{"sort --compress-program", "git status | sort --compress-program=sh"},
		{"sort -S buffer size", "git status | sort -S 10G"},
		{"sort --buffer-size", "git status | sort --buffer-size=10G"},
		{"sort -T temp dir", "git status | sort -T /tmp"},
		{"sort --random-source", "git status | sort --random-source=/dev/zero"},
		{"sort -z zero terminated", "git status | sort -z"},
		{"sort -k missing value", "git status | sort -k"},
		{"sort -t missing value", "git status | sort -t"},
		{"sort --check=bad", "git status | sort --check=bogus"},

		// Dangerous or invalid uniq flags.
		{"uniq -D unsupported", "git log --oneline | uniq -D"},
		{"uniq -f non-numeric", "git log --oneline | uniq -f bad"},
		{"uniq -w huge count", "git log --oneline | uniq -w 123456"},
		{"uniq -s negative", "git log --oneline | uniq -s -1"},
		{"uniq -f missing value", "git log --oneline | uniq -f"},

		// Dangerous or invalid wc flags.
		{"wc --files0-from", "go test ./... | wc --files0-from=list.txt"},
		{"wc --total", "go test ./... | wc --total=always"},

		// Invalid or incomplete cut forms.
		{"cut bare", "git log --oneline | cut"},
		{"cut -s without selector", "git log --oneline | cut -s"},
		{"cut --complement without selector", "git log --oneline | cut --complement"},
		{"cut -d without selector", "git log --oneline | cut -d ':'"},
		{"cut -f missing value", "git log --oneline | cut -f"},
		{"cut -d missing value", "git log --oneline | cut -d"},
		{"cut -f empty value", `git log --oneline | cut -f ""`},
		{"cut unknown flag", "git log --oneline | cut -z -f1"},

		// Protected families, non-allowlisted left side, double pipes, redirects, substitutions.
		{"file-read left side sort", "cat main.go | sort"},
		{"file-read left side wc", "cat main.go | wc -l"},
		{"file-read left side uniq", "cat main.go | uniq -c"},
		{"file-read left side cut", "cat main.go | cut -d: -f1"},
		{"wc left side sort", "wc -l a.go b.go | sort"},
		{"agent-desktop left side wc", "agent-desktop snapshot | wc -l"},
		{"non-allowlisted left side", "echo hi | sort"},
		{"triple pipe", "git log --oneline | sort | uniq -c | head -5"},
		{"redirect out", "git log --oneline | sort > out.txt"},
		{"background", "git log --oneline | wc -l &"},
		{"substitution in delimiter", `git log --oneline | cut -d "$(id)" -f1`},
		{"zsh rc_quotes", "git log --oneline | cut -d 'it''s' -f1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if d := Decide(tc.command, exe); d.Rewrite {
				t.Fatalf("Decide(%q) rewrote unexpectedly: %+v", tc.command, d)
			}
		})
	}
}

func TestWithSession_TextPipeWithSpecialDelimiter(t *testing.T) {
	const tag = "0123abcd"
	d := Decide(`git log --oneline | cut -d "' compact -- " -f1`, exe)
	if !d.Rewrite {
		t.Fatalf("Decide failed: %+v", d)
	}
	got := WithSession(d.Command, tag)
	if strings.Count(got, " compact --session="+tag+" --") != 1 {
		t.Fatalf("WithSession tagged wrong count: %s", got)
	}
	if !strings.Contains(got, `''\'' compact -- '`) {
		t.Fatalf("delimiter mutated by WithSession: %s", got)
	}
}

func TestGrammarExit_TextPipePreservesExitSemantics(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell semantics check")
	}
	stub, env := grammarFixture(t)
	for _, tc := range []struct {
		name string
		line string
		want int
	}{
		{"failing producer piped to wc exits 0", "go test ./... 2>&1 | wc -l", 0},
		{"failing producer piped to sort exits 0", "go test ./... 2>&1 | sort -u", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rewritten := grammarRewrite(t, stub, tc.line)
			orig := grammarExit(t, "sh", tc.line, env)
			got := grammarExit(t, "sh", rewritten, env)
			if orig != tc.want || got != orig {
				t.Fatalf("exit(%q) orig=%d rewritten=%d, want %d", tc.line, orig, got, tc.want)
			}
		})
	}
}
