// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"strings"
	"testing"
)

func TestBalanceFailedKeepsErrorsAndTail(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 400; i++ {
		fmt.Fprintf(&b, "compiling unit %d with a fairly long command line to pad the output\n", i)
		if i%100 == 50 {
			fmt.Fprintf(&b, "src/f%d.c:1:1: error: broken thing %d\n", i, i)
		}
	}
	b.WriteString("make: *** [all] Error 2\n")
	in := []byte(b.String())
	out, ok := BalanceFailed(in)
	if !ok || len(out) >= len(in)*3/4 {
		t.Fatalf("not cut: ok=%v %d of %d", ok, len(out), len(in))
	}
	for _, want := range []string{"error: broken thing 50", "error: broken thing 350", "make: *** [all] Error 2", "lines cut (downshift retrieve)"} {
		if !strings.Contains(string(out), want) {
			t.Errorf("missing %q", want)
		}
	}
}

func TestBalanceFailedSmallStaysWhole(t *testing.T) {
	in := []byte("a\nb\nerror: x\n")
	if _, ok := BalanceFailed(in); ok {
		t.Error("tiny output was cut")
	}
}
