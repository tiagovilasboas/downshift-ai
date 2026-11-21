// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package claudecode_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/adapters/claudecode"
	"github.com/tiagovilasboas/downshift-ai/internal/telemetry"
)

func jsonLine(t *testing.T, v map[string]any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b) + "\n"
}

// How a subagent transcript ends decides the status the stop records. All rows
// are synthetic: structure only, no task text.
func TestParseChildEnd_SubagentStopTranscriptEnd(t *testing.T) {
	defer claudecode.SetTranscriptSettle(time.Millisecond, 2*time.Millisecond, 20*time.Millisecond)()
	userText := func(s string) map[string]any {
		return map[string]any{"type": "user", "message": map[string]any{"content": []any{map[string]any{"type": "text", "text": s}}}}
	}
	apiErr := map[string]any{"type": "assistant", "isApiErrorMessage": true, "error": "rate_limit",
		"message": map[string]any{"id": "e1", "content": []any{map[string]any{"type": "text", "text": "x"}}}}
	okTurn := map[string]any{"type": "assistant",
		"message": map[string]any{"id": "m1", "model": "claude-sonnet-5-5", "usage": map[string]any{"input_tokens": 5, "output_tokens": 7}}}
	toolResult := map[string]any{"type": "user", "message": map[string]any{"content": []any{map[string]any{"type": "tool_result", "tool_use_id": "t1"}}}}
	attach := map[string]any{"type": "attachment", "attachment": map[string]any{"type": "hook_success"}}

	cases := []struct {
		name   string
		rows   []map[string]any
		want   telemetry.ChildStatus
		wantOK bool
	}{
		{"api error, no usage", []map[string]any{userText("go"), apiErr}, telemetry.ChildError, true},
		{"api error then hook attachment", []map[string]any{userText("go"), apiErr, attach}, telemetry.ChildError, true},
		{"api error after real usage", []map[string]any{okTurn, apiErr}, telemetry.ChildError, true},
		{"user interrupt, no usage", []map[string]any{userText("go"), userText("[Request interrupted by user]")}, telemetry.ChildInterrupted, true},
		{"interrupt for tool use", []map[string]any{okTurn, userText("[Request interrupted by user for tool use]")}, telemetry.ChildInterrupted, true},
		{"normal end with usage", []map[string]any{okTurn}, telemetry.ChildUnknown, true},
		{"ends on a tool result", []map[string]any{okTurn, toolResult}, telemetry.ChildUnknown, true},
		{"api error then recovery", []map[string]any{apiErr, okTurn}, telemetry.ChildUnknown, true},
		{"no usage, no end marker", []map[string]any{userText("go")}, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var sb strings.Builder
			for _, r := range tc.rows {
				sb.WriteString(jsonLine(t, r))
			}
			path := filepath.Join(t.TempDir(), "agent-"+stopAgent+".jsonl")
			if err := os.WriteFile(path, []byte(sb.String()), 0o600); err != nil {
				t.Fatal(err)
			}
			raw, _ := json.Marshal(map[string]any{
				"hook_event_name": "SubagentStop", "session_id": "s", "agent_id": stopAgent, "agent_transcript_path": path,
			})
			ce, ok := claudecode.ParseChildEnd(raw)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if ok && ce.Status != tc.want {
				t.Fatalf("status = %q, want %q", ce.Status, tc.want)
			}
		})
	}
}
