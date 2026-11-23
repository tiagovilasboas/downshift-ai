// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry_test

import (
	"github.com/tiagovilasboas/downshift-ai/internal/telemetry"
	"github.com/tiagovilasboas/downshift-ai/internal/telemetry/readout"
	"os"
	"strings"
	"testing"
	"time"
)

func TestSpawnStateOf(t *testing.T) {
	cases := []struct {
		name string
		ev   telemetry.Event
		want string
	}{
		{"rewrite emitted", telemetry.Event{Outcome: telemetry.OutcomeRewriteEmitted}, telemetry.StateSwitched},
		{"allow with nothing to do", telemetry.Event{Outcome: telemetry.OutcomeAllow}, telemetry.StateObserved},
		{"guardrail correction", telemetry.Event{Outcome: telemetry.OutcomeAllow, Corrections: []string{"R3"}}, telemetry.StateHeld},
		{"held beats an emitted outcome", telemetry.Event{Outcome: telemetry.OutcomeRewriteEmitted, Corrections: []string{"R8"}}, telemetry.StateHeld},
		{"held reason code", telemetry.Event{Outcome: telemetry.OutcomeAllow, ReasonCodes: []string{"HELD_UNCONFIDENT_UPSHIFT"}}, telemetry.StateHeld},
		{"credit held", telemetry.Event{Outcome: telemetry.OutcomeAllow, CreditHeld: true}, telemetry.StateHeld},
		{"paused", telemetry.Event{Outcome: telemetry.OutcomeBaseline, ReasonCodes: []string{telemetry.ReasonPaused}}, telemetry.StatePaused},
		{"old control-group baseline line", telemetry.Event{Outcome: telemetry.OutcomeBaseline, ReasonCodes: []string{"CONTROL"}}, telemetry.StateUnrouted},
		{"old no-route baseline line", telemetry.Event{Outcome: telemetry.OutcomeBaseline}, telemetry.StateUnrouted},
		{"blocked", telemetry.Event{Outcome: telemetry.OutcomeBlocked}, telemetry.StateBlocked},
	}
	for _, c := range cases {
		if got := telemetry.SpawnStateOf(c.ev); got != c.want {
			t.Errorf("%s: got %q want %q", c.name, got, c.want)
		}
	}
}

func TestSpawnRecordLine(t *testing.T) {
	cases := []struct {
		name string
		ev   telemetry.Event
		want string
	}{
		{"model switch with effort", telemetry.Event{Outcome: telemetry.OutcomeRewriteEmitted, FromModel: "claude-opus-5-5", ToModel: "claude-haiku-4-5",
			RequestedEffort: "high", FinalEffort: "low", Complexity: "TRIVIAL"}, "switched opus → haiku, effort high → low · TRIVIAL"},
		{"effort only", telemetry.Event{Outcome: telemetry.OutcomeRewriteEmitted, FromModel: "gpt-6-sol", ToModel: "gpt-6-sol",
			RequestedEffort: "unknown", FinalEffort: "low"}, "switched effort on gpt-6-sol"},
		{"held says what it would have done", telemetry.Event{Outcome: telemetry.OutcomeAllow, FromModel: "claude-opus-5-5", ToModel: "claude-opus-5-5",
			RecommendedModel: "claude-haiku-4-5", ReasonCodes: []string{"HELD_UNCONFIDENT_DOWNSHIFT"}, Complexity: "MEDIUM"},
			"held on opus (would be haiku) [HELD_UNCONFIDENT_DOWNSHIFT] · MEDIUM"},
		{"observed", telemetry.Event{Outcome: telemetry.OutcomeAllow, FromModel: "unknown", ToModel: "unknown"}, "observed, spawn unchanged on ?"},
	}
	for _, c := range cases {
		if got := telemetry.NewSpawnRecord(c.ev).Line(); got != c.want {
			t.Errorf("%s:\n got %q\nwant %q", c.name, got, c.want)
		}
	}
}

func TestWriteLiveCountsAndResetsPerSession(t *testing.T) {
	t.Setenv("DOWNSHIFT_EVENT_LOG", t.TempDir()+"/events.jsonl")
	rec := func(sess, state string) telemetry.SpawnRecord {
		return telemetry.SpawnRecord{State: state, Harness: "claude-code", Session: sess}
	}
	for _, r := range []telemetry.SpawnRecord{rec("aaaaaaaa", telemetry.StateSwitched), rec("aaaaaaaa", telemetry.StateHeld), rec("aaaaaaaa", telemetry.StateSwitched)} {
		if err := telemetry.WriteLive(r); err != nil {
			t.Fatal(err)
		}
	}
	l, err := telemetry.ReadLive()
	if err != nil || l.Counts[telemetry.StateSwitched] != 2 || l.Counts[telemetry.StateHeld] != 1 || l.Last.State != telemetry.StateSwitched {
		t.Fatalf("live: %+v err=%v", l, err)
	}
	if !l.Active(time.Now()) || l.Active(time.Now().Add(telemetry.ActiveWindow+time.Second)) {
		t.Fatal("active only inside the window")
	}
	if err := telemetry.WriteLive(rec("bbbbbbbb", telemetry.StateObserved)); err != nil {
		t.Fatal(err)
	}
	if l, _ = telemetry.ReadLive(); l.Counts[telemetry.StateSwitched] != 0 || l.Counts[telemetry.StateObserved] != 1 {
		t.Fatalf("counts must reset on a new session: %+v", l)
	}
}

func TestLiveRecordIsPromptFree(t *testing.T) {
	t.Setenv("DOWNSHIFT_EVENT_LOG", t.TempDir()+"/events.jsonl")
	if err := telemetry.WriteLive(telemetry.NewSpawnRecord(telemetry.Event{Outcome: telemetry.OutcomeAllow, Harness: "codex", SessionID: strings.Repeat("ab", 32)})); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(telemetry.LivePath())
	if strings.Contains(string(data), strings.Repeat("ab", 32)) {
		t.Fatalf("the full session hash must not be written: %s", data)
	}
}

func TestActivityLine(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	ev := func(ago time.Duration, outcome string) telemetry.Event {
		return telemetry.Event{Harness: "codex", SessionID: strings.Repeat("a", 64), Outcome: outcome, FromModel: "gpt-6-sol", ToModel: "gpt-6-sol",
			Timestamp: now.Add(-ago).Format(time.RFC3339Nano)}
	}
	h := strings.Repeat("a", 64)
	cases := []struct {
		name   string
		events []telemetry.Event
		want   string
	}{
		{"nothing yet", nil, "○ downshift on · no subagent routed yet"},
		{"recent", []telemetry.Event{ev(10*time.Second, telemetry.OutcomeAllow)}, "● downshift active · last spawn 10s ago: observed, spawn unchanged on gpt-6-sol"},
		{"old", []telemetry.Event{ev(20*time.Minute, telemetry.OutcomeAllow)}, "○ downshift idle · last spawn 20m0s ago: observed, spawn unchanged on gpt-6-sol"},
	}
	for _, c := range cases {
		if got := readout.ActivityLine(c.events, h, now); got != c.want {
			t.Errorf("%s:\n got %q\nwant %q", c.name, got, c.want)
		}
	}
}

func TestPanelLabelsAndKey(t *testing.T) {
	s := strings.Repeat("a", 64)
	ev := func(o string, mod func(*telemetry.Event)) telemetry.Event {
		e := telemetry.Event{Harness: "claude-code", SessionID: s, Outcome: o, Tier: "MID", FromModel: "claude-opus-5-5", ToModel: "claude-sonnet-5-5",
			Timestamp: time.Now().UTC().Format(time.RFC3339Nano), ToolUseHash: readout.ToolUseKey("toolu_1")}
		if mod != nil {
			mod(&e)
		}
		return e
	}
	events := []telemetry.Event{
		ev(telemetry.OutcomeRewriteEmitted, nil),
		ev(telemetry.OutcomeAllow, func(e *telemetry.Event) { e.Corrections = []string{"R3"}; e.ToModel = e.FromModel }),
		ev(telemetry.OutcomeAllow, func(e *telemetry.Event) { e.ToModel = e.FromModel }),
		ev(telemetry.OutcomeBaseline, func(e *telemetry.Event) { e.ToModel = e.FromModel }),
	}
	v := readout.BuildPanel(events, s)
	want := []string{"opus → sonnet (mid)", "held: original model kept", "kept", "unrouted (older log)"}
	for i, w := range want {
		if v.Spawns[i].Label != w {
			t.Errorf("spawn %d label %q want %q", i, v.Spawns[i].Label, w)
		}
	}
	if v.Switched != 1 || v.Held != 1 || v.Observed != 1 || v.Tokens.Observed() || v.Spawns[0].ToolUse != readout.ToolUseKey("toolu_1") {
		t.Fatalf("%+v", v)
	}
	// Pinned so the mod's own hash (same recipe) can be checked against it.
	if got := readout.ToolUseKey("toolu_1"); got != telemetry.HashSessionID("toolu_1") {
		t.Fatalf("ToolUseKey must equal HashSessionID: %s vs %s", got, telemetry.HashSessionID("toolu_1"))
	}
}

func TestReadLiveRefusesOversizedFile(t *testing.T) {
	t.Setenv("DOWNSHIFT_EVENT_LOG", t.TempDir()+"/events.jsonl")
	if err := os.WriteFile(telemetry.LivePath(), []byte(`{"session":"`+strings.Repeat("a", 64<<10)+`"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if l, err := telemetry.ReadLive(); err == nil || l.Session != "" {
		t.Fatalf("oversized live.json must be refused: %+v %v", l, err)
	}
}
