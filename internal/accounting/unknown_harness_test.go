// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package accounting

import (
	"reflect"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/execwrap"
)

func TestBuildCompactionDetail_UnknownReasons(t *testing.T) {
	now := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	ts := now.Add(-time.Hour).Format(time.RFC3339)
	evs := []execwrap.Event{
		{TS: ts, Entry: "direct"}, {TS: ts, Harness: "unknown", Entry: "direct"},
		{TS: ts, Entry: "post_tool_use"}, {TS: ts, Entry: "hook"}, {TS: ts},
		{TS: ts, Harness: "codex", Entry: "direct"},
	}
	skips := []execwrap.SkipEvent{{TS: ts, Reason: "metachar"}}
	got := BuildCompactionDetail(evs, skips, Scope{Kind: ScopeAll}, now)
	want := []ReasonCount{
		{Reason: "no_harness_env", Count: 2}, {Reason: "hook_no_harness", Count: 1},
		{Reason: "legacy_row", Count: 1}, {Reason: "post_tool_use", Count: 1}, {Reason: "skip_row", Count: 1},
	}
	seen := false
	for _, h := range got.Harnesses {
		if h.Harness == "codex" && h.UnknownReasons != nil {
			t.Errorf("known harness carries unknown reasons: %+v", h.UnknownReasons)
		}
		if h.Harness == UnknownHarness {
			seen = true
			if !reflect.DeepEqual(h.UnknownReasons, want) {
				t.Errorf("unknown reasons = %+v, want %+v", h.UnknownReasons, want)
			}
		}
	}
	if !seen {
		t.Fatal("no unknown harness row")
	}
}
