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
)

func TestSession_WorkingMemoryIndicator(t *testing.T) {
	panelFixtureEnv(t)
	appendPanelFixture(t)

	execPath, err := execwrap.EventsPath()
	if err != nil {
		t.Fatal(err)
	}
	// Add 1,324,000 bytes saved (400+300 already in fixture -> add 1,323,700 saved => total 1,324,000 => 331k tokens).
	big := execwrap.Event{
		TS:           time.Now().UTC().Add(-10 * time.Minute).Format(time.RFC3339),
		RawBytes:     1400000,
		EmittedBytes: 76300,
		Compacted:    true,
	}
	if err := execwrap.Append(execPath, big); err != nil {
		t.Fatal(err)
	}

	out, errOut, rc := sessionCmd(t)
	if rc != 0 {
		t.Fatalf("session rc=%d stderr=%s", rc, errOut)
	}
	wantLine := "Context preserved: ~331k tokens of noisy scrollback kept out of working memory"
	if !strings.Contains(out, wantLine) {
		t.Fatalf("session output missing %q:\n%s", wantLine, out)
	}

	jsonOut, errOut, rc := sessionCmd(t, "--json")
	if rc != 0 {
		t.Fatalf("session --json rc=%d stderr=%s", rc, errOut)
	}
	var parsed struct {
		TokensCompactedEstimate int64 `json:"tokens_compacted_estimate"`
		BytesCompactedTotal     int64 `json:"bytes_compacted_total"`
		WorkingMemory           *struct {
			Line string `json:"line"`
		} `json:"working_memory"`
	}
	if err := json.Unmarshal([]byte(jsonOut), &parsed); err != nil {
		t.Fatalf("unmarshal session --json: %v\n%s", err, jsonOut)
	}
	if parsed.TokensCompactedEstimate != 331000 || parsed.BytesCompactedTotal != 1324000 {
		t.Fatalf("json totals = %+v, want 331000 tokens and 1324000 bytes", parsed)
	}
	if parsed.WorkingMemory == nil || parsed.WorkingMemory.Line != wantLine {
		t.Fatalf("working_memory = %+v, want line %q", parsed.WorkingMemory, wantLine)
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
