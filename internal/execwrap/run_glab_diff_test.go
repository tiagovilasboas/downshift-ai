// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package execwrap

import (
	"bytes"
	"strings"
	"testing"
)

func TestRun_SmallGitDiffAndShowPassThroughRaw(t *testing.T) {
	for _, tc := range []struct {
		name    string
		fixture string
		argv    []string
		family  Family
	}{
		{"small git diff", "gitdiff_unstaged_change.txt", []string{"git", "diff"}, FamilyGitDiff},
		{"small git show", "gitshow_head.txt", []string{"git", "show"}, FamilyGitShow},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := string(readCorpusFixture(t, tc.fixture))
			fakeBin(t, "git", "cat <<'DS_EOF'\n"+raw+"DS_EOF\n")
			r, out, errOut := run(t, tc.argv, true)
			if r.ExitCode != 0 || r.Compacted || r.Family != tc.family || r.Passthrough != PassthroughSmallOutput {
				t.Fatalf("unexpected result: %+v (stderr %q)", r, errOut)
			}
			if out != raw {
				t.Fatalf("stdout = %q, want raw %q", out, raw)
			}
			if strings.Contains(out, markerPrefix) {
				t.Fatalf("unexpected marker in passthrough output: %q", out)
			}
		})
	}
}

func TestRun_GlabRedactsSecretsEvenWithTailNotesAndNeverStores(t *testing.T) {
	raw := string(readCorpusFixture(t, "glab_issue_view_header.txt"))
	fakeBin(t, "glab", "cat <<'DS_EOF'\n"+raw+"DS_EOF\n")

	var out, errb bytes.Buffer
	kept := false
	r := Run([]string{"glab", "issue", "view", "311"}, Options{
		Stdin:         strings.NewReader(""),
		Stdout:        &out,
		Stderr:        &errb,
		Compact:       true,
		DiscardStderr: true,
		Keep: func([]byte) (string, bool) {
			kept = true
			return "deadbeef0001", true
		},
	})
	if r.ExitCode != 0 || !r.Compacted || r.Family != FamilyGlab || kept || r.KeptID != "" {
		t.Fatalf("unexpected result: %+v kept=%v", r, kept)
	}
	got := out.String()
	if strings.Contains(got, "pr0xyS3cretValue") || strings.Contains(got, "example-deploy-secret") {
		t.Fatalf("glab secrets leaked in output: %q", got)
	}
	if !strings.Contains(got, "[redacted]") {
		t.Fatalf("missing [redacted] in output: %q", got)
	}
	if strings.Contains(got, "; original not kept]") || strings.Contains(got, "downshift retrieve") {
		t.Fatalf("unexpected compaction marker in glab output: %q", got)
	}
}
