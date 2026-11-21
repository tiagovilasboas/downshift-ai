// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

// fewLongLines is n lines of about size bytes each (few lines, many bytes);
// the middle line is mid when set.
func fewLongLines(n, size int, mid string) []byte {
	var b bytes.Buffer
	for i := 0; i < n; i++ {
		l := fmt.Sprintf("row %d ", i) + strings.Repeat("abcdefgh", size/8)
		if i == n/2 && mid != "" {
			l = mid
		}
		b.WriteString(l + "\n")
	}
	return b.Bytes()
}

func TestByteBudgetCut(t *testing.T) {
	const threshold = 4 << 10
	danger := "step 7 FAILED: assertion error in worker"
	cases := []struct {
		name      string
		data      []byte
		keep      bool
		exit      int
		balanced  bool
		wantCut   bool
		wantInOut string
	}{
		{"few long lines are cut", fewLongLines(20, 600, ""), true, 0, true, true, ""},
		{"danger line in the middle survives", fewLongLines(20, 600, danger), true, 0, true, true, danger},
		{"nothing is cut without keep", fewLongLines(20, 600, ""), false, 0, true, false, ""},
		{"below the threshold nothing is cut", fewLongLines(3, 600, ""), true, 0, true, false, ""},
		{"gain under 25 percent is not applied", fewLongLines(62, 64, ""), true, 0, true, false, ""},
		{"balanced off is not cut", fewLongLines(20, 600, ""), true, 0, false, false, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var stored []byte
			var keep func([]byte) (string, bool)
			if c.keep {
				keep = func(r []byte) (string, bool) { stored = append([]byte(nil), r...); return "0123456789ab", true }
			}
			res := CompactCapturedWithThreshold(nil, FamilyNone, c.data, c.exit, keep, c.balanced, threshold, "")
			cut := len(res.Output) < len(c.data)
			if cut != c.wantCut {
				t.Fatalf("cut=%v want %v (out %d of %d bytes)", cut, c.wantCut, len(res.Output), len(c.data))
			}
			if !c.wantCut {
				return
			}
			if !bytes.Equal(stored, c.data) {
				t.Error("CCR did not store the raw output: retrieve would not round-trip")
			}
			if res.KeptID != "0123456789ab" || !strings.Contains(string(res.Output), "downshift retrieve 0123456789ab") {
				t.Errorf("no retrieve marker: kept=%q", res.KeptID)
			}
			if len(res.Output)*4 > len(c.data)*3 {
				t.Errorf("gain under 25%%: %d of %d", len(res.Output), len(c.data))
			}
			if c.wantInOut != "" && !strings.Contains(string(res.Output), c.wantInOut) {
				t.Errorf("danger line lost from the cut:\n%s", res.Output)
			}
		})
	}
}
