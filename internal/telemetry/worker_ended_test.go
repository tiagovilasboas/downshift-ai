// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry_test

import (
	"encoding/json"
	"github.com/tiagovilasboas/downshift-ai/internal/telemetry"
	"github.com/tiagovilasboas/downshift-ai/internal/telemetry/readout"
	"strings"
	"testing"
)

func TestWorkerEndedCarriesTokensAndOutcomeOnly(t *testing.T) {
	ev := telemetry.WorkerEnded("codex", "done", &telemetry.TokenUsage{InputTokens: 10, OutputTokens: 5, CachedTokens: 7, CacheWriteTokens: 2})
	line, err := json.Marshal(ev)
	if err != nil {
		t.Fatal(err)
	}
	s := string(line)
	for _, want := range []string{`"outcome":"worker_ended"`, `"worker_outcome":"done"`, `"harness":"codex"`, `"input_tokens":10`, `"cached_tokens":7`, `"cache_write_tokens":2`} {
		if !strings.Contains(s, want) {
			t.Errorf("record lacks %s: %s", want, s)
		}
	}
	if ev.TokenUsageOrZero().InputTokens != 10 {
		t.Errorf("usage = %+v", ev.TokenUsageOrZero())
	}
	if none := telemetry.WorkerEnded("claude-code", "timeout", nil); none.HasTokenUsage() || none.InputTokens != nil {
		t.Errorf("a worker without usage must carry no token fields: %+v", none)
	}
}

func TestAggregateSkipsWorkerEnds(t *testing.T) {
	decision := telemetry.Event{CorrelationID: "a1", Harness: "claude-code", Outcome: telemetry.OutcomeAllow, Complexity: "SIMPLE"}
	worker := telemetry.WorkerEnded("claude-code", "done", &telemetry.TokenUsage{InputTokens: 1000, OutputTokens: 100})
	s := readout.Aggregate([]telemetry.Event{decision, worker})
	if s.Total != 1 || s.UsageRecords != 0 {
		t.Fatalf("worker end moved decision counts: total=%d usage=%d", s.Total, s.UsageRecords)
	}
	if !telemetry.IsObservationOutcome(telemetry.OutcomeWorkerEnded) {
		t.Fatal("a worker end must never be linked as a decision")
	}
}
