// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestDecide_SafeStderrRedirectsAndGrepPipes(t *testing.T) {
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
		// Harmless stderr-only redirects: 2>&1 and 2>/dev/null (including space and mid-command).
		{
			name:    "trailing 2>&1",
			command: "go test ./... 2>&1",
			family:  FamilyGoTest,
			want:    w + "go test ./... 2>&1",
			argv:    "go test ./...",
			reason:  ReasonAllowlistedCompound,
		},
		{
			name:    "mid-command 2>&1",
			command: "go test 2>&1 ./...",
			family:  FamilyGoTest,
			want:    w + "go test ./... 2>&1",
			argv:    "go test ./...",
			reason:  ReasonAllowlistedCompound,
		},
		{
			name:    "leading 2>&1",
			command: "2>&1 git status",
			family:  FamilyGitStatus,
			want:    w + "git status 2>&1",
			argv:    "git status",
			reason:  ReasonAllowlistedCompound,
		},
		{
			name:    "trailing 2>/dev/null",
			command: "git status 2>/dev/null",
			family:  FamilyGitStatus,
			want:    wd + "'git' 'status'",
			argv:    "git status",
			reason:  ReasonAllowlistedQuoted,
		},
		{
			name:    "trailing 2> /dev/null with space",
			command: "git status 2> /dev/null",
			family:  FamilyGitStatus,
			want:    wd + "'git' 'status'",
			argv:    "git status",
			reason:  ReasonAllowlistedQuoted,
		},
		{
			name:    "mid-command 2>/dev/null",
			command: "git log 2>/dev/null -n 5",
			family:  FamilyGitLog,
			want:    wd + "'git' 'log' '-n' '5'",
			argv:    "git log -n 5",
			reason:  ReasonAllowlistedQuoted,
		},
		{
			name:    "mid-command 2> /dev/null with || true",
			command: "go test 2> /dev/null ./... || true",
			family:  FamilyGoTest,
			want:    "'" + exe + "' compact --discard-stderr --exit-zero -- 'go' 'test' './...'",
			argv:    "go test ./...",
			reason:  ReasonAllowlistedQuoted,
		},
		{
			name:    "quoted argument containing 2>&1 is literal",
			command: `git commit -m "fix 2>&1"`,
			family:  FamilyGitCommit,
			want:    w + "'git' 'commit' '-m' 'fix 2>&1'",
			argv:    `git commit -m "fix 2>&1"`,
			segments: []string{
				"git commit -m fix 2>&1",
			},
			reason: ReasonAllowlistedQuoted,
		},

		// Safe trailing | grep and | rg pipelines.
		{
			name:     "pipe grep bare",
			command:  "go test ./... | grep FAIL",
			family:   FamilyGoTest,
			want:     w + "go test ./... | grep FAIL",
			argv:     "go test ./...",
			segments: []string{"grep FAIL"},
			reason:   ReasonAllowlistedCompound,
		},
		{
			name:     "pipe grep flags and context",
			command:  "go test ./... | grep -in -A 3 -B2 -m 10 FAIL",
			family:   FamilyGoTest,
			want:     w + "go test ./... | grep -in -A 3 -B2 -m 10 FAIL",
			argv:     "go test ./...",
			segments: []string{"grep -in -A 3 -B2 -m 10 FAIL"},
			reason:   ReasonAllowlistedCompound,
		},
		{
			name:     "pipe grep explicit -e patterns",
			command:  "go test ./... | grep -e FAIL -e panic",
			family:   FamilyGoTest,
			want:     w + "go test ./... | grep -e FAIL -e panic",
			argv:     "go test ./...",
			segments: []string{"grep -e FAIL -e panic"},
			reason:   ReasonAllowlistedCompound,
		},
		{
			name:     "pipe grep double-dashed pattern",
			command:  "git diff | grep -- -old",
			family:   FamilyGitDiff,
			want:     w + "git diff | grep -- -old",
			argv:     "git diff",
			segments: []string{"grep -- -old"},
			reason:   ReasonAllowlistedCompound,
		},
		{
			name:     "pipe grep quoted alternation",
			command:  `go test ./... | grep -E "FAIL|panic"`,
			family:   FamilyGoTest,
			want:     w + `go test ./... | 'grep' '-E' 'FAIL|panic'`,
			argv:     "go test ./...",
			segments: []string{`grep -E "FAIL|panic"`, "grep -E FAIL|panic"},
			reason:   ReasonAllowlistedQuoted,
		},
		{
			name:     "pipe grep quoted pipe char inside pattern",
			command:  `go test ./... | grep -E "a | b"`,
			family:   FamilyGoTest,
			want:     w + `go test ./... | 'grep' '-E' 'a | b'`,
			argv:     "go test ./...",
			segments: []string{`grep -E "a | b"`, "grep -E a | b"},
			reason:   ReasonAllowlistedQuoted,
		},
		{
			name:     "pipe rg",
			command:  `go test ./... | rg -n -i --no-heading "error|fail"`,
			family:   FamilyGoTest,
			want:     w + `go test ./... | 'rg' '-n' '-i' '--no-heading' 'error|fail'`,
			argv:     "go test ./...",
			segments: []string{`rg -n -i --no-heading "error|fail"`, "rg -n -i --no-heading error|fail"},
			reason:   ReasonAllowlistedQuoted,
		},
		{
			name:     "pipe egrep and fgrep",
			command:  "git log --oneline | egrep --color=never fix",
			family:   FamilyGitLog,
			want:     w + "git log --oneline | egrep --color=never fix",
			argv:     "git log --oneline",
			segments: []string{"egrep --color=never fix"},
			reason:   ReasonAllowlistedCompound,
		},
		{
			name:     "2>&1 before pipe rg",
			command:  "go test ./... 2>&1 | rg -C 2 FAIL",
			family:   FamilyGoTest,
			want:     w + "go test ./... 2>&1 | rg -C 2 FAIL",
			argv:     "go test ./...",
			segments: []string{"rg -C 2 FAIL"},
			reason:   ReasonAllowlistedCompound,
		},
		{
			name:     "2>/dev/null before pipe grep",
			command:  `find . -name "*.go" 2>/dev/null | grep test`,
			family:   FamilyFind,
			want:     wd + `'find' '.' '-name' '*.go' | grep test`,
			argv:     `find . -name "*.go"`,
			segments: []string{"find . -name *.go", "grep test"},
			reason:   ReasonAllowlistedQuoted,
		},
		{
			name:     "cd and 2>&1 and pipe grep",
			command:  "cd internal && go test ./... 2>&1 | grep FAIL",
			family:   FamilyGoTest,
			want:     "cd internal && " + w + "go test ./... 2>&1 | grep FAIL",
			argv:     "go test ./...",
			segments: []string{"cd internal", "grep FAIL"},
			reason:   ReasonAllowlistedCompound,
		},
		{
			name:     "pipe grep with trailing 2>/dev/null",
			command:  "go test ./... | grep FAIL 2>/dev/null",
			family:   FamilyGoTest,
			want:     w + "go test ./... | grep FAIL 2>/dev/null",
			argv:     "go test ./...",
			segments: []string{"grep FAIL", "grep FAIL 2>/dev/null"},
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

func TestDecide_RefusesUnsafeRedirectsAndPipes(t *testing.T) {
	cases := []struct {
		name    string
		command string
	}{
		// Unsafe redirects must remain strictly refused.
		{"stdout write", "go test ./... > out.txt"},
		{"stdout append", "go test ./... >> out.txt"},
		{"fd1 write", "go test ./... 1> out.txt"},
		{"stderr to file", "go test ./... 2> err.txt"},
		{"stderr append to file", "go test ./... 2>> err.txt"},
		{"combined redirect", "go test ./... &> out.txt"},
		{"combined devnull", "go test ./... &>/dev/null"},
		{"stdout devnull", "go test ./... >/dev/null"},
		{"fd1 devnull", "go test ./... 1>/dev/null"},
		{"stdout to stderr", "go test ./... 1>&2"},
		{"gt-amp stderr", "go test ./... >&2"},
		{"stdin redirect", "go test ./... < in.txt"},
		{"here-string", `go test ./... <<< "hi"`},
		{"heredoc", "cat <<EOF"},
		{"2>&1 plus file redirect", "go test ./... 2>&1 > out.txt"},
		{"file redirect plus 2>&1", "go test ./... > out.txt 2>&1"},
		{"2>/dev/null plus file redirect", "go test ./... 2>/dev/null > out.txt"},
		{"two 2>&1 redirects", "go test ./... 2>&1 2>&1"},
		{"two 2>/dev/null redirects", "go test 2>/dev/null ./... 2>/dev/null"},
		{"2>&1 then 2>/dev/null", "go test ./... 2>&1 2>/dev/null"},
		{"2>/dev/null then 2>&1", "go test ./... 2>/dev/null 2>&1"},
		{"glued 2>&1", "go test ./...2>&1"},
		{"glued trailing after 2>&1", "go test ./... 2>&1foo"},
		{"glued 2>/dev/null", "go test ./...2>/dev/null"},
		{"glued trailing after 2>/dev/null", "go test ./... 2>/dev/null2"},
		{"|| true before 2> /dev/null", "go test ./... || true 2> /dev/null"},
		{"2>&1 before || true", "go test ./... 2>&1 || true"},

		// Dangerous pipes, flags, operands, and substitutions must remain strictly refused.
		{"grep with file operand", "go test ./... | grep FAIL /etc/passwd"},
		{"rg with file operand", "go test ./... | rg FAIL /etc/passwd"},
		{"grep -e plus file operand", "go test ./... | grep -e FAIL /etc/passwd"},
		{"grep -f pattern file", "go test ./... | grep -f /etc/passwd"},
		{"grep --file pattern file", "go test ./... | grep --file=/etc/passwd"},
		{"grep recursive flag", "go test ./... | grep -r FAIL"},
		{"rg --pre command execution", "go test ./... | rg --pre cat FAIL"},
		{"rg -z decompressor spawn", "go test ./... | rg -z FAIL"},
		{"grep bare without pattern", "go test ./... | grep"},
		{"grep trailing -e without value", "go test ./... | grep -e"},
		{"grep bare dash operand", "go test ./... | grep -"},
		{"grep non-numeric -A", "go test ./... | grep -A bad FAIL"},
		{"grep huge -A count", "go test ./... | grep -A 123456 FAIL"},
		{"grep ambiguous --color without =", "go test ./... | grep --color FAIL"},
		{"grep substitution in pattern", `go test ./... | grep "$(id)"`},
		{"grep backtick in pattern", "go test ./... | grep \"`id`\""},
		{"grep unquoted dollar", "go test ./... | grep $HOME"},
		{"grep redirect out", "go test ./... | grep FAIL > out.txt"},
		{"grep redirect append", "go test ./... | grep FAIL >> out.txt"},
		{"grep redirect stdin", "go test ./... | grep FAIL < in.txt"},
		{"grep then second pipe", "go test ./... | grep FAIL | wc -l"},
		{"shell stage before grep", "go test ./... | sh | grep FAIL"},
		{"grep background", "go test ./... | grep FAIL &"},
		{"grep zsh rc_quotes", "go test ./... | grep 'it''s'"},
		{"grep zsh =cmd", "go test ./... | grep =foo"},
		{"non-allowlisted left side", "echo hi | grep hi"},
		{"file-read left side stays unwrapped", "cat main.go | grep func"},
		{"non-grep pipe stays refused", "go test ./... | awk '{print}'"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if d := Decide(tc.command, exe); d.Rewrite {
				t.Fatalf("Decide(%q) rewrote unexpectedly: %+v", tc.command, d)
			}
		})
	}
}

func TestWithSession_GrepPipeWithSpecialPattern(t *testing.T) {
	const tag = "0123abcd"
	d := Decide(`go test ./... | grep "' compact -- "`, exe)
	if !d.Rewrite {
		t.Fatalf("Decide failed: %+v", d)
	}
	got := WithSession(d.Command, tag)
	if strings.Count(got, " compact --session="+tag+" --") != 1 {
		t.Fatalf("WithSession tagged wrong count: %s", got)
	}
	if !strings.Contains(got, `''\'' compact -- '`) {
		t.Fatalf("pattern mutated by WithSession: %s", got)
	}
}

func TestMetacharClass_ExactIndexAfterMidDiscardRedirect(t *testing.T) {
	cmd := "go test 2>/dev/null ./... > out.txt"
	if got := MetacharClass(cmd); got != MetaRedirect {
		t.Fatalf("MetacharClass(%q) = %q, want %q", cmd, got, MetaRedirect)
	}
	class, fam := SkipDetail(SkipMetachar, cmd)
	if class != MetaRedirect || fam != string(FamilyGoTest) {
		t.Fatalf("SkipDetail(%q) = (%q, %q), want (%q, %q)", cmd, class, fam, MetaRedirect, FamilyGoTest)
	}
}

func TestGrammarExit_GrepPipePreservesExitSemantics(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell semantics check")
	}
	stub, env := grammarFixture(t)
	for _, tc := range []struct {
		name string
		line string
		want int
	}{
		{"matching grep exits 0", "go test ./... 2>&1 | grep FAIL", 0},
		{"non-matching grep exits 1", "go test ./... 2>/dev/null | grep NOMATCH", 1},
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
