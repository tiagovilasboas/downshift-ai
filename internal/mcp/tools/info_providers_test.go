// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package tools

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// The providers description is one of the three channels that teach the
// orchestrator to consult the live model list, so it must say to call providers
// before every spawn, not once per session. This golden text is the sensor for
// that guidance (docs/evidence/mcp-claude-code-2026-10-31.md, M6).
func TestProvidersDescriptionAsksToCallBeforeEverySpawn(t *testing.T) {
	for _, want := range []string{"before every subagent spawn", "not once per session", "Agent or Task"} {
		if !strings.Contains(providersDesc, want) {
			t.Errorf("providers description lacks %q: %s", want, providersDesc)
		}
	}
}

func TestProvidersPayloadSlim(t *testing.T) {
	d := fullDeps()
	d.Models = fakeModels{list: ModelList{
		Harness:        "claude-code",
		Fallback:       false,
		CacheWrittenAt: time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC),
		Models: []Model{
			{ID: "m1", Tier: "small", Efforts: []string{"low"}, Source: "catalog", Untiered: false},
			{ID: "m2", Tier: "", Source: "discovered", Untiered: true},
		},
	}}
	d.History = fakeHistory{sum: SessionSummary{
		Known:       false,
		Note:        "no session id",
		ByTier:      map[string]int{},
		ByAgentType: map[string]int{},
	}}
	env, err := call(t, d, "providers", `{"harness":"claude-code"}`)
	if err != nil {
		t.Fatal(err)
	}
	raw := string(env["data"])
	for _, omitted := range []string{
		`"cache_written_at"`,
		`"untiered":false`,
		`"fallback":false`,
		`"spawns"`,
		`"by_tier"`,
		`"by_agent_type"`,
		`"above_agent_default"`,
	} {
		if strings.Contains(raw, omitted) {
			t.Errorf("providers payload should omit %s, got: %s", omitted, raw)
		}
	}
	for _, kept := range []string{
		`"harness":"claude-code"`,
		`"id":"m1"`,
		`"tier":"small"`,
		`"efforts":["low"]`,
		`"source":"catalog"`,
		`"id":"m2"`,
		`"untiered":true`,
		`"agent_default_tiers":[`,
	} {
		if !strings.Contains(raw, kept) {
			t.Errorf("providers payload missing %s, got: %s", kept, raw)
		}
	}
	var got providersData
	if err := json.Unmarshal(env["data"], &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Models) != 2 || got.Models[0].ID != "m1" || !got.Models[1].Untiered || got.Session == nil || got.Session.Note != "no session id" {
		t.Fatalf("unmarshaled providersData = %+v", got)
	}
}
