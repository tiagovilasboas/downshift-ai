// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/paths"
)

// noisyGoTest is verbose go test output no compressor recognizes whose lines
// nearly all carry a keep word, so the line cut saves nothing.
func noisyGoTest(n int) []byte {
	var b bytes.Buffer
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "    retry_test.go:%d: warning: retrying request %d after a transient delay\n", i, i)
	}
	return b.Bytes()
}

// failingMake is a make build of many compile lines with a few errors.
func failingMake(units int) []byte {
	var b bytes.Buffer
	for i := 0; i < units; i++ {
		fmt.Fprintf(&b, "gcc -O2 -c src/file%d.c -o obj/file%d.o -Wall -Wextra\n", i, i)
		if i%100 == 50 {
			fmt.Fprintf(&b, "src/file%d.c:12:5: error: unknown type name foo%d\n", i, i)
		}
	}
	b.WriteString("make: *** [Makefile:10: all] Error 2\n")
	return b.Bytes()
}

func fallbackKeep() (func([]byte) (string, bool), *[]byte) {
	var stored []byte
	return func(r []byte) (string, bool) { stored = r; return "0123456789ab", true }, &stored
}

func TestFallbackCutUnrecognizedCleanRun(t *testing.T) {
	raw := noisyGoTest(500)
	keep, stored := fallbackKeep()
	r := CompactCaptured([]string{"go", "test", "-v", "./..."}, FamilyGoTest, raw, 0, keep, true, "")
	if !r.Compacted || r.Detail != DetailFallbackCut || r.Passthrough != "" {
		t.Fatalf("compacted=%v detail=%q pass=%q", r.Compacted, r.Detail, r.Passthrough)
	}
	if !bytes.Equal(*stored, raw) || len(r.Output)*2 > len(raw) {
		t.Errorf("stored original=%v, %d of %d bytes", bytes.Equal(*stored, raw), len(r.Output), len(raw))
	}
	if !strings.Contains(string(r.Output), "downshift retrieve 0123456789ab]") {
		t.Error("no retrieve marker")
	}
}

func TestFallbackCutFailedRunKeepsErrorsAndTail(t *testing.T) {
	raw := failingMake(400)
	keep, stored := fallbackKeep()
	r := CompactCapturedFailed([]string{"make"}, FamilyMake, raw, 2, keep, 0, "", Shapes{})
	if !r.Compacted || r.Detail != DetailFallbackCut {
		t.Fatalf("compacted=%v detail=%q pass=%q", r.Compacted, r.Detail, r.Passthrough)
	}
	out := string(r.Output)
	for _, want := range []string{"error: unknown type name foo50\n", "foo150\n", "foo250\n", "foo350\n", "make: *** [Makefile:10: all] Error 2\n", "file399.c"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q", want)
		}
	}
	if len(out) >= len(raw)*3/4 || !bytes.Equal(*stored, raw) {
		t.Errorf("%d of %d bytes, stored=%v", len(out), len(raw), bytes.Equal(*stored, raw))
	}
}

func TestFallbackCutUntouched(t *testing.T) {
	big, small := noisyGoTest(500), noisyGoTest(5)
	danger := append([]byte("diff --git a/x b/x\n@@ -1 +1 @@\n"), bytes.Repeat([]byte("+\tif err != nil { panic(\"error handling the thing\") }\n"), 400)...)
	cases := []struct {
		name   string
		family Family
		argv   []string
		data   []byte
		exit   int
		keep   bool
	}{
		{"small output", FamilyGoTest, []string{"go", "test"}, small, 0, true},
		{"no store", FamilyGoTest, []string{"go", "test", "-v"}, big, 0, false},
		{"failed, no store", FamilyMake, []string{"make"}, failingMake(400), 2, false},
		{"file read", FamilyFileRead, []string{"cat", "x"}, big, 0, true},
		{"curl never stored", FamilyCurl, []string{"curl", "x"}, big, 0, true},
	}
	for _, c := range cases {
		keep, _ := fallbackKeep()
		if !c.keep {
			keep = nil
		}
		r := CompactCapturedFailed(c.argv, c.family, c.data, c.exit, keep, 0, "", Shapes{})
		if r.Detail == DetailFallbackCut || !bytes.Equal(r.Output, c.data) && !r.Compacted {
			t.Errorf("%s: cut or altered: detail=%q", c.name, r.Detail)
		}
	}
	// A git diff holding a danger word is read verbatim: the fallback leaves it.
	keep, _ := fallbackKeep()
	if _, ok := fallbackCut(keep, FamilyGitDiff, []string{"git", "diff"}, 0, danger, DetailDangerWord, balanceConfig{}); ok {
		t.Error("a danger-word diff was cut")
	}
	if _, ok := fallbackCut(keep, FamilyGitDiff, []string{"git", "diff"}, 0, danger, DetailUnknownShape, balanceConfig{}); !ok {
		t.Error("a diff with another detail should take the cut")
	}
}

func TestFallbackCutKeepsExitCodeOnWrapper(t *testing.T) {
	raw := failingMake(400)
	keep, _ := fallbackKeep()
	h := compactHeld([]string{"make"}, raw, FamilyMake, 2, false, keep, nil, true, balanceConfig{fallbackOnly: true})
	if !h.compacted || h.detail != DetailFallbackCut || h.passthrough != "" {
		t.Fatalf("held: %+v", h.detail)
	}
}

func TestCompactCapturedFailed_FamilyNoneFallback(t *testing.T) {
	bigFail := failingMake(400)
	smallFail := []byte("=== RUN   TestCase0\n--- FAIL: TestCase0 (0.00s)\nFAIL\n")
	sess := strings.Repeat("cd", 32)

	cases := []struct {
		name      string
		argv      []string
		data      []byte
		exit      int
		keep      bool
		antiLoop  bool
		threshold int
		wantCut   bool
		wantDet   string
	}{
		{"big failure with CCR", []string{"sh", "-c", "make"}, bigFail, 2, true, false, 0, true, DetailFallbackCut},
		{"small failure stays raw", []string{"cd repo && go test ./..."}, smallFail, 1, true, false, 0, false, DetailNoCompressor},
		{"big failure without CCR stays raw", []string{"sh", "-c", "make"}, bigFail, 2, false, false, 0, false, DetailNoCompressor},
		{"big failure with active anti-loop bypasses", []string{"sh", "-c", "make"}, bigFail, 2, true, true, 0, false, DetailAntiLoopBypass},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(paths.EnvStateDir, t.TempDir())
			usedSess := ""
			if tc.antiLoop {
				usedSess = sess
				RecordAntiLoop(sess, 2, "deadbeef0001")
				RecordAntiLoop(sess, 2, "deadbeef0001")
			}
			var stored []byte
			var keep func([]byte) (string, bool)
			if tc.keep {
				keep = func(b []byte) (string, bool) {
					stored = append([]byte(nil), b...)
					return "0123456789ab", true
				}
			}
			r := CompactCapturedFailed(tc.argv, FamilyNone, tc.data, tc.exit, keep, tc.threshold, usedSess, Shapes{})
			if r.Compacted != tc.wantCut {
				t.Fatalf("Compacted = %v, want %v (detail=%q passthrough=%q)", r.Compacted, tc.wantCut, r.Detail, r.Passthrough)
			}
			if r.Detail != tc.wantDet {
				t.Errorf("Detail = %q, want %q", r.Detail, tc.wantDet)
			}
			if tc.wantCut {
				if r.KeptID != "0123456789ab" || !bytes.Equal(stored, tc.data) {
					t.Errorf("CCR mismatch: KeptID=%q stored=%v", r.KeptID, bytes.Equal(stored, tc.data))
				}
				out := string(r.Output)
				if !strings.Contains(out, "downshift retrieve 0123456789ab]") {
					t.Errorf("missing retrieve marker in:\n%s", out)
				}
				if !strings.Contains(out, "make: *** [Makefile:10: all] Error 2") {
					t.Errorf("missing failure tail in:\n%s", out)
				}
			} else if !bytes.Equal(r.Output, tc.data) {
				t.Errorf("raw output altered: len=%d want=%d", len(r.Output), len(tc.data))
			}
		})
	}
}
