// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package shellwrap

import (
	"errors"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/core"
	"github.com/tiagovilasboas/downshift-ai/internal/execwrap"
)

const testExe = "/opt/down shift/bin/downshift"

// withEnv pins the three inputs decide reads: the config flag, the
// environment and whether the downshift executable resolves.
func withEnv(t *testing.T, enabled bool, env map[string]string, onPath bool) {
	t.Helper()
	oldEnabled, oldExe, oldEnv := configEnabled, resolveExecutable, getenv
	SetEnvFuncs(
		func() bool { return enabled },
		func() (string, error) {
			if onPath {
				return testExe, nil
			}
			return "", errors.New("not executable")
		},
		func(k string) string { return env[k] },
	)
	t.Cleanup(func() { configEnabled, resolveExecutable, getenv = oldEnabled, oldExe, oldEnv })
}

func never(string) bool { return false }

func TestDecide(t *testing.T) {
	cases := []struct {
		name     string
		enabled  bool
		env      map[string]string
		onPath   bool
		command  string
		blocked  func(string) bool
		cap      bool
		rewrite  bool
		reason   string
		familyGo bool
	}{
		{name: "allowlisted command on a capable harness", enabled: true, onPath: true, command: "go test ./...", blocked: never, cap: true, rewrite: true, familyGo: true},
		{name: "depth guard: already inside the compactor", enabled: true, env: map[string]string{execwrap.DepthEnv: "1"}, onPath: true, command: "go test ./...", blocked: never, cap: true, reason: "disabled"},
		{name: "kill switch", enabled: true, env: map[string]string{execwrap.KillSwitchEnv: "0"}, onPath: true, command: "go test ./...", blocked: never, cap: true, reason: "disabled"},
		{name: "config turns it off", enabled: false, onPath: true, command: "go test ./...", blocked: never, cap: true, reason: "disabled"},
		{name: "executable missing", enabled: true, onPath: false, command: "go test ./...", blocked: never, cap: true, reason: "executable_unavailable"},
		{name: "harness cannot rewrite", enabled: true, onPath: true, command: "go test ./...", blocked: never, cap: false, reason: "harness_cannot_rewrite", familyGo: true},
		{name: "no rule reader blocks", enabled: true, onPath: true, command: "go test ./...", blocked: nil, cap: true, reason: "blocked_by_user_rules", familyGo: true},
		{name: "rule on the whole command", enabled: true, onPath: true, command: "go test ./...", blocked: func(s string) bool { return s == "go test ./..." }, cap: true, reason: "blocked_by_user_rules", familyGo: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			withEnv(t, c.enabled, c.env, c.onPath)
			d := decide(core.ShellPOSIX, c.command, c.blocked, c.cap)
			if d.Rewrite != c.rewrite {
				t.Fatalf("rewrite = %v, want %v (decision %+v)", d.Rewrite, c.rewrite, d)
			}
			if !c.rewrite && d.Reason != c.reason {
				t.Fatalf("reason = %q, want %q", d.Reason, c.reason)
			}
			if c.familyGo && d.Family != execwrap.FamilyGoTest {
				t.Fatalf("family = %q, want %q", d.Family, execwrap.FamilyGoTest)
			}
		})
	}
}

// A mixed chain is wrapped per segment on a harness that can rewrite; a rule on
// a kept segment, the kill switch or a harness that cannot rewrite stops it.
func TestDecideMixedChain(t *testing.T) {
	cmd := "cd dir && go build ./... && go test ./pkg/..."
	withEnv(t, true, nil, true)
	d := decide(core.ShellPOSIX, cmd, never, true)
	w := "'" + testExe + "' compact -- "
	if want := "cd dir && " + w + "go build ./... && " + w + "go test ./pkg/..."; !d.Rewrite || d.Command != want {
		t.Fatalf("decide = %+v, want command %q", d, want)
	}
	if got := decide(core.ShellPOSIX, cmd, func(s string) bool { return s == "cd dir" }, true); got.Rewrite || got.Reason != "blocked_by_user_rules" {
		t.Fatalf("rule on a kept segment must block, got %+v", got)
	}
	if got := decide(core.ShellPOSIX, cmd, never, false); got.Rewrite {
		t.Fatalf("harness that cannot rewrite got %+v", got)
	}
	withEnv(t, true, map[string]string{execwrap.KillSwitchEnv: "0"}, true)
	if got := decide(core.ShellPOSIX, cmd, never, true); got.Rewrite || got.Reason != "disabled" {
		t.Fatalf("kill switch got %+v", got)
	}
}

func TestDecideRefusesWhenOneChainSegmentIsBlocked(t *testing.T) {
	withEnv(t, true, nil, true)
	cmd := "go test ./... && go vet ./..."
	d := execwrap.Decide(cmd, testExe)
	if !d.Rewrite || len(d.Segments) == 0 {
		t.Fatalf("precondition: execwrap must rewrite %q as a chain, got %+v", cmd, d)
	}
	segment := d.Segments[len(d.Segments)-1]
	if ok := decide(core.ShellPOSIX, cmd, never, true); !ok.Rewrite {
		t.Fatalf("precondition: with no rule the chain is rewritten, got %+v", ok)
	}
	got := decide(core.ShellPOSIX, cmd, func(s string) bool { return s == segment }, true)
	if got.Rewrite || got.Reason != "blocked_by_user_rules" {
		t.Fatalf("a rule on segment %q must block the rewrite, got %+v", segment, got)
	}
}

func TestHandleUnpromptedExpandedReadOnly(t *testing.T) {
	plain := rewritingHarness(t, false, false) // claude-code
	cases := []struct {
		command string
		rewrite bool
		skip    string
	}{
		{"git -C repo status --short", true, ""},
		{"git -C repo branch -a", true, ""},
		{"git -C repo remote -v", true, ""},
		{"git -C repo stash list", true, ""},
		{"git tag -l --sort=-v:refname", true, ""},
		{"git config --list --show-origin", true, ""},
		{"git grep -n func internal", true, ""},
		{"ls", true, ""},
		{"docker ps -a", true, ""},
		{"docker images", true, ""},
		{"docker logs --tail 20 app", true, ""},
		{"git -C repo branch -D main", false, ReasonNotReadOnly},
		{"git tag v1.0", false, ReasonNotReadOnly},
		{"git config --set user.name x", false, ReasonNotReadOnly},
		{"git grep -O vim func", false, ReasonNotReadOnly},
		{"docker build .", false, ReasonNotReadOnly},
		{"docker exec app ls", false, ReasonNotReadOnly},
		{"kubectl get pods", false, ReasonNotReadOnly},
	}
	for _, tc := range cases {
		t.Run(tc.command, func(t *testing.T) {
			withEnv(t, true, nil, true)
			ev := shapedShell{fakeShell: fakeShell{command: tc.command}, unprompted: true}
			r := Handle(plain, ev, "")
			if (r.Rewrite != nil) != tc.rewrite || r.Skip != tc.skip {
				t.Fatalf("Handle(%q) rewrite=%v skip=%q, want rewrite=%v skip=%q",
					tc.command, r.Rewrite != nil, r.Skip, tc.rewrite, tc.skip)
			}
		})
	}
}
