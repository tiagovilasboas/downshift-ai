// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"fmt"
	"strings"
	"testing"
)

// distinctPaths is a long list of distinct file names (no shared prefix to
// factor, no repeat to fold), so only the balanced cut can shorten it.
func distinctPaths(n int) []byte {
	var b strings.Builder
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "internal/feature%03d/sub%02d/handler_%05d_%x.go\n", i%97, i%13, i, i*2654435761)
	}
	return []byte(b.String())
}

func commitLog(n int) []byte {
	var b strings.Builder
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "%07x feat(area%d): change number %d touches module %x\n", i*7919+4096, i%9, i, i*40503)
	}
	return []byte(b.String())
}

func cutListing(argv []string, family Family, data []byte, exit int) (Captured, string) {
	var stored []byte
	keep := func(raw []byte) (string, bool) { stored = raw; return "0123456789ab", true }
	res := CompactCaptured(argv, family, data, exit, keep, true, "")
	return res, string(stored)
}

func TestBalancedListingsCutWithRetrieveMarker(t *testing.T) {
	cases := []struct {
		name   string
		argv   []string
		family Family
		data   []byte
	}{
		{"ls", []string{"ls", "-1"}, FamilyLs, distinctPaths(900)},
		{"ls-files", []string{"git", "ls-files"}, FamilyGitLsFiles, distinctPaths(900)},
		{"diff-name-only", []string{"git", "diff", "--name-only"}, FamilyGitDiff, distinctPaths(900)},
		{"log-oneline", []string{"git", "log", "--oneline"}, FamilyGitLog, commitLog(700)},
		{"branch", []string{"git", "branch", "-a"}, FamilyGitBranch, distinctPaths(900)},
		{"tag", []string{"git", "tag"}, FamilyGitTag, distinctPaths(900)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			res, stored := cutListing(c.argv, c.family, c.data, 0)
			out := string(res.Output)
			if !res.Compacted || res.KeptID != "0123456789ab" || stored != string(c.data) {
				t.Fatalf("not cut or raw not kept: compacted=%v id=%q kept=%v", res.Compacted, res.KeptID, stored == string(c.data))
			}
			if !strings.Contains(out, "downshift retrieve") || len(res.Output)*4 > len(c.data)*3 {
				t.Errorf("no retrieve marker or gain under 25%%: %d of %d bytes", len(res.Output), len(c.data))
			}
			t.Logf("%s: %d -> %d bytes (%.1f%%)", c.name, len(c.data), len(res.Output), 100*float64(len(res.Output))/float64(len(c.data)))
		})
	}
}

func TestBalancedListingKeepsDangerLines(t *testing.T) {
	data := distinctPaths(900)
	lines := strings.Split(string(data), "\n")
	lines[450] = "fatal: unable to read tree 0123abcd"
	data = []byte(strings.Join(lines, "\n"))
	res, _ := cutListing([]string{"git", "ls-files"}, FamilyGitLsFiles, data, 0)
	if !strings.Contains(string(res.Output), "fatal: unable to read tree") {
		t.Error("a failure line in the middle was dropped")
	}
}

func TestBalancedListingsLeftAlone(t *testing.T) {
	long := distinctPaths(900)
	t.Run("short listing", func(t *testing.T) {
		short := distinctPaths(40)
		res, _ := cutListing([]string{"ls", "-1"}, FamilyLs, short, 0)
		if strings.Contains(string(res.Output), "downshift retrieve") {
			t.Errorf("a short listing was cut: %d of %d bytes", len(res.Output), len(short))
		}
	})
	t.Run("exit code not zero", func(t *testing.T) {
		res, _ := cutListing([]string{"git", "ls-files"}, FamilyGitLsFiles, long, 1)
		if strings.Contains(string(res.Output), "lines cut") {
			t.Error("a failed listing took the byte cut")
		}
	})
	t.Run("within the limit the agent chose", func(t *testing.T) {
		data := commitLog(300)
		res, _ := cutListing([]string{"git", "log", "--oneline", "-n", "300"}, FamilyGitLog, data, 0)
		if strings.Contains(string(res.Output), "lines cut") {
			t.Error("the lines the agent asked for were cut")
		}
	})
}

func TestArgvLineLimit(t *testing.T) {
	for argv, want := range map[string]int{"git log -n 50": 50, "git log -n50": 50, "git log -20": 20, "git log --max-count=7": 7, "git log --oneline -n 3": 3} {
		if n, ok := argvLineLimit(strings.Fields(argv)); !ok || n != want {
			t.Errorf("%q: got %d %v, want %d", argv, n, ok, want)
		}
	}
	for _, argv := range []string{"git log --oneline", "git log -n x", "git ls-files"} {
		if _, ok := argvLineLimit(strings.Fields(argv)); ok {
			t.Errorf("%q: unexpected limit", argv)
		}
	}
}
