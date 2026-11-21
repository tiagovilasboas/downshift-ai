// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"fmt"
	"strings"
	"testing"
)

// grepRows builds files*perFile grep -n rows and the match text of each.
func grepRows(files, perFile int) (string, []string) {
	var b strings.Builder
	var texts []string
	for f := 0; f < files; f++ {
		for i := 1; i <= perFile; i++ {
			text := fmt.Sprintf("\tvalue := compute(%d, %d)", f, i)
			texts = append(texts, text)
			fmt.Fprintf(&b, "internal/some/deep/package%d/handler_file.go:%d:%s\n", f, i, text)
		}
	}
	return b.String(), texts
}

func TestShapesInsideLimit(t *testing.T) {
	multi, multiTexts := grepRows(4, 15)
	single, singleTexts := grepRows(1, 60)
	cases := []struct {
		name, command, in string
		texts             []string
		wantShrink        bool
	}{
		{"multi-file head", "grep -rn err . | head -80", multi, multiTexts, true},
		{"multi-file tail", "grep -rn err . | tail -80", multi, multiTexts, true},
		{"single-file head", "grep -rn err f.go | head -80", single, singleTexts, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if s := ShapesFor(c.command, []byte(c.in)); !s.On || !s.Limited {
				t.Fatalf("shapes %+v", s)
			}
			res := shapedRun(c.command, c.in, true)
			if c.wantShrink && (!res.Compacted || len(res.Output) >= len(c.in)) {
				t.Fatalf("not grouped: compacted %v %d >= %d", res.Compacted, len(res.Output), len(c.in))
			}
			t.Logf("%d -> %d bytes", len(c.in), len(res.Output))
			if len(res.Output) > len(c.in) {
				t.Fatalf("output grew: %d > %d", len(res.Output), len(c.in))
			}
			if strings.Contains(string(res.Output), "more in") {
				t.Errorf("byte cut inside the limit:\n%s", res.Output)
			}
			for i, text := range c.texts {
				if !strings.Contains(string(res.Output), text) {
					t.Fatalf("row %d lost: %q", i, text)
				}
			}
		})
	}
}
