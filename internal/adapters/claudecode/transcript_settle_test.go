// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package claudecode

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// settleRow is one synthetic assistant transcript row.
func settleRow(t *testing.T, id string, in, out, cacheWrite, cacheRead int) string {
	t.Helper()
	b, err := json.Marshal(map[string]any{
		"type": "assistant",
		"message": map[string]any{
			"id": id, "model": "claude-haiku-test",
			"usage": map[string]any{
				"input_tokens": in, "output_tokens": out,
				"cache_creation_input_tokens": cacheWrite, "cache_read_input_tokens": cacheRead,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(b) + "\n"
}

const userRow = `{"type":"user"}` + "\n"

func appendFile(t *testing.T, path, s string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Error(err)
		return
	}
	defer f.Close()
	if _, err := f.WriteString(s); err != nil {
		t.Error(err)
	}
}

func TestReadSettledSubagentUsage(t *testing.T) {
	window := settleWindow{poll: 5 * time.Millisecond, quiet: 150 * time.Millisecond, deadline: 2 * time.Second}
	half := func(t *testing.T) (string, string) {
		full := settleRow(t, "m2", 2, 60, 10, 100)
		return full[:len(full)/2], full[len(full)/2:]
	}
	tests := []struct {
		name string
		// initial is written before the read; tail is appended 40 ms after it starts.
		initial, tail      func(t *testing.T) string
		wantOK             bool
		wantIn, wantOut    int64
		wantWrite, wantHit int64
	}{
		{
			name: "complete transcript is read as is",
			initial: func(t *testing.T) string {
				return settleRow(t, "m1", 2, 30, 100, 0) + settleRow(t, "m2", 2, 50, 20, 100)
			},
			wantOK: true, wantIn: 4, wantOut: 80, wantWrite: 120, wantHit: 220,
		},
		{
			name:    "last message flushed after the stop is picked up",
			initial: func(t *testing.T) string { return settleRow(t, "m1", 2, 30, 100, 0) },
			tail:    func(t *testing.T) string { return settleRow(t, "m2", 2, 900, 40, 100) },
			wantOK:  true, wantIn: 4, wantOut: 930, wantWrite: 140, wantHit: 240,
		},
		{
			name: "streamed rows of one message are not counted twice across re-reads",
			initial: func(t *testing.T) string {
				return settleRow(t, "m1", 2, 30, 100, 0) + settleRow(t, "m2", 2, 3, 40, 100)
			},
			tail:   func(t *testing.T) string { return settleRow(t, "m2", 2, 700, 40, 100) },
			wantOK: true, wantIn: 4, wantOut: 730, wantWrite: 140, wantHit: 240,
		},
		{
			name: "half-written last line is skipped, then counted once complete",
			initial: func(t *testing.T) string {
				head, _ := half(t)
				return settleRow(t, "m1", 2, 30, 100, 0) + "not json\n" + head
			},
			tail: func(t *testing.T) string {
				_, rest := half(t)
				return rest
			},
			wantOK: true, wantIn: 4, wantOut: 90, wantWrite: 110, wantHit: 210,
		},
		{
			name:    "no assistant usage reads as nothing",
			initial: func(t *testing.T) string { return userRow + "{broken\n" },
			wantOK:  false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "agent-a1.jsonl")
			if err := os.WriteFile(path, []byte(tc.initial(t)), 0o600); err != nil {
				t.Fatal(err)
			}
			done := make(chan struct{})
			if tc.tail != nil {
				tail := tc.tail(t)
				go func() {
					defer close(done)
					time.Sleep(40 * time.Millisecond)
					appendFile(t, path, tail)
				}()
			} else {
				close(done)
			}
			got, ok := readSettledSubagentUsage(path, window)
			<-done
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if !ok {
				return
			}
			u := got.Usage
			if u.InputTokens != tc.wantIn || u.OutputTokens != tc.wantOut || u.CacheWriteTokens != tc.wantWrite || u.CachedTokens != tc.wantHit {
				t.Fatalf("usage = %+v, want in=%d out=%d write=%d cached=%d", u, tc.wantIn, tc.wantOut, tc.wantWrite, tc.wantHit)
			}
		})
	}
}

// A transcript that cannot be read is not waited on: the hook returns at once.
func TestReadSettledSubagentUsage_FailsOpenWithoutWaiting(t *testing.T) {
	window := settleWindow{poll: 5 * time.Millisecond, quiet: time.Second, deadline: 5 * time.Second}
	dir := t.TempDir()
	oversized := filepath.Join(dir, "agent-big.jsonl")
	f, err := os.Create(oversized)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(maxTranscriptBytes + 1); err != nil {
		t.Fatal(err)
	}
	f.Close()
	notJSONL := filepath.Join(dir, "agent-a1.txt")
	if err := os.WriteFile(notJSONL, []byte(settleRow(t, "m1", 2, 3, 4, 5)), 0o600); err != nil {
		t.Fatal(err)
	}
	asDir := filepath.Join(dir, "agent-dir.jsonl")
	if err := os.Mkdir(asDir, 0o700); err != nil {
		t.Fatal(err)
	}
	for name, path := range map[string]string{
		"oversized": oversized,
		"not jsonl": notJSONL,
		"missing":   filepath.Join(dir, "agent-none.jsonl"),
		"directory": asDir,
	} {
		t.Run(name, func(t *testing.T) {
			start := time.Now()
			if _, ok := readSettledSubagentUsage(path, window); ok {
				t.Fatal("ok = true, want false")
			}
			if waited := time.Since(start); waited > 500*time.Millisecond {
				t.Fatalf("waited %v on an unreadable transcript", waited)
			}
		})
	}
}

// A transcript that keeps growing past the deadline still returns: the wait is
// bounded and the last good read is recorded.
func TestReadSettledSubagentUsage_StopsAtDeadline(t *testing.T) {
	window := settleWindow{poll: 5 * time.Millisecond, quiet: time.Second, deadline: 150 * time.Millisecond}
	path := filepath.Join(t.TempDir(), "agent-a1.jsonl")
	if err := os.WriteFile(path, []byte(settleRow(t, "m1", 2, 30, 100, 0)), 0o600); err != nil {
		t.Fatal(err)
	}
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-stop:
				return
			case <-time.After(10 * time.Millisecond):
				appendFile(t, path, userRow)
			}
		}
	}()
	start := time.Now()
	got, ok := readSettledSubagentUsage(path, window)
	waited := time.Since(start)
	close(stop)
	<-done
	if !ok || got.Usage.OutputTokens != 30 {
		t.Fatalf("got %+v ok=%v, want the first message's usage", got.Usage, ok)
	}
	if waited > time.Second {
		t.Fatalf("waited %v, want the 150ms deadline to hold", waited)
	}
}
