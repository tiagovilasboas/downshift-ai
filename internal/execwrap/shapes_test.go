// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"fmt"
	"strings"
	"testing"
)

// pipedDiff is a diff with 40 context lines around one changed line that
// holds a danger word, long enough that the byte cut would otherwise run.
func pipedDiff() string {
	var b strings.Builder
	b.WriteString("diff --git a/x_test.go b/x_test.go\nindex 1a2b3c4..5d6e7f8 100644\n--- a/x_test.go\n+++ b/x_test.go\n@@ -1,90 +1,90 @@\n")
	for i := 0; i < 40; i++ {
		fmt.Fprintf(&b, " context line number %d of the file\n", i)
	}
	b.WriteString("-\tt.Log(\"ok\")\n+\tt.Errorf(\"step failed: %v\", err)\n")
	for i := 0; i < 40; i++ {
		fmt.Fprintf(&b, " trailing line number %d of the file\n", i)
	}
	return b.String()
}

func shapedRun(command, out string, balanced bool) Captured {
	keep := func([]byte) (string, bool) { return "abcdef012345", true }
	return CompactCapturedShaped(nil, FamilyNone, []byte(out), 0, keep, balanced, 1024, "", ShapesFor(command, []byte(out)))
}

func TestShapesKeepDangerChangedLine(t *testing.T) {
	in := pipedDiff()
	res := shapedRun("git diff | cat", in, true)
	if !res.Compacted || len(res.Output) >= len(in) {
		t.Fatalf("diff not compacted: %v %d >= %d", res.Compacted, len(res.Output), len(in))
	}
	for _, l := range []string{"+\tt.Errorf(\"step failed: %v\", err)", "-\tt.Log(\"ok\")"} {
		if !strings.Contains(string(res.Output), l) {
			t.Errorf("changed line lost: %q", l)
		}
	}
	if !strings.HasPrefix(string(res.Output), "diff --git") {
		t.Errorf("output does not open with the diff: %.60q", res.Output)
	}
}

func TestShapesRunInsideHeadLimit(t *testing.T) {
	in := pipedDiff()
	if s := ShapesFor("git diff | head -200", []byte(in)); !s.On || !s.Limited {
		t.Errorf("shapes %+v, want On and Limited under a head limit the output fits", s)
	}
	if s := ShapesFor("git diff | cat", []byte(in)); s.Limited {
		t.Error("Limited without a head or tail")
	}
	res := shapedRun("git diff | head -200", in, true)
	for _, l := range []string{"+\tt.Errorf(\"step failed: %v\", err)", "-\tt.Log(\"ok\")"} {
		if !strings.Contains(string(res.Output), l) {
			t.Errorf("changed line lost inside a head limit: %q", l)
		}
	}
	if res := shapedRun("git diff | cat", in, false); strings.Contains(string(res.Output), "@@ -1,90 +1,90 @@") == false {
		t.Error("balanced off must leave the diff as printed")
	}
}
