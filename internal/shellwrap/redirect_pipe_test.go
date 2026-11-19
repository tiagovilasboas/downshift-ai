// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package shellwrap

import (
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/execwrap"
)

func TestHandle_RedirectsAndGrepPipes(t *testing.T) {
	plain := rewritingHarness(t, false, false)   // claude-code
	approving := rewritingHarness(t, true, true) // codex
	unknown := rewritingHarness(t, false, true)  // cursor

	wrapCases := []struct {
		name    string
		command string
		want    string
	}{
		{"trailing 2>&1", "go test ./... 2>&1", "'" + testExe + "' compact --harness=claude-code -- go test ./... 2>&1"},
		{"mid-command 2>&1", "go test 2>&1 ./...", "'" + testExe + "' compact --harness=claude-code -- go test ./... 2>&1"},
		{"2>/dev/null", "git status 2>/dev/null", "'" + testExe + "' compact --harness=claude-code --discard-stderr -- 'git' 'status'"},
		{"2> /dev/null", "git status 2> /dev/null", "'" + testExe + "' compact --harness=claude-code --discard-stderr -- 'git' 'status'"},
		{"pipe grep", "go test ./... | grep FAIL", "'" + testExe + "' compact --harness=claude-code -- go test ./... | grep FAIL"},
		{"pipe rg", "go test ./... 2>&1 | rg -i error", "'" + testExe + "' compact --harness=claude-code -- go test ./... 2>&1 | rg -i error"},
	}
	for _, tc := range wrapCases {
		t.Run(tc.name, func(t *testing.T) {
			withEnv(t, true, nil, true)
			r := Handle(plain, fakeShell{command: tc.command}, "")
			if r.Rewrite != tc.want {
				t.Fatalf("Handle(%q) rewrite = %v, want %q (skip=%q)", tc.command, r.Rewrite, tc.want, r.Skip)
			}
			if !WouldRewrite(plain, tc.command) {
				t.Fatalf("WouldRewrite(%q) = false, want true", tc.command)
			}
		})
	}

	// Approving / unknown-rules harnesses still refuse non-read-only & metachar shapes.
	for _, cmd := range []string{"git status 2>&1", "git status 2>/dev/null", "git status | grep clean", "go test ./... | rg FAIL"} {
		for _, h := range []string{approving, unknown} {
			withEnv(t, true, nil, true)
			if r := Handle(h, fakeShell{command: cmd}, ""); r.Rewrite != nil || r.Skip != ReasonNotReadOnly {
				t.Fatalf("%s Handle(%q): rewrite=%v skip=%q, want nil / %q", h, cmd, r.Rewrite, r.Skip, ReasonNotReadOnly)
			}
			if WouldRewrite(h, cmd) {
				t.Fatalf("%s WouldRewrite(%q) = true, want false", h, cmd)
			}
		}
	}

	// Deny rule on grep/rg segment stops pipe rewrite; quoted rule stops quoted pipe pattern.
	withEnv(t, true, nil, true)
	if r := Handle("claude-code", ruledShell{command: "go test ./... | grep FAIL", rules: []string{"Bash(grep FAIL)"}}, ""); r.Rewrite != nil || r.Skip != execwrap.SkipDenied {
		t.Fatalf("Bash(grep FAIL) must deny pipe: %+v", r)
	}
	if r := Handle("claude-code", ruledShell{command: `go test ./... | rg "FAIL"`, rules: []string{`Bash(git commit -m "wip")`}}, ""); r.Rewrite != nil || r.Skip != ReasonQuotedRule {
		t.Fatalf("quoted rule must stop quoted pipe pattern: %+v", r)
	}
}
