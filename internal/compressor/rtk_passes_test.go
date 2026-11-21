// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"strings"
	"testing"
)

var testPad = strings.Repeat("some ordinary short line of text\n", 40)

func TestCapLongLines(t *testing.T) {
	long := strings.Repeat("abcdefghij", 60) // 600 bytes
	cases := []struct {
		name string
		in   string
		ok   bool
		want string
	}{
		{"long line capped", testPad + "head\n" + long + "\ntail\n", true, long[:200] + " … (400 bytes cut)"},
		{"short lines no-op", strings.Repeat("short line\n", 50), false, ""},
		{"below gain no-op", strings.Repeat("some ordinary short line of text\n", 100) + strings.Repeat("y", 301) + "\n", false, ""},
		{"danger line kept whole", "ok\n" + strings.Repeat("error ", 100) + "\n", false, ""},
		{"utf8 not split", testPad + strings.Repeat("é", 400) + "\n", true, "…"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out, ok := CapLongLines([]byte(c.in))
			if ok != c.ok {
				t.Fatalf("ok = %v, want %v", ok, c.ok)
			}
			if !ok {
				if string(out) != c.in {
					t.Fatal("no-op changed the data")
				}
				return
			}
			if !strings.Contains(string(out), c.want) || len(out) >= len(c.in) {
				t.Fatalf("bad cap: %q", out)
			}
			if !strings.Contains(string(out), "head\n") && c.name == "long line capped" {
				t.Fatal("short line lost")
			}
		})
	}
}

func TestFoldNearRepeats(t *testing.T) {
	var ts strings.Builder
	for i := 0; i < 10; i++ {
		fmt.Fprintf(&ts, "2026-11-21T10:00:%02d worker %d processed job id=%d\n", i, i%3, 1000+i)
	}
	cases := []struct {
		name string
		in   string
		ok   bool
		want string
	}{
		{"timestamps fold", testPad + "start\n" + ts.String() + "end\n", true, "(8 similar lines)"},
		{"two lines no-op", "a 1 b\na 2 b\nsomething else entirely here\n", false, ""},
		{"tiny output no-op", "a 1\na 2\na 3\na 4\n", false, ""},
		{"danger lines never fold", strings.Repeat("error code 1\nerror code 2\nerror code 3\n", 5), false, ""},
		{"blank lines never fold", strings.Repeat("\n", 20), false, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out, ok := FoldNearRepeats([]byte(c.in))
			if ok != c.ok {
				t.Fatalf("ok = %v, want %v\n%s", ok, c.ok, out)
			}
			if !ok {
				if string(out) != c.in {
					t.Fatal("no-op changed the data")
				}
				return
			}
			s := string(out)
			if !strings.Contains(s, c.want) || !strings.Contains(s, ":00 worker 0") || !strings.Contains(s, "id=1009") || !strings.HasSuffix(s, "end\n") {
				t.Fatalf("bad fold: %q", s)
			}
		})
	}
}

func TestCondenseShellJSON(t *testing.T) {
	var items []string
	for i := 0; i < 40; i++ {
		items = append(items, fmt.Sprintf(`{"id":%d,"node_id":"MDQ6VXNlcj%d","url":"https://api.example.com/users/%d/very/long/path/segment/that/goes/on/and/on/forever","name":"n%d"}`, i, i, i, i))
	}
	valid := "[" + strings.Join(items, ",") + "]"
	cases := []struct {
		name string
		in   string
		ok   bool
	}{
		{"valid json pruned", valid, true},
		{"invalid json no-op", valid[:len(valid)-1], false},
		{"plain text no-op", strings.Repeat("hello world\n", 100), false},
		{"small json no-op", `{"a":1}`, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out, ok := CondenseShellJSON([]byte(c.in))
			if ok != c.ok {
				t.Fatalf("ok = %v, want %v", ok, c.ok)
			}
			if ok && len(out) >= len(c.in) {
				t.Fatal("not smaller")
			}
			if !ok && string(out) != c.in {
				t.Fatal("no-op changed the data")
			}
		})
	}
}

func TestLargeDiffContextKeepsEveryChange(t *testing.T) {
	var b strings.Builder
	b.WriteString("diff --git a/f.go b/f.go\nindex 1..2 100644\n--- a/f.go\n+++ b/f.go\n@@ -1,60 +1,60 @@\n")
	var changes []string
	for i := 0; i < 60; i++ {
		switch {
		case i%12 == 6:
			b.WriteString(fmt.Sprintf("-old line %d\n", i))
			b.WriteString(fmt.Sprintf("+new line %d\n", i))
			changes = append(changes, fmt.Sprintf("-old line %d", i), fmt.Sprintf("+new line %d", i))
		default:
			b.WriteString(fmt.Sprintf(" context line number %d with padding text\n", i))
		}
	}
	for b.Len() <= largeDiffBytes {
		b.WriteString(" filler context line with enough text to grow the diff size\n")
	}
	in := b.String()
	out, _, ok := rewriteUnifiedDiff([]byte(in), true)
	if !ok {
		t.Fatal("diff not recognised")
	}
	for _, c := range changes {
		if !strings.Contains(string(out), c+"\n") {
			t.Fatalf("lost change line %q", c)
		}
	}
	if !strings.Contains(string(out), "@@ -1,60 +1,60 @@") || !strings.Contains(string(out), "+++ b/f.go") {
		t.Fatal("lost hunk or file header")
	}
	if n := strings.Count(string(out), "\n context"); n > len(changes) {
		t.Fatalf("context not reduced to 1 line: %d context lines for %d change lines", n, len(changes))
	}
}
