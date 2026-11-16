// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/compressor"
)

func TestCompressExitExpected_MatchesCompressExit(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		mode     compressor.Mode
		exitCode int
		expected []compressor.Format
	}{
		{
			name:     "go_test_pass",
			input:    "ok  github.com/foo/bar/pkg1  0.1s\nok  github.com/foo/bar/pkg2  0.2s\nok  github.com/foo/bar/pkg3  0.3s\n",
			mode:     compressor.ModeSafe,
			exitCode: 0,
			expected: []compressor.Format{compressor.FormatGoTest},
		},
		{
			name:     "go_test_json_expected_jest_safety",
			input:    "{\"Time\":\"2026-01-01T00:00:00Z\",\"Action\":\"run\",\"Package\":\"pkg\",\"Test\":\"TestA\"}\n{\"Time\":\"2026-01-01T00:00:01Z\",\"Action\":\"output\",\"Package\":\"pkg\",\"Test\":\"TestA\",\"Output\":\"Test Suites: 1 passed\\n\"}\n{\"Time\":\"2026-01-01T00:00:01Z\",\"Action\":\"pass\",\"Package\":\"pkg\",\"Test\":\"TestA\",\"Elapsed\":0.1}\n{\"Time\":\"2026-01-01T00:00:01Z\",\"Action\":\"pass\",\"Package\":\"pkg\",\"Elapsed\":0.1}\n",
			mode:     compressor.ModeSafe,
			exitCode: 0,
			expected: []compressor.Format{compressor.FormatJest},
		},
		{
			name:     "git_status",
			input:    "On branch main\nYour branch is up to date with 'origin/main'.\n\nChanges not staged for commit:\n  (use \"git add <meta>...\" to update what will be committed)\n\n\tmodified:   a.go\n\tmodified:   b.go\n\tmodified:   c.go\n",
			mode:     compressor.ModeSafe,
			exitCode: 0,
			expected: []compressor.Format{compressor.FormatGitStatus},
		},
		{
			name:     "git_log",
			input:    "commit 1234567890abcdef1234567890abcdef12345678\nAuthor: Tiago Vilas Boas <t@example.com>\nDate:   Mon Jan 1 00:00:00 2026 +0000\n\n    first commit\n\ncommit abcdef1234567890abcdef1234567890abcdef12\nAuthor: Tiago Vilas Boas <t@example.com>\nDate:   Mon Jan 1 00:00:00 2026 +0000\n\n    second commit\n",
			mode:     compressor.ModeSafe,
			exitCode: 0,
			expected: []compressor.Format{compressor.FormatGitLog},
		},
		{
			name:     "git_diff",
			input:    "diff --git a/a.go b/a.go\nindex 1111111..2222222 100644\n--- a/a.go\n+++ b/a.go\n@@ -1,10 +1,10 @@\n line1\n line2\n line3\n line4\n-old\n+new\n line6\n line7\n line8\n line9\n line10\n",
			mode:     compressor.ModeSafe,
			exitCode: 0,
			expected: []compressor.Format{compressor.FormatGitDiff, compressor.FormatLockfileDiff},
		},
		{
			name:     "lockfile_diff_with_git_diff_first",
			input:    "diff --git a/package-lock.json b/package-lock.json\nindex 1111111..2222222 100644\n--- a/package-lock.json\n+++ b/package-lock.json\n@@ -1,6 +1,6 @@\n   \"packages\": {\n     \"node_modules/lodash\": {\n-      \"version\": \"4.17.20\"\n+      \"version\": \"4.17.21\"\n     }\n   }\n",
			mode:     compressor.ModeSafe,
			exitCode: 0,
			expected: []compressor.Format{compressor.FormatGitDiff, compressor.FormatLockfileDiff},
		},
		{
			name:     "git_show",
			input:    "commit 1234567890abcdef1234567890abcdef12345678\nAuthor: Tiago Vilas Boas <t@example.com>\nDate:   Mon Jan 1 00:00:00 2026 +0000\n\n    change\n\ndiff --git a/a.go b/a.go\nindex 1111111..2222222 100644\n--- a/a.go\n+++ b/a.go\n@@ -1,7 +1,7 @@\n line1\n line2\n line3\n-old\n+new\n line5\n line6\n line7\n",
			mode:     compressor.ModeSafe,
			exitCode: 0,
			expected: []compressor.Format{compressor.FormatGitShow},
		},
		{
			name:     "jest_pass_with_vitest_first",
			input:    "PASS src/a.test.ts\nPASS src/b.test.ts\n\nTest Suites: 2 passed, 2 total\nTests:       4 passed, 4 total\nSnapshots:   0 total\nTime:        1.234 s\n",
			mode:     compressor.ModeSafe,
			exitCode: 0,
			expected: []compressor.Format{compressor.FormatVitest, compressor.FormatJest},
		},
		{
			name:     "jest_pass_expected_go_test_fallback",
			input:    "PASS src/a.test.ts\nPASS src/b.test.ts\n\nTest Suites: 2 passed, 2 total\nTests:       4 passed, 4 total\nSnapshots:   0 total\nTime:        1.234 s\n",
			mode:     compressor.ModeSafe,
			exitCode: 0,
			expected: []compressor.Format{compressor.FormatGoTest},
		},
		{
			name:     "vitest_pass",
			input:    " ✓ src/a.test.ts (2 tests) 10ms\n ✓ src/b.test.ts (3 tests) 12ms\n\n Test Files  2 passed (2)\n      Tests  5 passed (5)\n   Duration  250ms\n",
			mode:     compressor.ModeSafe,
			exitCode: 0,
			expected: []compressor.Format{compressor.FormatVitest, compressor.FormatJest},
		},
		{
			name:     "pytest_pass",
			input:    "============================= test session starts ==============================\ncollected 4 items\n\ntests/test_a.py ....                                                     [100%]\n\n============================== 4 passed in 0.12s ===============================\n",
			mode:     compressor.ModeSafe,
			exitCode: 0,
			expected: []compressor.Format{compressor.FormatPytest},
		},
		{
			name:     "tsc_pass",
			input:    "Found 0 errors.\n",
			mode:     compressor.ModeSafe,
			exitCode: 0,
			expected: []compressor.Format{compressor.FormatTSC},
		},
		{
			name:     "docker_ps",
			input:    "CONTAINER ID   IMAGE     COMMAND   CREATED   STATUS    PORTS     NAMES\nabc123456789   nginx     \"nginx\"   2h ago    Up 2h     80/tcp    web\n",
			mode:     compressor.ModeSafe,
			exitCode: 0,
			expected: []compressor.Format{compressor.FormatDockerPs},
		},
		{
			name:     "kubectl_pods",
			input:    "NAME                     READY   STATUS    RESTARTS   AGE\nweb-7d8f9b6c4-abcde      1/1     Running   0          2d\n",
			mode:     compressor.ModeSafe,
			exitCode: 0,
			expected: []compressor.Format{compressor.FormatKubectlPods},
		},
		{
			name:     "npm_install",
			input:    "added 42 packages, and audited 100 packages in 3s\n\n10 packages are looking for funding\n  run `npm fund` for details\n\nfound 0 vulnerabilities\n",
			mode:     compressor.ModeSafe,
			exitCode: 0,
			expected: []compressor.Format{compressor.FormatProgress, compressor.FormatNpmAudit, compressor.FormatNpmInstall},
		},
		{
			name:     "generic_repeated_lines",
			input:    strings.Repeat("checking item ok\n", 10),
			mode:     compressor.ModeSafe,
			exitCode: 0,
			expected: []compressor.Format{compressor.FormatGeneric},
		},
		{
			name:     "nonzero_exit_preserved",
			input:    "ok  pkg1 0.1s\n",
			mode:     compressor.ModeSafe,
			exitCode: 1,
			expected: []compressor.Format{compressor.FormatGoTest},
		},
		{
			name:     "observe_mode",
			input:    "ok  github.com/foo/bar/pkg1  0.1s\nok  github.com/foo/bar/pkg2  0.2s\nok  github.com/foo/bar/pkg3  0.3s\n",
			mode:     compressor.ModeObserve,
			exitCode: 0,
			expected: []compressor.Format{compressor.FormatGoTest},
		},
		{
			name:     "empty_expected_falls_back",
			input:    "ok  github.com/foo/bar/pkg1  0.1s\nok  github.com/foo/bar/pkg2  0.2s\nok  github.com/foo/bar/pkg3  0.3s\n",
			mode:     compressor.ModeSafe,
			exitCode: 0,
			expected: nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := []byte(tc.input)
			want := compressor.CompressExit(in, tc.mode, tc.exitCode)
			got := compressor.CompressExitExpected(in, tc.mode, tc.exitCode, tc.expected...)
			if got.Format != want.Format {
				t.Errorf("Format: got %q, want %q", got.Format, want.Format)
			}
			if got.Applied != want.Applied {
				t.Errorf("Applied: got %v, want %v (got reason=%q, want reason=%q)", got.Applied, want.Applied, got.Reason, want.Reason)
			}
			if got.Reason != want.Reason {
				t.Errorf("Reason: got %q, want %q", got.Reason, want.Reason)
			}
			if got.OriginalBytes != want.OriginalBytes || got.ReducedBytes != want.ReducedBytes {
				t.Errorf("Bytes: got (%d->%d), want (%d->%d)", got.OriginalBytes, got.ReducedBytes, want.OriginalBytes, want.ReducedBytes)
			}
			if !bytes.Equal(got.Output, want.Output) {
				t.Errorf("Output mismatch:\ngot:  %q\nwant: %q", got.Output, want.Output)
			}
		})
	}
}
