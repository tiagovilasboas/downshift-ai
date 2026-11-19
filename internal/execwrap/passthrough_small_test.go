// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readCorpusFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "compressor", "testdata", "corpus", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// Small git diff and git show outputs whose summary + marker would not be
// shorter than raw must pass through byte-identical with no marker.
func TestCompactHeld_SmallGitDiffAndShowPassThroughWithoutMarker(t *testing.T) {
	cases := []struct {
		name    string
		fixture string
		argv    []string
		family  Family
	}{
		{"small git diff", "gitdiff_unstaged_change.txt", []string{"git", "diff"}, FamilyGitDiff},
		{"small git diff with danger word", "gitdiff_danger_word.txt", []string{"git", "diff"}, FamilyGitDiff},
		{"small git show", "gitshow_head.txt", []string{"git", "show"}, FamilyGitShow},
		{"small git show with danger word", "gitshow_danger_word.txt", []string{"git", "show"}, FamilyGitShow},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw := readCorpusFixture(t, tc.fixture)
			var keepCalls [][]byte
			for _, keep := range []func([]byte) (string, bool){nil, storeOK(&keepCalls)} {
				h := compactHeld(tc.argv, raw, tc.family, 0, false, keep, nil, false, balanceConfig{})
				if h.compacted {
					t.Fatalf("expected passthrough, got compacted=true out=%q", h.out)
				}
				if h.passthrough != PassthroughSmallOutput {
					t.Fatalf("passthrough = %q, want %q", h.passthrough, PassthroughSmallOutput)
				}
				if !bytes.Equal(h.out, raw) {
					t.Fatalf("output changed (%d -> %d bytes), want raw", len(raw), len(h.out))
				}
				if HasMarker(string(h.out)) || strings.Contains(string(h.out), markerPrefix) {
					t.Fatalf("passthrough output carries a marker: %q", h.out)
				}
			}
			if len(keepCalls) != 0 {
				t.Fatalf("expected raw not to be stored, got %d keep calls", len(keepCalls))
			}

			capRes := CompactCaptured(tc.argv, tc.family, raw, 0, nil, false, "")
			if capRes.Compacted || !bytes.Equal(capRes.Output, raw) || capRes.Passthrough != PassthroughSmallOutput {
				t.Fatalf("CompactCaptured: compacted=%v passthrough=%q len=%d/%d", capRes.Compacted, capRes.Passthrough, len(capRes.Output), len(raw))
			}
		})
	}
}

// glab is redaction-only: clean outputs pass through untouched without a
// marker, and outputs with secrets are always redacted without adding a marker
// (even when adding a marker would make them longer than raw) and never stored.
func TestCompactHeld_GlabRedactsWithoutMarkerAndKeepsPlainRaw(t *testing.T) {
	t.Run("plain glab passes through untouched", func(t *testing.T) {
		raw := readCorpusFixture(t, "glab_ci_list_plain.txt")
		h := compactHeld([]string{"glab", "ci", "list"}, raw, FamilyGlab, 0, false, nil, nil, false, balanceConfig{})
		if h.compacted || h.passthrough != PassthroughSmallOutput || !bytes.Equal(h.out, raw) {
			t.Fatalf("plain glab changed: compacted=%v passthrough=%q out=%q", h.compacted, h.passthrough, h.out)
		}
		if strings.Contains(string(h.out), markerPrefix) {
			t.Fatalf("unexpected marker: %q", h.out)
		}
	})

	for _, tc := range []struct {
		name    string
		fixture string
		secrets []string
	}{
		{
			name:    "mr view with tokens",
			fixture: "glab_mr_view_token.txt",
			secrets: []string{"example-deploy-secret", "example-api-secret", "eyJhbGciOiJIUzI1NiJ9"},
		},
		{
			name:    "issue view where marker would exceed raw",
			fixture: "glab_issue_view_header.txt",
			secrets: []string{"pr0xyS3cretValue", "example-deploy-secret"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := readCorpusFixture(t, tc.fixture)
			var keepCalls [][]byte
			h := compactHeld([]string{"glab", "issue", "view", "311"}, raw, FamilyGlab, 0, false, storeOK(&keepCalls), nil, false, balanceConfig{})
			if !h.compacted {
				t.Fatalf("expected redacted glab output to apply, passthrough=%q", h.passthrough)
			}
			if len(h.out) >= len(raw) {
				t.Fatalf("redacted glab grew: %d >= %d", len(h.out), len(raw))
			}
			if strings.Contains(string(h.out), markerPrefix) {
				t.Fatalf("glab output must not carry compaction marker: %q", h.out)
			}
			if !strings.Contains(string(h.out), "[redacted]") {
				t.Fatalf("expected [redacted] in output: %q", h.out)
			}
			for _, s := range tc.secrets {
				if strings.Contains(string(h.out), s) {
					t.Fatalf("secret %q leaked in output: %q", s, h.out)
				}
			}
			if len(keepCalls) != 0 {
				t.Fatalf("glab raw output must never be offered to keep: %d calls", len(keepCalls))
			}
		})
	}

	t.Run("short secret still redacts without marker", func(t *testing.T) {
		raw := []byte("title:\tfix\n--\nclone https://u:tok3n@gitlab.example.com/a.git\n")
		h := compactHeld([]string{"glab", "mr", "view", "1"}, raw, FamilyGlab, 0, false, nil, nil, false, balanceConfig{})
		if !h.compacted || strings.Contains(string(h.out), "tok3n") || !strings.Contains(string(h.out), "[redacted]") {
			t.Fatalf("short secret not redacted: compacted=%v out=%q", h.compacted, h.out)
		}
		if strings.Contains(string(h.out), markerPrefix) {
			t.Fatalf("short secret glab got marker: %q", h.out)
		}
	})
}
