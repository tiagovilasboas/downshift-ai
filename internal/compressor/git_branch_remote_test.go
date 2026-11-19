// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor_test

import (
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/compressor"
)

func TestCompressGitBranch(t *testing.T) {
	cases := []struct {
		name        string
		input       string
		exitCode    int
		wantApplied bool
		wantOut     string
	}{
		{
			name: "verbose padding and remotes factoring",
			input: "  feat/a                    1a2b3c4 [origin/feat/a] fix:   keep double space in subject\n" +
				"+ feat/wt                   2b3c4d5 (/tmp/wt) wip\n" +
				"* main                      3c4d5e6 [origin/main] init\n" +
				"  remotes/origin/HEAD       -> origin/main\n" +
				"  remotes/origin/chore/rel  4d5e6f7 release\n" +
				"  remotes/origin/main       3c4d5e6 init\n",
			wantApplied: true,
			wantOut: "feat/a 1a2b3c4 [origin/feat/a] fix:   keep double space in subject\n" +
				"+ feat/wt 2b3c4d5 (/tmp/wt) wip\n" +
				"* main 3c4d5e6 [origin/main] init\n" +
				"remotes/origin/:\n" +
				" HEAD -> origin/main\n" +
				" chore/rel 4d5e6f7 release\n" +
				" main 3c4d5e6 init\n",
		},
		{
			name: "detached head with verbose padding",
			input: "* (HEAD detached at 9cde7b5)   9cde7b51 head msg\n" +
				"  main                         9cde7b51 main msg\n",
			wantApplied: true,
			wantOut: "* (HEAD detached at 9cde7b5) 9cde7b51 head msg\n" +
				"main 9cde7b51 main msg\n",
		},
		{
			name:        "single current branch has no gain",
			input:       "* main\n",
			wantApplied: false,
		},
		{
			name:        "show-current plain line stays raw",
			input:       "main\n",
			wantApplied: false,
		},
		{
			name:        "stray error line stays raw",
			input:       "* main\nfatal: bad ref\n",
			wantApplied: false,
		},
		{
			name:        "non-zero exit stays raw",
			input:       "  feat/a\n* main\n",
			exitCode:    1,
			wantApplied: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := compressor.CompressCommandOutput([]byte(tc.input), compressor.FormatGitBranch, tc.exitCode)
			if res.Applied != tc.wantApplied {
				t.Fatalf("applied=%v, want %v (reason=%q out=%q)", res.Applied, tc.wantApplied, res.Reason, res.Output)
			}
			if tc.wantApplied && string(res.Output) != tc.wantOut {
				t.Fatalf("got:\n%s\nwant:\n%s", res.Output, tc.wantOut)
			}
			if !tc.wantApplied && string(res.Output) != tc.input {
				t.Fatalf("raw case mutated input: %q", res.Output)
			}
		})
	}
}

func TestCompressGitRemoteList(t *testing.T) {
	cases := []struct {
		name        string
		input       string
		exitCode    int
		wantApplied bool
		wantOut     string
	}{
		{
			name: "matching fetch and push folded",
			input: "origin\tgit@github.com:org/repo.git (fetch)\n" +
				"origin\tgit@github.com:org/repo.git (push)\n" +
				"upstream\thttps://github.com/up/repo.git (fetch)\n" +
				"upstream\thttps://github.com/up/repo.git (push)\n",
			wantApplied: true,
			wantOut: "origin\tgit@github.com:org/repo.git (fetch, push)\n" +
				"upstream\thttps://github.com/up/repo.git (fetch, push)\n",
		},
		{
			name: "different fetch and push urls preserved",
			input: "origin\thttps://github.com/org/repo.git (fetch)\n" +
				"origin\tgit@github.com:org/repo.git (push)\n",
			wantApplied: false,
		},
		{
			name:        "plain git remote names without -v stay raw",
			input:       "origin\nupstream\n",
			wantApplied: false,
		},
		{
			name: "non-zero exit stays raw",
			input: "origin\tgit@github.com:org/repo.git (fetch)\n" +
				"origin\tgit@github.com:org/repo.git (push)\n",
			exitCode:    128,
			wantApplied: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := compressor.CompressCommandOutput([]byte(tc.input), compressor.FormatGitRemoteList, tc.exitCode)
			if res.Applied != tc.wantApplied {
				t.Fatalf("applied=%v, want %v (reason=%q out=%q)", res.Applied, tc.wantApplied, res.Reason, res.Output)
			}
			if tc.wantApplied && string(res.Output) != tc.wantOut {
				t.Fatalf("got:\n%s\nwant:\n%s", res.Output, tc.wantOut)
			}
			if !tc.wantApplied && string(res.Output) != tc.input {
				t.Fatalf("raw case mutated input: %q", res.Output)
			}
		})
	}
}
