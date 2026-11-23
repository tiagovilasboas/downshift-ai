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
	"github.com/tiagovilasboas/downshift-ai/internal/paths"
)

func bigFailingMakeOutput() string {
	var b strings.Builder
	for i := 0; i < 200; i++ {
		fmt.Fprintf(&b, "gcc -O2 -c src/unit%d.c -o obj/unit%d.o -Wall -Wextra\n", i, i)
		if i == 100 {
			b.WriteString("src/unit100.c:12:5: error: unknown type name widget_t\n")
		}
	}
	b.WriteString("make: *** [Makefile:10: all] Error 2\n")
	return b.String()
}

func TestCompactShell_AntiLoopAutoBypassesAfterTwoFailures(t *testing.T) {
	t.Setenv(paths.EnvStateDir, t.TempDir())
	sess := "0123456789abcdef"
	failOut := bigFailingMakeOutput()
	opts := Options{Keep: keepOK, Balanced: true, Session: sess}
	in := hookport.ToolOutput{
		Category:  core.CategoryShell,
		Command:   "make test",
		Stdout:    failOut,
		HasStdout: true,
		ExitCode:  2,
		Session:   sess,
	}

	first := Compact(allCaps, in, opts)
	if !first.Replaced || first.Detail != execwrap.DetailFallbackCut {
		t.Fatalf("1st failure = replaced:%v detail:%q, want fallback_cut", first.Replaced, first.Detail)
	}
	second := Compact(allCaps, in, opts)
	if !second.Replaced || second.Detail != execwrap.DetailFallbackCut {
		t.Fatalf("2nd failure = replaced:%v detail:%q, want fallback_cut", second.Replaced, second.Detail)
	}

	// Third identical failure in the same session delivers full raw output.
	third := Compact(allCaps, in, opts)
	if third.Replaced || third.Compacted || third.Detail != execwrap.DetailAntiLoopBypass || third.Output.Stdout != failOut {
		t.Fatalf("3rd failure = replaced:%v compacted:%v detail:%q raw_match:%v, want bypass with raw output",
			third.Replaced, third.Compacted, third.Detail, third.Output.Stdout == failOut)
	}

	// A passing command clears the streak so future failures can use fallback_cut again.
	passIn := hookport.ToolOutput{
		Category:  core.CategoryShell,
		Command:   "make test",
		Stdout:    bigLog(),
		HasStdout: true,
		ExitCode:  0,
		Session:   sess,
	}
	if pass := Compact(allCaps, passIn, opts); !pass.Replaced {
		t.Fatalf("passing run after streak = %+v, want compacted", pass)
	}
	fourth := Compact(allCaps, in, opts)
	if !fourth.Replaced || fourth.Detail != execwrap.DetailFallbackCut {
		t.Fatalf("failure after pass = replaced:%v detail:%q, want fallback_cut", fourth.Replaced, fourth.Detail)
	}
}
