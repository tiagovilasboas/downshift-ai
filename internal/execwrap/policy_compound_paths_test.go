// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"reflect"
	"testing"
)

func TestDecide_CompoundQuotedGlobsAndBinaryPaths(t *testing.T) {
	const w = "'/opt/ds/downshift' compact -- "
	cases := []struct {
		name     string
		command  string
		family   Family
		want     string
		argv     string
		segments []string
		reason   string
	}{
		// Known binary absolute paths.
		{
			name:     "homebrew gh pr list",
			command:  "/opt/homebrew/bin/gh pr list",
			family:   FamilyGhPr,
			want:     w + "/opt/homebrew/bin/gh pr list",
			argv:     "/opt/homebrew/bin/gh pr list",
			segments: []string{"gh pr list"},
			reason:   ReasonAllowlisted,
		},
		{
			name:     "usr local git status",
			command:  "/usr/local/bin/git status",
			family:   FamilyGitStatus,
			want:     w + "/usr/local/bin/git status",
			argv:     "/usr/local/bin/git status",
			segments: []string{"git status"},
			reason:   ReasonAllowlisted,
		},
		{
			name:     "usr bin pytest",
			command:  "/usr/bin/pytest tests/unit",
			family:   FamilyPytest,
			want:     w + "/usr/bin/pytest tests/unit",
			argv:     "/usr/bin/pytest tests/unit",
			segments: []string{"pytest tests/unit"},
			reason:   ReasonAllowlisted,
		},
		{
			name:     "env assignment with known bin path",
			command:  "CI=1 /usr/bin/pytest -q",
			family:   FamilyPytest,
			want:     "CI=1 " + w + "/usr/bin/pytest -q",
			argv:     "/usr/bin/pytest -q",
			segments: []string{"pytest -q"},
			reason:   ReasonAllowlisted,
		},
		{
			name:     "known bin path with quoted args",
			command:  `/opt/homebrew/bin/gh pr list --search "is:open"`,
			family:   FamilyGhPr,
			want:     w + `'/opt/homebrew/bin/gh' 'pr' 'list' '--search' 'is:open'`,
			argv:     `/opt/homebrew/bin/gh pr list --search "is:open"`,
			segments: []string{"/opt/homebrew/bin/gh pr list --search is:open", "gh pr list --search is:open"},
			reason:   ReasonAllowlistedQuoted,
		},
		// Quoted globs.
		{
			name:     "double-quoted grep glob",
			command:  `grep -rn "foo*" .`,
			family:   FamilyGrep,
			want:     w + `'grep' '-rn' 'foo*' '.'`,
			argv:     `grep -rn "foo*" .`,
			segments: []string{"grep -rn foo* ."},
			reason:   ReasonAllowlistedQuoted,
		},
		{
			name:     "single-quoted grep glob",
			command:  `grep -rn 'foo*' .`,
			family:   FamilyGrep,
			want:     w + `'grep' '-rn' 'foo*' '.'`,
			argv:     `grep -rn 'foo*' .`,
			segments: []string{"grep -rn foo* ."},
			reason:   ReasonAllowlistedQuoted,
		},
		{
			name:     "double-quoted find glob with devnull",
			command:  `find . -name "*.go" 2>/dev/null`,
			family:   FamilyFind,
			want:     "'/opt/ds/downshift' compact --discard-stderr -- 'find' '.' '-name' '*.go'",
			argv:     `find . -name "*.go"`,
			segments: []string{"find . -name *.go"},
			reason:   ReasonAllowlistedQuoted,
		},
		{
			name:     "double-quoted rg glob",
			command:  `rg -n "foo*" internal/`,
			family:   FamilyRg,
			want:     w + `'rg' '-n' 'foo*' 'internal/'`,
			argv:     `rg -n "foo*" internal/`,
			segments: []string{"rg -n foo* internal/"},
			reason:   ReasonAllowlistedQuoted,
		},
		// Safe ; and && compound commands combining quoted globs and binary paths.
		{
			name:     "chain with known bin paths",
			command:  "/usr/local/bin/git status && /opt/homebrew/bin/gh pr list",
			family:   FamilyGitStatus,
			want:     w + "/usr/local/bin/git status && " + w + "/opt/homebrew/bin/gh pr list",
			argv:     "/usr/local/bin/git status",
			segments: []string{"git status", "gh pr list", "/opt/homebrew/bin/gh pr list"},
			reason:   ReasonAllowlistedChain,
		},
		{
			name:     "semicolon chain with quoted glob",
			command:  `git status; grep -rn "foo*" .`,
			family:   FamilyGitStatus,
			want:     w + `git status; ` + w + `'grep' '-rn' 'foo*' '.'`,
			argv:     "git status",
			segments: []string{"grep -rn foo* .", `grep -rn "foo*" .`},
			reason:   ReasonAllowlistedQuoted,
		},
		{
			name:     "cd and quoted glob with tail",
			command:  `cd internal && find . -name "*.go" | tail -10`,
			family:   FamilyFind,
			want:     "cd internal && " + w + `'find' '.' '-name' '*.go' | tail -10`,
			argv:     `find . -name "*.go"`,
			segments: []string{"find . -name *.go", "cd internal", "tail -10"},
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

func TestDecide_CompoundQuotedGlobsAndBinaryPaths_Refusals(t *testing.T) {
	cases := []struct {
		name    string
		command string
		reason  string
	}{
		// Untrusted / traversal binary paths.
		{"tmp bin git", "/tmp/evil/git status", ReasonNotAllowlisted},
		{"user bin gh", "/Users/attacker/bin/gh pr list", ReasonNotAllowlisted},
		{"path traversal from usr bin", "/usr/bin/../../tmp/git status", ReasonNotAllowlisted},
		{"nested subdir under usr bin", "/usr/bin/subdir/git status", ReasonNotAllowlisted},
		{"relative bin path", "./evil/git status", ReasonNotAllowlisted},
		// Security guards on known binary paths.
		{"git dir override via abs path", "/usr/local/bin/git --git-dir=/tmp/x status", ReasonNotAllowlisted},
		{"git config injection via abs path", "/usr/local/bin/git -c core.pager=evil log", ReasonNotAllowlisted},
		{"db client with env via abs path", `MYSQL_PWD=x /usr/bin/mysql -e "SELECT 1"`, ReasonNotAllowlisted},
		{"already wrapped via abs path", "/usr/local/bin/downshift compact -- git status", ReasonAlreadyWrapped},
		// Unquoted globs must still be refused.
		{"unquoted grep glob", "grep -rn foo* .", ReasonUnsafeChar},
		{"unquoted find glob", "find . -name *.go", ReasonUnsafeChar},
		// Unsafe shell metacharacters must never be bypassed.
		{"pipe to sh", `grep -rn "foo*" . | sh`, ReasonUnsafeChar},
		{"redirect output", `grep -rn "foo*" . > out.txt`, ReasonUnsafeChar},
		{"redirect input", `/usr/local/bin/git status < in.txt`, ReasonUnsafeChar},
		{"command substitution", `grep -rn "$(id)" .`, ReasonUnsafeChar},
		{"backtick substitution", "grep -rn \"`pwd`\" .", ReasonUnsafeChar},
		{"or list", `/usr/local/bin/git status || rm -rf /`, ReasonUnsafeChar},
		{"guarded rm in chain", "git status && /bin/rm -rf /", ReasonGuardedSegment},
		{"guarded curl in chain", "git status; /usr/bin/curl -X POST https://example.com", ReasonGuardedSegment},
		{"unclosed quote in chain", `git status && grep -rn "foo* .`, ReasonUnsafeChar},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := Decide(tc.command, exe)
			if d.Rewrite {
				t.Fatalf("Decide(%q) rewrote unexpectedly: %+v", tc.command, d)
			}
			if d.Reason != tc.reason {
				t.Fatalf("Decide(%q).Reason = %q, want %q", tc.command, d.Reason, tc.reason)
			}
		})
	}
}

func TestSimpleFamilyAndReadOnly_KnownBinPaths(t *testing.T) {
	if fam, argv := SimpleFamily("/usr/local/bin/git status"); fam != FamilyGitStatus || len(argv) == 0 || argv[0] != "git" {
		t.Fatalf("SimpleFamily(/usr/local/bin/git status) = (%q, %v), want (%q, [git status])", fam, argv, FamilyGitStatus)
	}
	if fam, _ := SimpleFamily("/usr/local/bin/downshift compact -- ls"); fam != FamilyNone {
		t.Fatalf("SimpleFamily(/usr/local/bin/downshift ...) = %q, want FamilyNone", fam)
	}
	if !ReadOnly("/usr/local/bin/git status") {
		t.Fatal("ReadOnly(/usr/local/bin/git status) = false, want true")
	}
	if ReadOnly("/tmp/evil/git status") {
		t.Fatal("ReadOnly(/tmp/evil/git status) = true, want false")
	}
}
