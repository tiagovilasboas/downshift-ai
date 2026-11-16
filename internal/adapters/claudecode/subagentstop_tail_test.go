// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package claudecode_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/adapters/claudecode"
)

// tailRow is one synthetic assistant row of a subagent transcript.
func tailRow(t *testing.T, id, model string, out int) []byte {
	t.Helper()
	b, err := json.Marshal(map[string]any{
		"type": "assistant",
		"message": map[string]any{
			"id": id, "model": model,
			"usage": map[string]any{
				"input_tokens": 2, "output_tokens": out,
				"cache_creation_input_tokens": 100, "cache_read_input_tokens": 50,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return append(b, '\n')
}

// syncCompletion is the PostToolUse a finished foreground child sends: its
// tool_response.usage is the child's LAST API call, not its total.
func syncCompletion(model string) []byte {
	b, _ := json.Marshal(map[string]any{
		"hook_event_name": "PostToolUse", "session_id": "sess-stop", "tool_name": "Agent",
		"tool_use_id": "toolu_sync1",
		"tool_response": map[string]any{
			"status": "completed", "agentId": stopAgent, "resolvedModel": model,
			"totalTokens": 2 + 100 + 50 + 900,
			"usage": map[string]any{
				"input_tokens": 2, "output_tokens": 900,
				"cache_creation_input_tokens": 100, "cache_read_input_tokens": 50,
			},
		},
	})
	return b
}

// SubagentStop can fire before the child's last message reaches the file; the
// hook waits for the tail and prices the whole child.
func TestSubagentStop_WaitsForTheUnflushedTail(t *testing.T) {
	log, small, frontier, res := stopSetup(t)
	seedLaunch(t, log, small, frontier, res)
	defer claudecode.SetTranscriptSettle(5*time.Millisecond, 200*time.Millisecond, 2*time.Second)()

	path := filepath.Join(t.TempDir(), "agent-"+stopAgent+".jsonl")
	if err := os.WriteFile(path, tailRow(t, "msg_1", small, 40), 0o600); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		time.Sleep(50 * time.Millisecond)
		f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
		if err != nil {
			t.Error(err)
			return
		}
		defer f.Close()
		_, _ = f.Write(tailRow(t, "msg_2", small, 900))
	}()
	claudecode.HandleSubagentStop(stopPayload(path), "test", res)
	<-done

	got := usageEvents(t)
	if len(got) != 1 {
		t.Fatalf("usage events = %d, want 1", len(got))
	}
	if in, out := tokens(got[0].InputTokens), tokens(got[0].OutputTokens); in != 4 || out != 940 {
		t.Fatalf("usage in=%d out=%d, want 4 and 940 (both messages)", in, out)
	}
}

// tokens reads an optional token count; -1 when absent.
func tokens(p *int64) int64 {
	if p == nil {
		return -1
	}
	return *p
}

// A foreground child reports twice: SubagentStop (transcript total) and the
// Agent tool's PostToolUse (last API call only). Whatever the order, the child
// is priced once, from its transcript.
func TestChildEnd_SyncChildPricedOnceFromTranscript(t *testing.T) {
	for _, order := range []string{"stop then post", "post then stop", "post only"} {
		t.Run(order, func(t *testing.T) {
			log, small, frontier, res := stopSetup(t)
			seedLaunch(t, log, small, frontier, res)
			path := filepath.Join(t.TempDir(), "agent-"+stopAgent+".jsonl")
			rows := append(tailRow(t, "msg_1", small, 40), tailRow(t, "msg_2", small, 900)...)
			if err := os.WriteFile(path, rows, 0o600); err != nil {
				t.Fatal(err)
			}
			stop := func() { claudecode.HandleSubagentStop(stopPayload(path), "test", res) }
			post := func() { claudecode.HandlePostToolUse(syncCompletion(small), "test", res) }
			switch order {
			case "stop then post":
				stop()
				post()
			case "post then stop":
				post()
				stop()
			default:
				post()
			}

			got := usageEvents(t)
			if order == "post only" {
				if len(got) != 0 {
					t.Fatalf("usage events = %d, want 0: tool_response.usage is the last call, not the child's total", len(got))
				}
				return
			}
			if len(got) != 1 {
				t.Fatalf("usage events = %d, want 1 (no double count)", len(got))
			}
			if in, out := tokens(got[0].InputTokens), tokens(got[0].OutputTokens); in != 4 || out != 940 {
				t.Fatalf("usage in=%d out=%d, want the transcript total 4 / 940", in, out)
			}
		})
	}
}
