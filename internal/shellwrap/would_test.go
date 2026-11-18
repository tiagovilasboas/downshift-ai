// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package shellwrap

import "testing"

type mockRewriteEvent struct {
	command         string
	isShell         bool
	blocked         bool
	background      bool
	promptsSuppress bool
	rules           []string
	rulesErr        error
	rewriteCalled   *bool
}

func (m mockRewriteEvent) ShellCommand() (string, bool) { return m.command, m.isShell }
func (m mockRewriteEvent) ShellBlocked(string) bool     { return m.blocked }
func (m mockRewriteEvent) ShellRewrite(c string) (any, error) {
	if m.rewriteCalled != nil {
		*m.rewriteCalled = true
	}
	return c, nil
}
func (m mockRewriteEvent) ShellRunsInBackground() bool          { return m.background }
func (m mockRewriteEvent) ShellPromptsSuppressed() bool         { return m.promptsSuppress }
func (m mockRewriteEvent) ShellDenyAskRules() ([]string, error) { return m.rules, m.rulesErr }

func TestWouldRewriteEvent(t *testing.T) {
	plain := rewritingHarness(t, false, false) // claude-code
	cases := []struct {
		name    string
		harness string
		ev      func(called *bool) any
		want    bool
	}{
		{
			name:    "prompts suppressed blocks non-read-only command",
			harness: plain,
			ev: func(called *bool) any {
				return mockRewriteEvent{command: "go test ./...", isShell: true, promptsSuppress: true, rewriteCalled: called}
			},
			want: false,
		},
		{
			name:    "prompts suppressed still allows read-only command",
			harness: plain,
			ev: func(called *bool) any {
				return mockRewriteEvent{command: "git status", isShell: true, promptsSuppress: true, rewriteCalled: called}
			},
			want: true,
		},
		{
			name:    "prompts active allows non-read-only command",
			harness: plain,
			ev: func(called *bool) any {
				return mockRewriteEvent{command: "go test ./...", isShell: true, promptsSuppress: false, rewriteCalled: called}
			},
			want: true,
		},
		{
			name:    "background shell call is not rewritten",
			harness: plain,
			ev: func(called *bool) any {
				return mockRewriteEvent{command: "git status", isShell: true, background: true, rewriteCalled: called}
			},
			want: false,
		},
		{
			name:    "blocked shell call is not rewritten",
			harness: plain,
			ev: func(called *bool) any {
				return mockRewriteEvent{command: "git status", isShell: true, blocked: true, rewriteCalled: called}
			},
			want: false,
		},
		{
			name:    "quoted command refused under quoted rule",
			harness: plain,
			ev: func(called *bool) any {
				return mockRewriteEvent{command: `git commit -m "wip"`, isShell: true, rules: []string{`Bash(git commit -m "wip")`}, rewriteCalled: called}
			},
			want: false,
		},
		{
			name:    "non-shell call returns false",
			harness: plain,
			ev: func(called *bool) any {
				return mockRewriteEvent{command: "git status", isShell: false, rewriteCalled: called}
			},
			want: false,
		},
		{
			name:    "nil event returns false",
			harness: plain,
			ev:      func(*bool) any { return nil },
			want:    false,
		},
		{
			name:    "unknown harness returns false",
			harness: "no-such-harness",
			ev: func(called *bool) any {
				return mockRewriteEvent{command: "git status", isShell: true, rewriteCalled: called}
			},
			want: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withEnv(t, true, nil, true)
			var called bool
			got := WouldRewriteEvent(tc.harness, tc.ev(&called))
			if got != tc.want {
				t.Fatalf("WouldRewriteEvent = %v, want %v", got, tc.want)
			}
			if called {
				t.Fatal("WouldRewriteEvent invoked ShellRewrite")
			}
		})
	}
}
