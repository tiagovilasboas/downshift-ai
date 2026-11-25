// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"encoding/json"
	"path/filepath"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/paths"
)

// RetrievalsFileName is the log of bytes `downshift retrieve` handed back to the
// agent, under the state directory. A compacted run that kept its raw output
// saved bytes only until the agent read them back; this log is how the
// summaries find out.
const RetrievalsFileName = "exec-retrievals.jsonl"

// Retrieval is one read of kept output. It carries the CCR id (a hash of the
// stored text), the number of bytes returned and the time. Never the output,
// the query, a path or a command.
//
// Session is the session reference (SessionRef) the kept-output hint handed
// the agent, so a read is charged to the session that kept the output. It is
// empty for a read that came without one (the MCP tool, older rows) and then
// omitted from the row.
type Retrieval struct {
	TS      string `json:"ts"`
	Kept    string `json:"kept"`
	Bytes   int64  `json:"bytes"`
	Session string `json:"session,omitempty"`
	Tool    string `json:"tool,omitempty"`
	Family  string `json:"family,omitempty"`
}

// RetrievalMeta carries optional provenance for RecordRetrievalMeta.
type RetrievalMeta struct {
	Tool   string
	Family string
}

// sessionRefLen is the length of a session reference: the first 12 hex
// characters of the hashed session, like a CCR id.
const sessionRefLen = 12

// SessionRef returns the short stable reference of a hashed session: its first
// 12 characters, when h is 12 to 64 lowercase hex characters. Anything else
// gives "" (no session). The kept-output hint prints it, `retrieve` records
// it, and matching compares it with the prefix of an event's Session.
func SessionRef(h string) string {
	if len(h) < sessionRefLen || len(h) > 64 {
		return ""
	}
	for _, c := range h {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return ""
		}
	}
	return h[:sessionRefLen]
}

// RecordRetrieval appends one retrieval. The CLI and the MCP tool both call it
// with the length of what they returned. session is a hashed session or its
// reference (SessionRef), "" when the read came without one; anything else is
// stored as no session. It never fails the read: errors are dropped, and an id
// that is not 12 lowercase hex characters or a read of zero bytes records
// nothing.
func RecordRetrieval(id, session string, returned int, now time.Time) {
	RecordRetrievalMeta(id, session, returned, RetrievalMeta{}, now)
}

// RecordRetrievalMeta appends one retrieval with optional provenance metadata
// (tool and family). See RecordRetrieval for validation and error handling.
func RecordRetrievalMeta(id, session string, returned int, meta RetrievalMeta, now time.Time) {
	if returned <= 0 || !validKeptID(id) {
		return
	}
	path, err := paths.Join(RetrievalsFileName)
	if err != nil {
		return
	}
	line, err := json.Marshal(Retrieval{
		TS:      now.UTC().Format(time.RFC3339),
		Kept:    id,
		Bytes:   int64(returned),
		Session: SessionRef(session),
		Tool:    meta.Tool,
		Family:  meta.Family,
	})
	if err != nil {
		return
	}
	_ = appendLine(path, line)
}

// retrievalsPath is the retrievals log that sits beside the exec log at
// eventsPath, so a test or a caller with its own events file reads its own.
func retrievalsPath(eventsPath string) string {
	return filepath.Join(filepath.Dir(eventsPath), RetrievalsFileName)
}

// ReadRetrievalEntries reads all valid Retrieval records from the retrievals
// log beside eventsPath (rotated generation included). Malformed lines and
// non-positive or invalid-id rows are skipped.
func ReadRetrievalEntries(eventsPath string) []Retrieval {
	var out []Retrieval
	_ = scanLog(retrievalsPath(eventsPath), func(line []byte) {
		var r Retrieval
		if json.Unmarshal(line, &r) != nil || r.Bytes <= 0 || !validKeptID(r.Kept) {
			return
		}
		out = append(out, r)
	})
	return out
}

// ReadRetrievals totals the bytes read back per CCR id from the retrievals log
// beside the exec log at eventsPath (rotated generation included), for the
// reads that carry no session. A missing file is an empty map; malformed lines
// are skipped.
func ReadRetrievals(eventsPath string) map[string]int64 {
	byID, _ := splitRetrievals(ReadRetrievalEntries(eventsPath))
	return byID
}

// validKeptID reports whether id looks like a CCR id: 12 lowercase hex
// characters. execwrap does not import internal/ccr, so it checks the shape.
func validKeptID(id string) bool {
	if len(id) != 12 {
		return false
	}
	for _, c := range id {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}
