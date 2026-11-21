// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"strings"
	"testing"
)

func TestGroupGrepLines(t *testing.T) {
	const p = "internal/very/long/package/path/handler.go"
	cases := []struct {
		name   string
		in     []string
		want   []string // nil: the input comes back unchanged
		wantOK bool
	}{
		{
			name:   "numbered lines of one file",
			in:     []string{p + ":10:\tfoo()", p + ":20:\tbar()", p + ":30:\tbaz()"},
			want:   []string{p + ":", "  10:\tfoo()", "  20:\tbar()", "  30:\tbaz()"},
			wantOK: true,
		},
		{
			name:   "two files keep first-seen order and per-file order",
			in:     []string{p + ":1:a", "other/dir/file.go:5:x", p + ":2:b", "other/dir/file.go:6:y"},
			want:   []string{p + ":", "  1:a", "  2:b", "other/dir/file.go:", "  5:x", "  6:y"},
			wantOK: true,
		},
		{
			name:   "path:text without line numbers",
			in:     []string{p + ":alpha", p + ":beta", p + ":gamma"},
			want:   []string{p + ":", "  :alpha", "  :beta", "  :gamma"},
			wantOK: true,
		},
		{
			name:   "a path seen once stays verbatim",
			in:     []string{p + ":1:a", p + ":2:b", "lonely/file.go:9:c"},
			want:   []string{p + ":", "  1:a", "  2:b", "lonely/file.go:9:c"},
			wantOK: true,
		},
		{
			name:   "danger line stays verbatim and is not grouped",
			in:     []string{p + ":1:a", p + ":2:an error here", p + ":3:c", p + ":4:d", p + ":5:e", p + ":6:f"},
			wantOK: true,
		},
		{
			name:   "windows paths",
			in:     []string{`C:\proj\deep\dir\y.go:12:t1`, `C:\proj\deep\dir\y.go:13:t2`, `C:\proj\deep\dir\y.go:14:t3`},
			want:   []string{`C:\proj\deep\dir\y.go:`, "  12:t1", "  13:t2", "  14:t3"},
			wantOK: true,
		},
		{
			name:   "single line is not grouped",
			in:     []string{p + ":1:only"},
			wantOK: false,
		},
		{
			name:   "plain prose does not match",
			in:     []string{"Note: this is text", "Another: sentence here", "http://example.com/x", "done"},
			wantOK: false,
		},
		{
			name:   "mixed output under 80 percent does not match",
			in:     []string{p + ":1:a", p + ":2:b", "free text one", "free text two", "free text three"},
			wantOK: false,
		},
		{
			name:   "mixed output over 80 percent keeps stray lines in place",
			in:     []string{p + ":1:a", p + ":2:b", p + ":3:c", p + ":4:d", p + ":5:e", "-- separator"},
			want:   []string{p + ":", "  1:a", "  2:b", "  3:c", "  4:d", "  5:e", "-- separator"},
			wantOK: true,
		},
		{
			name:   "gain under 15 percent returns the input",
			in:     []string{"a.go:1:" + strings.Repeat("x", 80), "a.go:2:" + strings.Repeat("y", 80)},
			wantOK: false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := strings.Join(c.in, "\n") + "\n"
			got, ok := GroupGrepLines([]byte(in))
			if c.name == "danger line stays verbatim and is not grouped" {
				checkDangerCase(t, in, got, ok)
				return
			}
			if ok != c.wantOK {
				t.Fatalf("ok = %v, want %v\n%s", ok, c.wantOK, got)
			}
			if !ok {
				if string(got) != in {
					t.Errorf("input changed on false:\n%s", got)
				}
				return
			}
			if want := strings.Join(c.want, "\n") + "\n"; string(got) != want {
				t.Errorf("got:\n%s\nwant:\n%s", got, want)
			}
		})
	}
}

// checkDangerCase asserts the danger line survives byte for byte and the
// other lines of its file are still grouped.
func checkDangerCase(t *testing.T, in string, got []byte, ok bool) {
	t.Helper()
	if !ok {
		t.Fatalf("want grouping applied around the danger line\n%s", got)
	}
	const danger = "internal/very/long/package/path/handler.go:2:an error here"
	if !strings.Contains(string(got), danger+"\n") {
		t.Errorf("danger line altered:\n%s", got)
	}
	if !strings.Contains(string(got), "  3:c\n  4:d") {
		t.Errorf("safe lines not grouped:\n%s", got)
	}
	_ = in
}

// TestGroupGrepLinesLossless expands the grouped text back and compares it to
// the input: every line number and text survives, in order within a path.
func TestGroupGrepLinesLossless(t *testing.T) {
	var in []string
	for _, f := range []string{"pkg/one/file_one.go", "pkg/two/file_two.go", "pkg/three/file_three.go"} {
		for i := 1; i <= 6; i++ {
			in = append(in, f+":"+strings.Repeat("1", i)+":    value := compute("+f+")")
		}
	}
	raw := strings.Join(in, "\n") + "\n"
	got, ok := GroupGrepLines([]byte(raw))
	if !ok {
		t.Fatal("not grouped")
	}
	var back []string
	path := ""
	for _, l := range strings.Split(strings.TrimSuffix(string(got), "\n"), "\n") {
		if !strings.HasPrefix(l, "  ") {
			path = strings.TrimSuffix(l, ":")
			continue
		}
		back = append(back, path+":"+strings.TrimPrefix(l, "  "))
	}
	if strings.Join(back, "\n") != strings.Join(in, "\n") {
		t.Errorf("expansion differs from input:\n%s", got)
	}
}

func TestGroupGrepLinesCutsMinifiedLine(t *testing.T) {
	const p = "dist/bundle/app.min.js"
	long := strings.Repeat("x", 900)
	raw := p + ":1:" + long + "\n" + p + ":2:short\n" + p + ":3:short\n"
	got, ok := GroupGrepLines([]byte(raw))
	if !ok {
		t.Fatal("not grouped")
	}
	if len(got) > 600 || !strings.Contains(string(got), "bytes cut") {
		t.Errorf("long line not cut (%d bytes)", len(got))
	}
}
