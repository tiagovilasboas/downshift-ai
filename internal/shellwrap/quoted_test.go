// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package shellwrap

import (
	"errors"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/core"
	"github.com/tiagovilasboas/downshift-ai/internal/execwrap"
)

// A harness whose rewrite approves the command (Codex) or whose user rules
// are unknown (Cursor) still gets no quoted word, no 2>/dev/null and no
// || true rewritten, even around a read-only argv; Claude Code does.
func TestQuotedFormsStayOffTheApprovalPath(t *testing.T) {
	approving := rewritingHarness(t, true, true)
	unknown := rewritingHarness(t, false, true)
	plain := rewritingHarness(t, false, false)
	for _, command := range []string{`git log --grep "fix"`, `git status 2>/dev/null`, `git status || true`, `git diff 'a'`} {
		for _, h := range []string{approving, unknown} {
			withEnv(t, true, nil, true)
			if r := Handle(h, fakeShell{command: command}, ""); r.Rewrite != nil || r.Skip != ReasonNotReadOnly {
				t.Errorf("%s %q: rewrite=%v skip=%q, want no rewrite and %q", h, command, r.Rewrite, r.Skip, ReasonNotReadOnly)
			}
		}
		withEnv(t, true, nil, true)
		if r := Handle(plain, fakeShell{command: command}, "abc123"); r.Rewrite == nil {
			t.Errorf("%s %q: not rewritten (skip %q)", plain, command, r.Skip)
		}
	}
}

// A deny or ask rule written against the quoted command, against its argv as
// typed, or against the argv unquoted, stops the rewrite.
func TestQuotedCommandStoppedByUserRule(t *testing.T) {
	const command = `git commit -m "ship it" 2>/dev/null || true`
	for name, rule := range map[string]string{
		"whole command":   command,
		"argv as written": `git commit -m "ship it"`,
		"argv unquoted":   "git commit -m ship it",
	} {
		t.Run(name, func(t *testing.T) {
			withEnv(t, true, nil, true)
			blocked := func(text string) bool { return strings.HasPrefix(text, rule) }
			d := decide(core.ShellPOSIX, command, blocked, true)
			if d.Rewrite || execwrap.SkipReason(d, command) != execwrap.SkipDenied {
				t.Fatalf("decision %+v, want blocked_by_user_rules", d)
			}
		})
	}
	withEnv(t, true, nil, true)
	if d := decide(core.ShellPOSIX, command, never, true); !d.Rewrite || d.Family != execwrap.FamilyGitCommit {
		t.Fatalf("with no rule the quoted commit wraps as git-commit: %+v", d)
	}
}

// ruledShell is a shell call whose harness has the given deny/ask rules,
// matched as exact text the way a quote-holding Claude Code rule is.
type ruledShell struct {
	command string
	rules   []string
	err     error
}

func (f ruledShell) ShellCommand() (string, bool)             { return f.command, true }
func (f ruledShell) ShellRewrite(command string) (any, error) { return command, nil }
func (f ruledShell) ShellDenyAskRules() ([]string, error)     { return f.rules, f.err }
func (f ruledShell) ShellBlocked(command string) bool {
	for _, r := range f.rules {
		if r == "Bash("+command+")" {
			return true
		}
	}
	return false
}

// A deny or ask rule holding a quote blocks only when it can plausibly apply
// to the command being rewritten (same binary, unscoped rule, wildcard binary,
// or unreadable rule set). A quoted rule scoped to a different specific binary
// does not block unrelated quoted commands.
func TestQuotedRuleKeepsQuotedPathOff(t *testing.T) {
	quotedRule := []string{`Bash(git commit -m "wip")`}
	for _, command := range []string{`git commit -m 'wip'`, `git commit -m w"i"p`, `git commit -m wip 2>/dev/null`, `git status || true`} {
		withEnv(t, true, nil, true)
		if r := Handle("claude-code", ruledShell{command: command, rules: quotedRule}, ""); r.Rewrite != nil || r.Skip != ReasonQuotedRule {
			t.Errorf("%q under %q: rewrite=%v skip=%q, want %q", command, quotedRule, r.Rewrite, r.Skip, ReasonQuotedRule)
		}
		if r := Handle("claude-code", ruledShell{command: command, err: errors.New("unreadable")}, ""); r.Rewrite != nil || r.Skip != ReasonQuotedRule {
			t.Errorf("%q with unreadable rules: rewrite=%v skip=%q", command, r.Rewrite, r.Skip)
		}
		if r := Handle("claude-code", ruledShell{command: command, rules: []string{"Bash(gh repo delete:*)", "Bash(rm -rf /)"}}, ""); r.Rewrite == nil {
			t.Errorf("%q under quote-free rules: not rewritten (skip %q)", command, r.Skip)
		}
	}
	withEnv(t, true, nil, true)
	if r := Handle("claude-code", ruledShell{command: "go test ./...", rules: quotedRule}, ""); r.Rewrite == nil {
		t.Errorf("a command off the quoted path is not affected: skip %q", r.Skip)
	}
}

func TestQuotedRuleTargetedScoping(t *testing.T) {
	cases := []struct {
		name        string
		command     string
		rules       []string
		wantRewrite bool
		wantSkip    string
	}{
		{
			name:        "unrelated git quoted rule allows grep",
			command:     `grep -rn "foo bar" .`,
			rules:       []string{`Bash(git commit -m "wip")`},
			wantRewrite: true,
		},
		{
			name:        "unrelated git quoted rule allows go test",
			command:     `go test -run "TestA|TestB" ./...`,
			rules:       []string{`Bash(git commit -m "wip")`},
			wantRewrite: true,
		},
		{
			name:        "unrelated curl quoted rule allows find",
			command:     `find . -name "*.go"`,
			rules:       []string{`Bash(curl -d "secret" *)`},
			wantRewrite: true,
		},
		{
			name:        "matching binary path normalized",
			command:     `/usr/bin/git commit -m 'wip'`,
			rules:       []string{`Bash(git commit -m "wip")`},
			wantRewrite: false,
			wantSkip:    ReasonQuotedRule,
		},
		{
			name:        "rule with path matches bare binary",
			command:     `git commit -m 'wip'`,
			rules:       []string{`Bash(/usr/bin/git commit -m "wip")`},
			wantRewrite: false,
			wantSkip:    ReasonQuotedRule,
		},
		{
			name:        "wildcard quoted rule fails closed",
			command:     `grep -rn "foo" .`,
			rules:       []string{`Bash(* "secret")`},
			wantRewrite: false,
			wantSkip:    ReasonQuotedRule,
		},
		{
			name:        "globbed binary quoted rule fails closed",
			command:     `grep -rn "foo" .`,
			rules:       []string{`Bash(gr* "secret")`},
			wantRewrite: false,
			wantSkip:    ReasonQuotedRule,
		},
		{
			name:        "non-Bash quoted rule fails closed",
			command:     `grep -rn "foo" .`,
			rules:       []string{`Shell(git "wip")`},
			wantRewrite: false,
			wantSkip:    ReasonQuotedRule,
		},
		{
			name:        "chain containing matching binary blocks",
			command:     `go test ./... && git commit -m 'wip'`,
			rules:       []string{`Bash(git commit -m "wip")`},
			wantRewrite: false,
			wantSkip:    ReasonQuotedRule,
		},
		{
			name:        "chain without matching binary allows",
			command:     `go vet ./... && go test -run "A && B"`,
			rules:       []string{`Bash(git commit -m "wip")`},
			wantRewrite: true,
		},
		{
			name:        "compound pipe to matching binary blocks",
			command:     `go test ./... | grep "FAIL"`,
			rules:       []string{`Bash(grep "secret" *)`},
			wantRewrite: false,
			wantSkip:    ReasonQuotedRule,
		},
		{
			name:        "env-prefixed rule scoped to other binary allows",
			command:     `grep -rn "foo" .`,
			rules:       []string{`Bash(FOO="1" git commit -m "wip")`},
			wantRewrite: true,
		},
		{
			name:        "env-prefixed rule matching binary blocks",
			command:     `git commit -m 'wip'`,
			rules:       []string{`Bash(FOO="1" git commit -m "wip")`},
			wantRewrite: false,
			wantSkip:    ReasonQuotedRule,
		},
		{
			name:        "unclosed quote in rule fails closed",
			command:     `grep -rn "foo" .`,
			rules:       []string{`Bash(git commit -m "wip)`},
			wantRewrite: false,
			wantSkip:    ReasonQuotedRule,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withEnv(t, true, nil, true)
			r := Handle("claude-code", ruledShell{command: tc.command, rules: tc.rules}, "")
			if (r.Rewrite != nil) != tc.wantRewrite || r.Skip != tc.wantSkip {
				t.Fatalf("Handle(%q, %v) = rewrite=%v skip=%q, want rewrite=%v skip=%q",
					tc.command, tc.rules, r.Rewrite != nil, r.Skip, tc.wantRewrite, tc.wantSkip)
			}
		})
	}
}
