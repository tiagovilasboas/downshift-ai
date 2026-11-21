// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package claudecode

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/tiagovilasboas/downshift-ai/internal/telemetry"
)

// endTailBytes bounds the tail of a subagent transcript read for its end. The
// last non-attachment row is small (an error line or an interrupt marker);
// a larger one is a tool result, which is not an end this reads.
const endTailBytes = 256 << 10

// interruptMarker is the text Claude Code writes as the last user row when the
// user cancels a running subagent ("[Request interrupted by user]" and the
// "... for tool use" variant).
const interruptMarker = "[Request interrupted"

// transcriptEnd classifies how a subagent transcript ends. A child that died
// on an API error (rate limit, server error, auth) or was cancelled writes no
// usable assistant turn, so the usage read finds nothing and the end would be
// lost. ChildUnknown when the tail is neither, or unreadable. Only booleans
// leave this function: no error text or task text is kept.
func transcriptEnd(path string) telemetry.ChildStatus {
	if filepath.Ext(path) != ".jsonl" {
		return telemetry.ChildUnknown
	}
	// Lstat first: opening a FIFO or device blocks until a writer appears.
	if fi, err := os.Lstat(path); err != nil || !fi.Mode().IsRegular() {
		return telemetry.ChildUnknown
	}
	f, err := os.Open(path)
	if err != nil {
		return telemetry.ChildUnknown
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil || !fi.Mode().IsRegular() || fi.Size() > maxTranscriptBytes {
		return telemetry.ChildUnknown
	}
	off := max(fi.Size()-endTailBytes, 0)
	buf, err := io.ReadAll(io.NewSectionReader(f, off, endTailBytes))
	if err != nil {
		return telemetry.ChildUnknown
	}
	apiErr, interrupted := lastRowEnd(buf, off > 0)
	return telemetry.ClassifyTranscriptEnd(apiErr, interrupted)
}

// lastRowEnd inspects the last row that is not an attachment (hook output
// Claude Code appends after the real end). partial drops the first line of the
// buffer, which a mid-file read cut.
func lastRowEnd(buf []byte, partial bool) (apiErr, interrupted bool) {
	lines := bytes.Split(bytes.TrimRight(buf, "\n"), []byte("\n"))
	if partial && len(lines) > 0 {
		lines = lines[1:]
	}
	for i := len(lines) - 1; i >= 0; i-- {
		var row struct {
			Type    string `json:"type"`
			APIErr  bool   `json:"isApiErrorMessage"`
			Message struct {
				Content json.RawMessage `json:"content"`
			} `json:"message"`
		}
		if json.Unmarshal(lines[i], &row) != nil {
			return false, false
		}
		if row.Type == "attachment" {
			continue
		}
		if row.Type == "assistant" {
			return row.APIErr, false
		}
		return false, row.Type == "user" && startsWithInterrupt(row.Message.Content)
	}
	return false, false
}

func startsWithInterrupt(content json.RawMessage) bool {
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(content, &blocks) != nil || len(blocks) == 0 {
		return false
	}
	return blocks[0].Type == "text" && strings.HasPrefix(blocks[0].Text, interruptMarker)
}
