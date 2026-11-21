// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"strings"
	"testing"
)

func TestShapesLossyGates(t *testing.T) {
	data := []byte(pad() + strings.Repeat("abcdefghij", 80) + "\ny\n")
	keep := func([]byte) (string, bool) { return "0123456789ab", true }
	refuse := func([]byte) (string, bool) { return "", false }
	cases := []struct {
		name string
		s    Shapes
		keep func([]byte) (string, bool)
		want bool
	}{
		{"stored runs the cap", Shapes{On: true}, keep, true},
		{"no store port", Shapes{On: true}, nil, false},
		{"store refuses", Shapes{On: true}, refuse, false},
		{"agent head or tail limit", Shapes{On: true, Limited: true}, keep, false},
		{"file read keeps long lines", Shapes{On: true, FileRead: true}, keep, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out, ok := c.s.rewriteStored(data, true, c.keep)
			if ok != c.want || (ok && len(out) >= len(data)) {
				t.Fatalf("ok = %v, want %v", ok, c.want)
			}
			if ok && !strings.HasSuffix(string(out), "downshift retrieve 0123456789ab]\n") {
				t.Fatalf("no retrieve marker: %q", out[len(out)-80:])
			}
		})
	}
}

func TestReadsFile(t *testing.T) {
	for cmd, want := range map[string]bool{
		"cat README.md": true, "sed -n 1,40p a.md": true, "bat x": true,
		"gh api repos/x/y": false, "curl -s https://x": false,
	} {
		if got := readsFile(cmd); got != want {
			t.Errorf("readsFile(%q) = %v, want %v", cmd, got, want)
		}
	}
}

// pad is 40 distinct lines (digit-free, so no fold applies) of ordinary text.
func pad() string {
	var b strings.Builder
	for i := 0; i < 40; i++ {
		b.WriteString("row " + string(rune('a'+i/26)) + string(rune('a'+i%26)) + " of ordinary text\n")
	}
	return b.String()
}
