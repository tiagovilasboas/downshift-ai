// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/execwrap"
)

// listingFixtures are long listings built the way a real repository's are: a
// few hundred files under shared directories, and a commit log. They are
// generated, so the baseline needs no large fixture file.
func listingFixtures() map[string]struct {
	argv   []string
	family execwrap.Family
	data   []byte
} {
	var files, ls, log strings.Builder
	for i := 0; i < 1500; i++ {
		fmt.Fprintf(&files, "internal/domain%02d/feature%02d/handler_%04d.go\n", i%23, i%7, i)
	}
	for i := 0; i < 1200; i++ {
		fmt.Fprintf(&ls, "module_%04d_%x.go\n", i, i*2654435761)
	}
	for i := 0; i < 700; i++ {
		fmt.Fprintf(&log, "%07x feat(area%d): change number %d touches module %x\n", i*7919+4096, i%9, i, i*40503)
	}
	type fx = struct {
		argv   []string
		family execwrap.Family
		data   []byte
	}
	return map[string]fx{
		"git_ls_files": {[]string{"git", "ls-files"}, execwrap.FamilyGitLsFiles, []byte(files.String())},
		"ls_1":         {[]string{"ls", "-1"}, execwrap.FamilyLs, []byte(ls.String())},
		"git_log":      {[]string{"git", "log", "--oneline"}, execwrap.FamilyGitLog, []byte(log.String())},
	}
}

// TestListingBaseline reports the bytes the agent reads for a long listing
// (family compressor, then the balanced cut with CCR storing) and bounds the
// ratio: a worse result must not pass.
func TestListingBaseline(t *testing.T) {
	keep := func([]byte) (string, bool) { return "baseline0001", true }
	for name, f := range listingFixtures() {
		t.Run(name, func(t *testing.T) {
			res := execwrap.CompactCapturedWithThreshold(f.argv, f.family, f.data, 0, keep, true, 0, "")
			ratio := float64(len(res.Output)) / float64(len(f.data))
			t.Logf("%-14s raw=%6d out=%6d ratio=%.3f", name, len(f.data), len(res.Output), ratio)
			if !strings.Contains(string(res.Output), "downshift retrieve") {
				t.Fatalf("no retrieve marker: the cut lines are not recoverable")
			}
			if ratio > 0.15 {
				t.Errorf("ratio %.3f over the 0.15 bound", ratio)
			}
		})
	}
}
