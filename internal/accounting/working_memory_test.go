// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package accounting

import (
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/execwrap"
)

func TestBuildWorkingMemory(t *testing.T) {
	now := time.Now().UTC()
	ts := now.Add(-time.Hour).Format(time.RFC3339)

	t.Run("empty", func(t *testing.T) {
		nums := Compute(nil, nil, Scope{Kind: ScopeDays, Days: 30})
		wm := BuildWorkingMemory(nums)
		if wm.TokensCompactedEstimate != 0 || wm.BytesCompactedTotal != 0 || wm.Line != "" {
			t.Fatalf("empty WorkingMemory = %+v, want zero and empty line", wm)
		}
	})

	t.Run("sub_token_bytes", func(t *testing.T) {
		execs := []execwrap.Event{{TS: ts, RawBytes: 10, EmittedBytes: 7, Compacted: true}}
		nums := Compute(nil, execs, Scope{Kind: ScopeDays, Days: 30})
		wm := BuildWorkingMemory(nums)
		if wm.BytesCompactedTotal != 3 || wm.TokensCompactedEstimate != 0 {
			t.Fatalf("sub_token WorkingMemory = %+v, want 3 bytes and 0 tokens", wm)
		}
		want := "Context preserved: 3 bytes of noisy scrollback kept out of working memory"
		if wm.Line != want {
			t.Fatalf("Line = %q, want %q", wm.Line, want)
		}
	})

	t.Run("thousands_of_tokens", func(t *testing.T) {
		// 1,324,000 saved bytes / 4 = 331,000 tokens -> 331k
		execs := []execwrap.Event{{TS: ts, RawBytes: 1400000, EmittedBytes: 76000, Compacted: true}}
		nums := Compute(nil, execs, Scope{Kind: ScopeDays, Days: 30})
		wm := BuildWorkingMemory(nums)
		if wm.TokensCompactedEstimate != 331000 || wm.BytesCompactedTotal != 1324000 || wm.CompactedCommands != 1 {
			t.Fatalf("WorkingMemory = %+v", wm)
		}
		want := "Context preserved: ~331k tokens of noisy scrollback kept out of working memory"
		if wm.Line != want {
			t.Fatalf("Line = %q, want %q", wm.Line, want)
		}
	})
}
