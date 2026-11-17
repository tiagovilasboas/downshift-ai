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

func makeLockfile(lines int) string {
	var b strings.Builder
	for i := 1; i <= lines; i++ {
		fmt.Fprintf(&b, "    \"pkg-%04d\": { \"version\": \"1.0.%d\", \"integrity\": \"sha512-abcdef%04d\" },\n", i, i%10, i)
	}
	return b.String()
}

func TestCompactRead_LockfileCutAndEditVerbatim(t *testing.T) {
	raw := makeLockfile(1500) // ~125 KB > 64 KiB
	if len(raw) <= ReadThreshold {
		t.Fatalf("fixture too small: %d", len(raw))
	}
	var stored []byte
	keep := func(b []byte) (string, bool) {
		stored = append([]byte(nil), b...)
		return "ccrread12345", true
	}
	in := hookport.ToolOutput{Category: core.CategoryRead, Content: raw, HasContent: true}
	sess := strings.Repeat("ab", 32)
	res := Compact(allCaps, in, Options{Keep: keep, Balanced: true, Session: sess})

	if !res.Considered || !res.Replaced || !res.Compacted || !res.Folded {
		t.Fatalf("unexpected result flags: %+v", res)
	}
	if res.Family != FamilyReadTool || res.KeptID != "ccrread12345" {
		t.Fatalf("family=%q kept=%q", res.Family, res.KeptID)
	}
	if string(stored) != raw {
		t.Errorf("CCR stored bytes differ from raw (%d vs %d)", len(stored), len(raw))
	}
	if !execwrap.HasMarker(res.Output.Content) {
		t.Errorf("missing trailing execwrap marker in:\n%s", res.Output.Content)
	}
	for _, want := range []string{
		"lines ",
		"omitted (",
		"Read(offset=",
		"downshift retrieve --session=abababababab ccrread12345",
	} {
		if !strings.Contains(res.Output.Content, want) {
			t.Errorf("missing %q in cut output", want)
		}
	}

	// Verify kept head and tail lines are byte-for-byte identical for Edit.
	rawLines := strings.SplitAfter(raw, "\n")
	if rawLines[len(rawLines)-1] == "" {
		rawLines = rawLines[:len(rawLines)-1]
	}
	headSample := strings.Join(rawLines[:20], "")
	tailSample := strings.Join(rawLines[len(rawLines)-20:], "")
	if !strings.HasPrefix(res.Output.Content, headSample) {
		t.Errorf("head lines were modified; Edit would fail to match")
	}
	if !strings.Contains(res.Output.Content, tailSample) {
		t.Errorf("tail lines were modified; Edit would fail to match")
	}
}

func TestCompactRead_UnderOrEqualThresholdUntouched(t *testing.T) {
	exact := strings.Repeat("a", ReadThreshold-1) + "\n"
	if len(exact) != ReadThreshold {
		t.Fatalf("len=%d, want %d", len(exact), ReadThreshold)
	}
	called := false
	keep := func([]byte) (string, bool) {
		called = true
		return "shouldnotrun", true
	}
	in := hookport.ToolOutput{Category: core.CategoryRead, Content: exact, HasContent: true}
	res := Compact(allCaps, in, Options{Keep: keep, Balanced: true})
	if called {
		t.Error("CCR keep called for Read <= 64 KiB")
	}
	if !res.Considered || res.Replaced || res.Output.Content != exact {
		t.Errorf("Read <= 64 KiB was altered: %+v", res)
	}
}

func TestCompactRead_DisabledOrCCRFailLeavesRaw(t *testing.T) {
	raw := makeLockfile(1500)
	in := hookport.ToolOutput{Category: core.CategoryRead, Content: raw, HasContent: true}

	if res := Compact(allCaps, in, Options{Keep: keepOK, Balanced: false}); res.Replaced || res.Output.Content != raw {
		t.Error("cut with Balanced=false")
	}
	if res := Compact(allCaps, in, Options{Keep: nil, Balanced: true}); res.Replaced || res.Output.Content != raw {
		t.Error("cut with Keep=nil")
	}
	failKeep := func([]byte) (string, bool) { return "", false }
	if res := Compact(allCaps, in, Options{Keep: failKeep, Balanced: true}); res.Replaced || res.Output.Content != raw {
		t.Error("cut when CCR keep failed")
	}
}

func TestCompactRead_SingleLineOverThresholdUntouched(t *testing.T) {
	raw := strings.Repeat("x", ReadThreshold+1024)
	called := false
	keep := func([]byte) (string, bool) {
		called = true
		return "id", true
	}
	in := hookport.ToolOutput{Category: core.CategoryRead, Content: raw, HasContent: true}
	res := Compact(allCaps, in, Options{Keep: keep, Balanced: true})
	if called || res.Replaced || res.Output.Content != raw {
		t.Errorf("single-line Read should stay untouched: called=%v replaced=%v", called, res.Replaced)
	}
}
