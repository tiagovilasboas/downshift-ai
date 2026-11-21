// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"strings"
	"testing"
)

func shapedHunk(file string, ctx int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "diff --git a/%[1]s b/%[1]s\nindex 1a2b3c4..5d6e7f8 100644\n--- a/%[1]s\n+++ b/%[1]s\n", file)
	fmt.Fprintf(&b, "@@ -10,%d +10,%d @@ func f()\n", 2*ctx+1, 2*ctx+1)
	for i := 0; i < ctx; i++ {
		fmt.Fprintf(&b, " before line %d\n", i)
	}
	b.WriteString("-old value\n+new value\n")
	for i := 0; i < ctx; i++ {
		fmt.Fprintf(&b, " after line %d\n", i)
	}
	return b.String()
}

// changeHeavy is a diff that is almost all +/- lines: nothing worth cutting.
func changeHeavy() string {
	var b strings.Builder
	b.WriteString("diff --git a/a.go b/a.go\n--- a/a.go\n+++ b/a.go\n@@ -1,40 +1,40 @@\n")
	for i := 0; i < 20; i++ {
		fmt.Fprintf(&b, "-old line number %d\n+new line number %d\n", i, i)
	}
	return b.String()
}

const shapedHeader = "commit 0123456789abcdef0123456789abcdef01234567\nAuthor: A Dev <a@example.com>\nDate:   Mon Nov 2 10:00:00 2026 +0000\n\n    tidy the config loader\n\n"

func TestDiffShaped(t *testing.T) {
	plain := shapedHunk("a.go", 8) + shapedHunk("b.go", 8)
	binary := plain + "diff --git a/logo.png b/logo.png\nindex 1a2b3c4..5d6e7f8 100644\nBinary files a/logo.png and b/logo.png differ\n"
	rename := plain + "diff --git a/old.go b/new.go\nsimilarity index 95%\nrename from old.go\nrename to new.go\n"
	tests := []struct {
		name     string
		in       string
		wantOK   bool
		mustKeep []string
	}{
		{"git show with message", shapedHeader + plain, true,
			[]string{"commit 0123456789abcdef0123456789abcdef01234567", "Author: A Dev <a@example.com>", "Date:   Mon Nov 2", "    tidy the config loader", "-old value", "+new value"}},
		{"plain diff", plain, true, []string{"-old value", "+new value", "@@ -10,17 +10,17 @@ func f()", "--- a/a.go", "+++ b/b.go"}},
		{"diff after pipe noise", "12 files changed\n" + plain, true, []string{"12 files changed", "-old value"}},
		{"binary file diff", shapedHeader + binary, true, []string{"Binary files a/logo.png and b/logo.png differ", "-old value"}},
		{"rename", shapedHeader + rename, true, []string{"rename from old.go", "rename to new.go", "similarity index 95%"}},
		{"no diff", "just some text\nnothing here\n", false, nil},
		{"change-heavy diff under gain", changeHeavy(), false, nil},
		{"crlf", strings.ReplaceAll(shapedHeader+plain, "\n", "\r\n"), true, []string{"-old value\r", "+new value\r", "tidy the config loader\r"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out, ok := DiffShaped([]byte(tc.in))
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if !ok {
				if string(out) != tc.in {
					t.Fatal("false result must return the input unchanged")
				}
				return
			}
			if len(out) >= len(tc.in) {
				t.Fatalf("no gain: %d >= %d", len(out), len(tc.in))
			}
			for _, k := range tc.mustKeep {
				if !strings.Contains(string(out), k) {
					t.Errorf("lost %q", k)
				}
			}
			// CRLF index lines are kept (the rule matches LF only): safe, just larger.
			if !strings.Contains(tc.in, "\r") && strings.Contains(string(out), "index 1a2b3c4") {
				t.Error("index line should be dropped")
			}
			if !changeLinesPreserved([]byte(tc.in), out) {
				t.Error("a change line was dropped")
			}
		})
	}
}

func TestDiffShapedKeepsStrayDanger(t *testing.T) {
	in := "fatal: bad object\n" + shapedHunk("a.go", 8)
	if _, ok := DiffShaped([]byte(in)); ok {
		t.Fatal("a stray danger line must keep the output raw")
	}
}
