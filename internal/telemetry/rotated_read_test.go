// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/telemetry"
)

func TestReadEventsWithRotated_ChronologicalOrder(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "events.jsonl")
	t.Setenv("DOWNSHIFT_EVENT_LOG", logPath)

	writeLines := func(name string, ids ...string) {
		t.Helper()
		var b strings.Builder
		for _, id := range ids {
			fmt.Fprintf(&b, `{"harness":"claudecode","outcome":"allow","correlation_id":%q}`+"\n", id)
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(b.String()), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	writeLines("events.jsonl.2026-10-01T00-00-00", "d1", "d2")
	writeLines("events.jsonl.2026-10-02T00-00-00", "d3")
	writeLines("events.jsonl", "d4", "d5")
	// Lock file must be ignored.
	if err := os.WriteFile(filepath.Join(dir, "events.jsonl.lock"), []byte("lock"), 0o600); err != nil {
		t.Fatal(err)
	}

	events, err := telemetry.ReadEvents()
	if err != nil {
		t.Fatalf("ReadEvents: %v", err)
	}
	want := []string{"d1", "d2", "d3", "d4", "d5"}
	if len(events) != len(want) {
		t.Fatalf("got %d events, want %d", len(events), len(want))
	}
	for i, id := range want {
		if events[i].CorrelationID != id {
			t.Errorf("events[%d].CorrelationID = %q, want %q", i, events[i].CorrelationID, id)
		}
	}
}

func TestReadEventsWithRotated_ByteCapKeepsNewest(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "events.jsonl")

	line := func(id string) string {
		return fmt.Sprintf(`{"harness":"claudecode","outcome":"allow","correlation_id":%q}`+"\n", id)
	}
	l1 := line("old-1") + line("old-2")
	l2 := line("mid-1") + line("mid-2")
	l3 := line("new-1")

	if err := os.WriteFile(filepath.Join(dir, "events.jsonl.2026-10-01T00-00-00"), []byte(l1), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "events.jsonl.2026-10-02T00-00-00"), []byte(l2), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(logPath, []byte(l3), 0o600); err != nil {
		t.Fatal(err)
	}

	// Cap allows live log (l3) + mid backup (l2), excluding old backup (l1).
	capBytes := int64(len(l3) + len(l2))
	events, err := telemetry.ReadEventsWithRotatedFrom(logPath, capBytes)
	if err != nil {
		t.Fatalf("ReadEventsWithRotatedFrom: %v", err)
	}
	want := []string{"mid-1", "mid-2", "new-1"}
	if len(events) != len(want) {
		t.Fatalf("got %d events (%v), want %v", len(events), events, want)
	}
	for i, id := range want {
		if events[i].CorrelationID != id {
			t.Errorf("events[%d].CorrelationID = %q, want %q", i, events[i].CorrelationID, id)
		}
	}
}

func TestReadEventsWithRotated_SingleLargeBackupTailed(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "events.jsonl")

	var b strings.Builder
	for i := 0; i < 20; i++ {
		fmt.Fprintf(&b, `{"harness":"claudecode","outcome":"allow","correlation_id":"d-%02d"}`+"\n", i)
	}
	backupData := b.String()
	if err := os.WriteFile(filepath.Join(dir, "events.jsonl.2026-10-01T00-00-00"), []byte(backupData), 0o600); err != nil {
		t.Fatal(err)
	}
	// Empty live log right after rotation.
	if err := os.WriteFile(logPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	// Cap smaller than the backup file: should tail-read the newest events, not drop to zero.
	capBytes := int64(len(backupData) / 2)
	events, err := telemetry.ReadEventsWithRotatedFrom(logPath, capBytes)
	if err != nil {
		t.Fatalf("ReadEventsWithRotatedFrom: %v", err)
	}
	if len(events) == 0 || len(events) >= 20 {
		t.Fatalf("expected partial tail of backup (0 < n < 20), got %d", len(events))
	}
	if events[len(events)-1].CorrelationID != "d-19" {
		t.Errorf("last event = %q, want d-19", events[len(events)-1].CorrelationID)
	}
}

func TestReadEventsWithRotated_MissingDirReturnsEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing", "events.jsonl")
	events, err := telemetry.ReadEventsWithRotatedFrom(path, telemetry.DefaultRotatedMaxBytes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("expected 0 events, got %d", len(events))
	}
}

func TestReadEventsTailUnlinked_UnaffectedByBackups(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "events.jsonl")
	t.Setenv("DOWNSHIFT_EVENT_LOG", logPath)

	backupLine := `{"harness":"claudecode","outcome":"allow","correlation_id":"backup-only"}` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "events.jsonl.2026-10-01T00-00-00"), []byte(backupLine), 0o600); err != nil {
		t.Fatal(err)
	}

	// Write a decision, an unlinked usage, and a resolved record in the live log.
	liveLines := strings.Join([]string{
		`{"harness":"claudecode","outcome":"rewrite_emitted","correlation_id":"dec-1","complexity":"SIMPLE","requested_model":"sonnet","verdict":"DOWNSHIFT","tier":"SMALL"}`,
		`{"harness":"claudecode","outcome":"usage","agent_hash":"h1"}`,
		`{"harness":"claudecode","outcome":"resolved","agent_hash":"h1","linked_decision":"dec-1"}`,
	}, "\n") + "\n"
	if err := os.WriteFile(logPath, []byte(liveLines), 0o600); err != nil {
		t.Fatal(err)
	}

	events, err := telemetry.ReadEventsTailUnlinked(4096)
	if err != nil {
		t.Fatalf("ReadEventsTailUnlinked: %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("got %d events, want 3 (live log only)", len(events))
	}
	for _, ev := range events {
		if ev.CorrelationID == "backup-only" {
			t.Fatal("ReadEventsTailUnlinked must not read rotated backups")
		}
		if ev.Outcome == telemetry.OutcomeUsage && ev.LinkedDecision != "" {
			t.Fatalf("ReadEventsTailUnlinked must not run linkLateUsage, got LinkedDecision=%q", ev.LinkedDecision)
		}
	}
}
