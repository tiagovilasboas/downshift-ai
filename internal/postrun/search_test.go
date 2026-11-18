// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package postrun

import (
	"fmt"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/core"
	"github.com/tiagovilasboas/downshift-ai/internal/execwrap"
	"github.com/tiagovilasboas/downshift-ai/internal/hookport"
)

// M2 regression: a 2,000-match Grep reaches the model under 16 KB, capped per
// file and in total, with counts of the remaining matches and raw text in CCR.
func TestCompact_SearchGrep2000MatchesUnder16KB(t *testing.T) {
	cases := []struct {
		name      string
		files     int
		perFile   int
		linePad   string
		wantCount string
	}{
		{
			name:      "single file 2000 matches",
			files:     1,
			perFile:   2000,
			linePad:   "match payload",
			wantCount: "1980 more matches in 1 files",
		},
		{
			name:      "100 files 20 matches each",
			files:     100,
			perFile:   20,
			linePad:   "match payload",
			wantCount: "1750 more matches in 88 files",
		},
		{
			name:      "100 files long lines",
			files:     100,
			perFile:   20,
			linePad:   strings.Repeat("x", 200),
			wantCount: "1750 more matches in 88 files",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var b strings.Builder
			for f := 0; f < tc.files; f++ {
				for m := 1; m <= tc.perFile; m++ {
					fmt.Fprintf(&b, "pkg/mod%03d/file.go:%d:%s\n", f, m, tc.linePad)
				}
			}
			raw := b.String()

			var stored []byte
			keep := func(data []byte) (string, bool) {
				stored = append([]byte(nil), data...)
				return "ccr2000match", true
			}

			in := hookport.ToolOutput{
				Category:   core.CategorySearch,
				Content:    raw,
				HasContent: true,
			}
			res := Compact(allCaps, in, Options{Keep: keep, Balanced: true})
			if !res.Considered || !res.Replaced || !res.Folded {
				t.Fatalf("expected replaced+folded result, got %+v", res)
			}
			if res.KeptID != "ccr2000match" || string(stored) != raw {
				t.Errorf("CCR storage mismatch: keptID=%q storedLen=%d rawLen=%d", res.KeptID, len(stored), len(raw))
			}
			if len(res.Output.Content) >= execwrap.BalanceThreshold {
				t.Errorf("emitted content %d bytes >= 16 KB (%d)", len(res.Output.Content), execwrap.BalanceThreshold)
			}
			if res.EmittedBytes >= int64(execwrap.BalanceThreshold) {
				t.Errorf("EmittedBytes %d >= 16 KB (%d)", res.EmittedBytes, execwrap.BalanceThreshold)
			}
			if !strings.Contains(res.Output.Content, tc.wantCount) {
				t.Errorf("missing remaining-match count %q in:\n%s", tc.wantCount, res.Output.Content)
			}
			if !strings.Contains(res.Output.Content, "downshift retrieve ccr2000match]") {
				t.Errorf("missing CCR retrieve marker in:\n%s", res.Output.Content)
			}
		})
	}
}

func TestCompact_SearchGlobLargePathListUnder16KB(t *testing.T) {
	paths := make([]string, 0, 1000)
	for i := 0; i < 1000; i++ {
		paths = append(paths, fmt.Sprintf("internal/pkg%03d/component/service_handler_%03d.go", i%50, i))
	}
	wantRaw := strings.Join(paths, "\n") + "\n"

	var stored []byte
	keep := func(data []byte) (string, bool) {
		stored = append([]byte(nil), data...)
		return "ccrglobpaths", true
	}

	in := hookport.ToolOutput{
		Category: core.CategorySearch,
		Paths:    paths,
		HasPaths: true,
	}
	res := Compact(allCaps, in, Options{Keep: keep, Balanced: true})
	if !res.Considered || !res.Replaced || !res.Folded {
		t.Fatalf("expected replaced+folded result, got %+v", res)
	}
	if res.KeptID != "ccrglobpaths" || string(stored) != wantRaw {
		t.Errorf("CCR storage mismatch: keptID=%q storedLen=%d rawLen=%d", res.KeptID, len(stored), len(wantRaw))
	}
	joined := strings.Join(res.Output.Paths, "\n")
	if len(joined) >= execwrap.BalanceThreshold {
		t.Errorf("emitted paths %d bytes >= 16 KB (%d)", len(joined), execwrap.BalanceThreshold)
	}
	if !strings.Contains(joined, "more files (downshift retrieve)") {
		t.Errorf("missing remaining-file count in:\n%s", joined)
	}
	if !strings.Contains(joined, "downshift retrieve ccrglobpaths]") {
		t.Errorf("missing CCR retrieve marker in:\n%s", joined)
	}
}

func TestCompact_SearchDualKeptIDs(t *testing.T) {
	var content strings.Builder
	for i := 1; i <= 500; i++ {
		fmt.Fprintf(&content, "pkg/mod%02d/file.go:%d:match payload\n", i%10, i)
	}
	paths := make([]string, 0, 500)
	for i := 0; i < 500; i++ {
		paths = append(paths, fmt.Sprintf("internal/pkg%02d/component/service_handler_%03d.go", i%20, i))
	}
	in := hookport.ToolOutput{
		Category:   core.CategorySearch,
		Content:    content.String(),
		Paths:      paths,
		HasContent: true,
		HasPaths:   true,
	}

	seq := 0
	ids := []string{"aaaaaaaaaaaa", "bbbbbbbbbbbb"}
	res := Compact(allCaps, in, Options{
		Keep: func([]byte) (string, bool) {
			id := ids[seq]
			seq++
			return id, true
		},
		Balanced: true,
	})
	if !res.Replaced || res.KeptID != "aaaaaaaaaaaa" || len(res.ExtraKeptIDs) != 1 || res.ExtraKeptIDs[0] != "bbbbbbbbbbbb" {
		t.Fatalf("dual search kept IDs: kept=%q extra=%v replaced=%v", res.KeptID, res.ExtraKeptIDs, res.Replaced)
	}

	// Duplicate IDs across content and paths are not repeated in ExtraKeptIDs.
	dup := Compact(allCaps, in, Options{Keep: keepOK, Balanced: true})
	if dup.KeptID != "0123456789ab" || len(dup.ExtraKeptIDs) != 0 {
		t.Fatalf("duplicate search kept IDs: kept=%q extra=%v", dup.KeptID, dup.ExtraKeptIDs)
	}
}
