// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/execwrap"
	"github.com/tiagovilasboas/downshift-ai/internal/telemetry"
)

func TestSession_WorkingMemoryIndicator(t *testing.T) {
	panelFixtureEnv(t)
	appendPanelFixture(t)

	execPath, err := execwrap.EventsPath()
	if err != nil {
		t.Fatal(err)
	}
	// 1,324,000 bytes saved in this session => 331k tokens. The fixture's
	// own exec events carry no session, so they belong to no session and
	// stay out of the session-scoped indicator.
	big := execwrap.Event{
		TS:           time.Now().UTC().Add(-10 * time.Minute).Format(time.RFC3339),
		RawBytes:     1400000,
		EmittedBytes: 76000,
		Compacted:    true,
		Session:      telemetry.HashSessionID("s-1"),
	}
	// Another session's savings must not leak into this session's line.
	other := execwrap.Event{
		TS:           time.Now().UTC().Add(-5 * time.Minute).Format(time.RFC3339),
		RawBytes:     900000,
		EmittedBytes: 0,
		Compacted:    true,
		Session:      telemetry.HashSessionID("s-other"),
	}
	if err := execwrap.Append(execPath, other); err != nil {
		t.Fatal(err)
	}
	if err := execwrap.Append(execPath, big); err != nil {
		t.Fatal(err)
	}

	out, errOut, rc := sessionCmd(t)
	if rc != 0 {
		t.Fatalf("session rc=%d stderr=%s", rc, errOut)
	}
	wantLine := "Context preserved (this session): ~331k tokens of noisy scrollback kept out of working memory"
	if !strings.Contains(out, wantLine) {
		t.Fatalf("session output missing %q:\n%s", wantLine, out)
	}

	jsonOut, errOut, rc := sessionCmd(t, "--json")
	if rc != 0 {
		t.Fatalf("session --json rc=%d stderr=%s", rc, errOut)
	}
	var parsed struct {
		TokensCompactedEstimate *int64 `json:"tokens_compacted_estimate"`
		WorkingMemory           *struct {
			TokensCompactedEstimate int64  `json:"tokens_compacted_estimate"`
			BytesCompactedTotal     int64  `json:"bytes_compacted_total"`
			Scope                   string `json:"scope"`
			Line                    string `json:"line"`
		} `json:"working_memory"`
	}
	if err := json.Unmarshal([]byte(jsonOut), &parsed); err != nil {
		t.Fatalf("unmarshal session --json: %v\n%s", err, jsonOut)
	}
	if parsed.TokensCompactedEstimate != nil {
		t.Fatal("top-level tokens_compacted_estimate duplicates working_memory; want it absent")
	}
	wm := parsed.WorkingMemory
	if wm == nil || wm.TokensCompactedEstimate != 331000 || wm.BytesCompactedTotal != 1324000 || wm.Scope != "this session" || wm.Line != wantLine {
		t.Fatalf("working_memory = %+v, want 331000 tokens, 1324000 bytes, this session, line %q", wm, wantLine)
	}

	full := panelJSON(t)
	brief := panelJSON(t, "--brief")
	if string(full["working_memory_line"]) != `"`+wantLine+`"` {
		t.Fatalf("full panel working_memory_line = %s, want %q", full["working_memory_line"], wantLine)
	}
	if string(brief["working_memory_line"]) != string(full["working_memory_line"]) {
		t.Fatalf("brief working_memory_line = %s, full = %s", brief["working_memory_line"], full["working_memory_line"])
	}
}
