// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package missedsavings

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/lazyre"
)

// event is what one transcript line says about shell commands: calls made
// (command text, in memory only) and results returned (size only).
type event struct {
	at      time.Time
	calls   []call
	results []result
}

type call struct {
	id         string
	command    string
	nonBash    bool
	background bool
}

type result struct {
	id        string
	bytes     int64
	compacted bool
}

// parser turns one transcript line into an event; ok is false for any line
// that is malformed or irrelevant.
type parser func(line []byte) (event, bool)

// parseTime reads an RFC 3339 stamp; the zero time means unknown.
func parseTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

var compactMarkerRe = lazyre.New(`^\[downshift compact: (?:compacted )?\d+ to \d+ bytes\b`)

// hasCompactMarker reports whether text ends with a downshift compaction
// marker line (ignoring trailing wrapper tail notes such as exit/stderr notes).
// Markers quoted mid-output (e.g. grep or diff hits) are ignored.
func hasCompactMarker(text string) bool {
	s := strings.TrimRight(text, "\r\n")
	for s != "" {
		i := strings.LastIndexByte(s, '\n')
		line := strings.TrimRight(s[i+1:], "\r")
		if compactMarkerRe.MatchString(line) {
			return true
		}
		if !strings.HasPrefix(line, "[downshift compact: ") {
			return false
		}
		if i < 0 {
			return false
		}
		s = strings.TrimRight(s[:i], "\r\n")
	}
	return false
}

// textStats sizes a tool result (a JSON string, or an array of blocks with a
// "text" field) and reports whether it carries a trailing compaction marker.
func textStats(raw json.RawMessage) (int64, bool) {
	if len(raw) == 0 {
		return 0, false
	}
	switch raw[0] {
	case '"':
		var s string
		if json.Unmarshal(raw, &s) == nil {
			return int64(len(s)), hasCompactMarker(s)
		}
	case '[':
		var parts []struct {
			Text string `json:"text"`
		}
		if json.Unmarshal(raw, &parts) == nil {
			var n int64
			compacted := false
			for _, p := range parts {
				n += int64(len(p.Text))
				if hasCompactMarker(p.Text) {
					compacted = true
				}
			}
			return n, compacted
		}
	}
	return 0, false
}
