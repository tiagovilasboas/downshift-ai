// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSkipLogHoldsNoCommandText(t *testing.T) {
	p := filepath.Join(t.TempDir(), SkipsFileName)
	secret := "/Users/someone/secret-project/bin/gh pr list --token abc123"
	d := Decide(secret, "/x/downshift")
	reason := SkipReason(d, secret)
	if reason != SkipAbsPath {
		t.Fatalf("reason = %q, want %q", reason, SkipAbsPath)
	}
	if err := AppendSkip(p, reason, "claude-code", time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := AppendSkip(p, "", "claude-code", time.Now()); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, leak := range []string{"secret", "abc123", "pr list", "/Users", "token"} {
		if strings.Contains(string(raw), leak) {
			t.Errorf("skip row leaks %q: %s", leak, raw)
		}
	}
	got, err := SummarizeSkips(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[SkipAbsPath] != 1 {
		t.Errorf("counts = %v, want one abs_path", got)
	}
}

func TestSummarizeSkipsMissingFile(t *testing.T) {
	got, err := SummarizeSkips(filepath.Join(t.TempDir(), "none.jsonl"))
	if err != nil || got != nil {
		t.Errorf("got %v, %v; want nil, nil", got, err)
	}
}

func TestSkipDetailNonMetacharPopulatesFamilyWithoutLeaking(t *testing.T) {
	cases := []struct {
		name       string
		reason     string
		command    string
		wantFamily string
	}{
		{"not_read_only go test", "not_read_only", "go test ./secret/pkg", string(FamilyGoTest)},
		{"not_read_only quoted git commit", "not_read_only", `git commit -m "secret msg"`, string(FamilyGitCommit)},
		{"abs_path known bin", SkipAbsPath, "/usr/bin/gh pr list --repo secret/repo", string(FamilyGhPr)},
		{"abs_path custom bin", SkipAbsPath, "/Users/someone/secret-project/bin/gh pr list --token abc123", string(FamilyGhPr)},
		{"denied", SkipDenied, "FOO=secret go test ./...", string(FamilyGoTest)},
		{"too_long", ReasonTooLong, "git diff --secret-flag", string(FamilyGitDiff)},
		{"background", "background", "go test ./secret &", string(FamilyGoTest)},
		{"disabled", SkipDisabled, "cargo test --package secret", string(FamilyCargoTest)},
		{"unknown command", SkipNotAllowlisted, "vim /secret/file.txt", ""},
		{"leading subst stays empty", SkipDisabled, "$(evil) go test", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			class, fam := SkipDetail(tc.reason, tc.command)
			if class != "" {
				t.Errorf("SkipDetail(%q, %q) class = %q, want empty", tc.reason, tc.command, class)
			}
			if fam != tc.wantFamily {
				t.Errorf("SkipDetail(%q, %q) family = %q, want %q", tc.reason, tc.command, fam, tc.wantFamily)
			}
			p := filepath.Join(t.TempDir(), SkipsFileName)
			if err := AppendSkipEvent(p, SkipEvent{Reason: tc.reason, Harness: "claude-code", Metachar: class, Family: fam}, time.Now()); err != nil {
				t.Fatal(err)
			}
			raw, err := os.ReadFile(p)
			if err != nil {
				t.Fatal(err)
			}
			for _, leak := range []string{"secret", "abc123", "/Users", "/usr/bin", "evil"} {
				if strings.Contains(string(raw), leak) {
					t.Errorf("skip row leaks %q: %s", leak, raw)
				}
			}
		})
	}
}
