// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package claudecode

import (
	"os"
	"path/filepath"
	"time"
)

// settleWindow bounds the wait for a subagent transcript to finish flushing.
//
// SubagentStop can fire before Claude Code has appended the child's last
// assistant message(s). Measured 2026-11-16 against 279 recorded children: 211
// usage records were short by exactly their last one or two messages (input
// short by 2 or 4 tokens, plus that message's cache and output), and the
// transcript file changed 17 to 90 ms after the hook had recorded. Re-reading
// the file while it still grows picks that tail up.
type settleWindow struct {
	poll     time.Duration // how often the file size is checked
	quiet    time.Duration // how long the size must stay unchanged to count as settled
	deadline time.Duration // the most the hook waits in total, settled or not
}

// transcriptSettle is the window the SubagentStop hook uses. The whole wait is
// far below the harness's hook timeout; past the deadline the last good read is
// what gets recorded (fail open).
var transcriptSettle = settleWindow{
	poll:     25 * time.Millisecond,
	quiet:    250 * time.Millisecond,
	deadline: time.Second,
}

// readSettledSubagentUsage reads a subagent transcript, then keeps re-reading it
// while it grows, until its size has stayed unchanged for the quiet period or
// the deadline passes. The last successful read wins; a transcript that cannot
// be read (not .jsonl, not a regular file, oversized) is not waited on.
// Dedupe by message id is readSubagentUsage's, so a re-read never adds a
// message twice.
func readSettledSubagentUsage(path string, w settleWindow) (subagentUsage, bool) {
	size, ok := transcriptSize(path)
	if !ok {
		return subagentUsage{}, false
	}
	got, gotOK := readSubagentUsage(path)
	start := time.Now()
	lastChange := start
	for time.Since(start) < w.deadline {
		time.Sleep(w.poll)
		now, ok := transcriptSize(path)
		if !ok {
			break // the file went away or past the cap: keep what was read
		}
		if now != size {
			size, lastChange = now, time.Now()
			if g, ok := readSubagentUsage(path); ok {
				got, gotOK = g, true
			}
			continue
		}
		if time.Since(lastChange) >= w.quiet {
			break
		}
	}
	return got, gotOK
}

// transcriptSize is the size of a readable transcript within the read cap; ok
// is false for anything readSubagentUsage would refuse.
func transcriptSize(path string) (int64, bool) {
	if filepath.Ext(path) != ".jsonl" {
		return 0, false
	}
	fi, err := os.Lstat(path)
	if err != nil || !fi.Mode().IsRegular() || fi.Size() > maxTranscriptBytes {
		return 0, false
	}
	return fi.Size(), true
}
