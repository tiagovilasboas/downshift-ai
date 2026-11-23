// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry_test

import (
	"encoding/json"
	"github.com/tiagovilasboas/downshift-ai/internal/telemetry"
	"github.com/tiagovilasboas/downshift-ai/internal/telemetry/readout"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func liveDecision(sess, outcome, from, to, tier string, codes ...string) telemetry.Event {
	return telemetry.Event{
		CorrelationID: from + to + outcome + strings.Join(codes, ","), Harness: "claude-code", SessionID: sess,
		Outcome: outcome, Verdict: "DOWNSHIFT", FromModel: from, ToModel: to, Tier: tier, ReasonCodes: codes,
	}
}

func liveUsage(sess, linked string, actual, baseline float64) telemetry.Event {
	return telemetry.Event{Outcome: telemetry.OutcomeUsage, Harness: "claude-code", SessionID: sess, LinkedDecision: linked,
		InputTokens: i64(600), OutputTokens: i64(400), CachedTokens: i64(9000)}
}

func i64(n int64) *int64 { return &n }

func TestShortModel(t *testing.T) {
	for in, want := range map[string]string{
		"claude-haiku-4-5": "haiku", "claude-haiku-4-5-20251001": "haiku", "claude-3-5-haiku-20241022": "haiku",
		"claude-opus-4-7": "opus", "gpt-5.6-sol": "gpt-5.6-sol", "claude-4": "claude-4", "": "",
	} {
		if got := telemetry.ShortModel(in); got != want {
			t.Errorf("ShortModel(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSessionSummaryLine(t *testing.T) {
	const s = "sess-a"
	haiku, opus := "claude-haiku-4-5", "claude-opus-4-7"
	rewrite := func() telemetry.Event { return liveDecision(s, telemetry.OutcomeRewriteEmitted, opus, haiku, "small") }
	up := func() telemetry.Event {
		return liveDecision(s, telemetry.OutcomeRewriteEmitted, haiku, opus, "frontier")
	} // an upshift: tier goes up
	cases := []struct {
		name   string
		events []telemetry.Event
		want   string
	}{
		{"empty log", nil, ""},
		{"only another session", []telemetry.Event{liveDecision("sess-b", telemetry.OutcomeRewriteEmitted, opus, haiku, "small")}, ""},
		{"only quota-like noise", []telemetry.Event{{Outcome: "error", SessionID: s}, {Outcome: telemetry.OutcomeResolved, SessionID: s}}, ""},
		{"one spawn, no cost", []telemetry.Event{rewrite()}, "◆ downshift · 1 subagent · 1 switched · 0 held · 1 on haiku · 0 on doubt"},
		{"below", []telemetry.Event{rewrite(), liveUsage(s, rewrite().CorrelationID, 0.10, 0.50)},
			"◆ downshift · 1 subagent · 1 switched · 0 held · 1 on haiku · 1.0k of 1.0k billable below the requested model (1 spawn, no session model) · 0 on doubt"},
		{"above the baseline is not below", []telemetry.Event{up(), liveUsage(s, up().CorrelationID, 0.50, 0.10)},
			"◆ downshift · 1 subagent · 1 switched · 0 held · 1 on opus · 0 of 1.0k billable below the requested model (1 spawn, no session model) · 0 on doubt"},
		{"paused child counts in the total", []telemetry.Event{rewrite(), liveUsage(s, rewrite().CorrelationID, 0.10, 0.50),
			liveDecision(s, telemetry.OutcomeBaseline, opus, opus, "frontier", telemetry.ReasonPaused), liveUsage(s, opus+opus+telemetry.OutcomeBaseline+telemetry.ReasonPaused, 0.50, 0.50)},
			// The statusline counts routed spawns; the paused child's tokens
			// still enter the token total, so the share is not inflated.
			"◆ downshift · 1 subagent · 1 switched · 0 held · 1 on haiku · 1.0k of 2.0k billable below the requested model (2 spawns, no session model) · 0 on doubt"},
		{"allow counts the requested model", []telemetry.Event{liveDecision(s, telemetry.OutcomeAllow, opus, opus, "frontier"), rewrite(), rewrite()},
			"◆ downshift · 3 subagents · 2 switched · 0 held · 2 on haiku · 0 on doubt"},
		{"doubt", []telemetry.Event{
			liveDecision(s, telemetry.OutcomeRewriteEmitted, "claude-sonnet-5-5", opus, "frontier", telemetry.ReasonClassifierUnconfident),    // written on doubt
			liveDecision(s, telemetry.OutcomeAllow, opus, opus, "frontier", telemetry.ReasonClassifierUnconfident, telemetry.ReasonRiskFloor), // lifted: not doubt
		}, "◆ downshift · 2 subagents · 1 switched · 0 held · 2 on opus · 1 on doubt"},
		{"doubt that was not written is not counted", []telemetry.Event{
			liveDecision(s, telemetry.OutcomeAllow, opus, opus, "frontier", telemetry.ReasonClassifierUnconfident),
		}, "◆ downshift · 1 subagent · 0 switched · 0 held · 1 on opus · 0 on doubt"},
		{"unknown models are not a top model", []telemetry.Event{liveDecision(s, telemetry.OutcomeAllow, "unknown", "unknown", "mid")},
			"◆ downshift · 1 subagent · 0 switched · 0 held · 0 on doubt"},
		{"usage without tokens adds no token segment", []telemetry.Event{rewrite(), {Outcome: telemetry.OutcomeUsage, SessionID: s}},
			"◆ downshift · 1 subagent · 1 switched · 0 held · 1 on haiku · 0 on doubt"},
	}
	for _, c := range cases {
		if got := readout.SessionSummary(c.events, s).Line(); got != c.want {
			t.Errorf("%s:\n got %q\nwant %q", c.name, got, c.want)
		}
	}
	if got := readout.SessionSummary([]telemetry.Event{rewrite()}, "").Line(); got != "" {
		t.Errorf("an empty session hash must match nothing, got %q", got)
	}
}

func TestReadEventsTailIsBoundedAndDropsTheCutLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	f, _ := os.Create(path)
	pad := strings.Repeat("x", 200)
	const n = 20000 // about 6 MB, far over the window
	for i := 0; i < n; i++ {
		b, _ := json.Marshal(telemetry.Event{CorrelationID: pad, Outcome: telemetry.OutcomeAllow, SessionID: "s", Timestamp: "t"})
		f.Write(append(b, '\n'))
	}
	f.Close()
	events, err := telemetry.ReadEventsTailFrom(path, 100<<10)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) == 0 || len(events) > 400 {
		t.Fatalf("tail read %d events, want a few hundred at most", len(events))
	}
	for _, ev := range events {
		if ev.SessionID != "s" {
			t.Fatalf("a cut line was parsed as an event: %+v", ev)
		}
	}
	if all, _ := telemetry.ReadEventsTailFrom(path, 0); len(all) != n {
		t.Fatalf("no bound should read all %d, got %d", n, len(all))
	}
	if ev, err := telemetry.ReadEventsTailFrom(filepath.Join(t.TempDir(), "none"), 1<<20); err != nil || ev != nil {
		t.Fatalf("missing log = %v, %v; want nil, nil", ev, err)
	}
}

func TestReadEventsTailUnlinkedSkipsLateUsageLink(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	t.Setenv("DOWNSHIFT_EVENT_LOG", path)
	rows := []telemetry.Event{
		{CorrelationID: "d1", Timestamp: "2026-10-20T10:00:00Z", Harness: "claude-code", SessionID: "s", Outcome: telemetry.OutcomeAllow,
			Verdict: "DOWNSHIFT", FromModel: "claude-opus-4-8", ToModel: "claude-haiku-4-5", Tier: "small"},
		{CorrelationID: "u1", Timestamp: "2026-10-20T10:00:05Z", Harness: "claude-code", SessionID: "s", Outcome: telemetry.OutcomeUsage,
			AgentHash: "a1", FromModel: "unknown", ToModel: "claude-haiku-4-5", Verdict: "UNKNOWN", InputTokens: i64(100)},
		{CorrelationID: "r1", Timestamp: "2026-10-20T10:00:06Z", Harness: "claude-code", SessionID: "s", Outcome: telemetry.OutcomeResolved,
			AgentHash: "a1", LinkedDecision: "d1", FromModel: "claude-opus-4-8", ToModel: "claude-haiku-4-5"},
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, ev := range rows {
		b, _ := json.Marshal(ev)
		f.Write(append(b, '\n'))
	}
	f.Close()

	unlinked, err := telemetry.ReadEventsTailUnlinked(telemetry.DefaultTailBytes)
	if err != nil {
		t.Fatal(err)
	}
	if len(unlinked) != 3 {
		t.Fatalf("got %d events, want 3", len(unlinked))
	}
	if unlinked[1].LinkedDecision != "" || unlinked[1].FromModel != "unknown" {
		t.Errorf("ReadEventsTailUnlinked linked late usage: %+v", unlinked[1])
	}

	linked, err := telemetry.ReadEventsTail(telemetry.DefaultTailBytes)
	if err != nil {
		t.Fatal(err)
	}
	if linked[1].LinkedDecision != "d1" || linked[1].FromModel != "claude-opus-4-8" {
		t.Errorf("ReadEventsTail did not link late usage: %+v", linked[1])
	}
}

func TestChildEndLine(t *testing.T) {
	cases := []struct {
		name string
		ev   telemetry.Event
		want string
	}{
		{"full", telemetry.Event{FromModel: "claude-opus-4-7", ToModel: "claude-haiku-4-5-20251001", InputTokens: i64(12000), OutputTokens: i64(2000)}, "↳ downshift: finished on haiku · 14k tokens (work 14k, cache 0) · lower tier than opus"},
		{"same model says nothing about saving", telemetry.Event{FromModel: "claude-haiku-4-5", ToModel: "claude-haiku-4-5", InputTokens: i64(900)}, "↳ downshift: finished on haiku · 900 tokens (work 900, cache 0)"},
		{"pricier", telemetry.Event{FromModel: "claude-sonnet-5-5", ToModel: "claude-opus-5-5", InputTokens: i64(100), CachedTokens: i64(5000)}, "↳ downshift: finished on opus · 5.1k tokens (work 100, cache 5.0k) · higher tier than sonnet"},
		{"unknown requested model", telemetry.Event{FromModel: "unknown", ToModel: "claude-haiku-4-5"},
			"↳ downshift: finished on haiku"},
		{"no baseline: the tier comparison needs no dollar", telemetry.Event{FromModel: "claude-opus-4-7", ToModel: "claude-haiku-4-5", InputTokens: i64(1500)},
			"↳ downshift: finished on haiku · 1.5k tokens (work 1.5k, cache 0) · lower tier than opus"},
		{"model only", telemetry.Event{ToModel: "claude-sonnet-4-5"}, "↳ downshift: finished on sonnet"},
		{"tokens only", telemetry.Event{ToModel: "unknown", OutputTokens: i64(2500000)}, "↳ downshift: 2.5M tokens (work 2.5M, cache 0)"},
		{"nothing known", telemetry.Event{ToModel: "unknown"}, ""},
	}
	for _, c := range cases {
		c.ev.Harness = "claude-code" // a real child event always names its harness
		if got := telemetry.ChildEndLine(c.ev); got != c.want {
			t.Errorf("%s:\n got %q\nwant %q", c.name, got, c.want)
		}
		if strings.Contains(telemetry.ChildEndLine(c.ev), "$") {
			t.Errorf("%s: the live child-end line carries a dollar figure", c.name)
		}
	}
}

func TestQuietAndChildLineEnabled(t *testing.T) {
	t.Setenv("DOWNSHIFT_QUIET", "")
	if telemetry.Quiet() || !telemetry.ChildLineEnabled(true) || telemetry.ChildLineEnabled(false) {
		t.Fatal("default: shown only when the harness can show it")
	}
	t.Setenv("DOWNSHIFT_QUIET", "1")
	if !telemetry.Quiet() || telemetry.ChildLineEnabled(true) {
		t.Fatal("DOWNSHIFT_QUIET=1 must silence the line")
	}
}

func TestSessionRoutingLineQuotaStale(t *testing.T) {
	if got := (readout.SessionRouting{QuotaStale: true}).Line(); got != "◆ downshift · quota stale" {
		t.Errorf("no spawns: %q", got)
	}
	if got := (readout.SessionRouting{Spawns: 1, QuotaStale: true}).Line(); !strings.HasSuffix(got, " · 0 on doubt · quota stale") {
		t.Errorf("with spawns: %q", got)
	}
	if got := (readout.SessionRouting{}).Line(); got != "" {
		t.Errorf("empty: %q", got)
	}
}
