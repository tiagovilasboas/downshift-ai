// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package postrun

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/paths"
)

const (
	readDedupSubdir   = "read-dedup"
	readDedupTTL      = 10 * time.Minute
	readDedupMinBytes = 512
	readDedupMaxFiles = 512
	readDedupTempAge  = time.Minute
	readDedupMaxSpans = 16
	readDedupMaxLines = 4096
)

type readToolInput struct {
	FilePath string `json:"file_path"`
	Path     string `json:"path"`
	Offset   int    `json:"offset"`
	Limit    int    `json:"limit"`
}

type readSpan struct {
	Offset      int      `json:"offset,omitempty"`
	Limit       int      `json:"limit,omitempty"`
	Start       int      `json:"start"`
	End         int      `json:"end"`
	At          int64    `json:"at"`
	ContentHash string   `json:"content_hash,omitempty"`
	LineHashes  []string `json:"line_hashes,omitempty"`
}

type readSnapshot struct {
	MTime int64      `json:"mtime"`
	Size  int64      `json:"size"`
	At    int64      `json:"at"`
	ID    string     `json:"id,omitempty"` // CCR handle (read-seen only)
	Spans []readSpan `json:"spans,omitempty"`
}

// DedupRead suppresses redundant repeat and unchanged sub-range file-read
// outputs within the same session when mtime, size, and recorded content
// hashes match within readDedupTTL, rawOutput >= readDedupMinBytes, and ccrID
// is set. Modified files, non-matching ranges, missing sessions, stat failures,
// and storage errors fail open and return (rawOutput, false).
func DedupRead(sessionID string, toolInput json.RawMessage, rawOutput string, ccrID string) (compacted string, changed bool) {
	defer func() {
		if recover() != nil {
			compacted, changed = rawOutput, false
		}
	}()
	dir, err := paths.Join(readDedupSubdir)
	if err != nil {
		return rawOutput, false
	}
	return dedupReadAt(sessionID, toolInput, rawOutput, ccrID, dir, time.Now())
}

func dedupReadAt(sessionID string, toolInput json.RawMessage, rawOutput string, ccrID string, dir string, now time.Time) (string, bool) {
	if sessionID == "" || rawOutput == "" {
		return rawOutput, false
	}
	cleanPath, offset, limit, ok := parseReadInput(toolInput)
	if !ok {
		return rawOutput, false
	}
	keyPath := dedupKeyPath(dir, sessionID, cleanPath, offset, limit)
	info, err := os.Stat(cleanPath)
	if err != nil || info.IsDir() {
		_ = os.Remove(keyPath)
		return rawOutput, false
	}
	mtime, size := info.ModTime().UnixNano(), info.Size()
	cur := buildReadSpan(offset, limit, rawOutput, now)

	var spans []readSpan
	recorded := false
	if prev, prevHit := readDedupSnapshot(keyPath, readDedupTTL, now); prevHit && prev.MTime == mtime && prev.Size == size {
		var live []readSpan
		for _, s := range prev.Spans {
			age := now.Sub(time.Unix(0, s.At))
			if s.At > 0 && age >= 0 && age <= readDedupTTL {
				live = append(live, s)
			}
		}
		exact, matched, hit, stale := matchReadSpan(live, cur)
		if hit {
			if len(rawOutput) >= readDedupMinBytes && ccrID != "" {
				arg := readRetrieveArg(sessionID, ccrID)
				ptr := fmt.Sprintf("[downshift: unchanged %s lines %d-%d (%d B) already read in lines %d-%d this session; downshift retrieve %s]\n",
					filepath.Base(cleanPath), cur.Start, cur.End, len(rawOutput), matched.Start, matched.End, arg)
				if exact {
					ptr = fmt.Sprintf("[downshift: unchanged %s (%d lines, %d B) already read in this session; downshift retrieve %s]\n",
						filepath.Base(cleanPath), countLines(rawOutput), len(rawOutput), arg)
				}
				if len(ptr) < len(rawOutput) {
					return ptr, true
				}
			}
			spans, recorded = live, true
		} else if !stale {
			for _, s := range live {
				if (s.Offset == cur.Offset && s.Limit == cur.Limit) || (len(cur.LineHashes) > 0 && cur.Start <= s.Start && cur.End >= s.End) {
					continue
				}
				spans = append(spans, s)
			}
		}
	}
	if !recorded {
		spans = append(spans, cur)
		if len(spans) > readDedupMaxSpans {
			spans = spans[len(spans)-readDedupMaxSpans:]
		}
	}
	writeDedupSnapshot(keyPath, readSnapshot{MTime: mtime, Size: size, At: now.UnixNano(), Spans: spans}, now)
	pruneDedupDir(dir, keyPath, readDedupTTL, now)
	return rawOutput, false
}

func buildReadSpan(offset, limit int, raw string, now time.Time) readSpan {
	sum := sha256.Sum256([]byte(raw))
	lineSpans := splitLineSpans(raw)
	start := offset
	if start <= 0 {
		start = 1
	}
	span := readSpan{
		Offset: offset, Limit: limit, Start: start, End: start + len(lineSpans) - 1,
		At: now.UnixNano(), ContentHash: hex.EncodeToString(sum[:16]),
	}
	if len(lineSpans) > 0 && len(lineSpans) <= readDedupMaxLines {
		hashes := make([]string, len(lineSpans))
		for i, sp := range lineSpans {
			ls := sha256.Sum256([]byte(strings.TrimSuffix(raw[sp.start:sp.end], "\n")))
			hashes[i] = hex.EncodeToString(ls[:8])
		}
		span.LineHashes = hashes
	}
	return span
}

func matchReadSpan(spans []readSpan, cur readSpan) (exact bool, matched readSpan, hit bool, stale bool) {
	for _, s := range spans {
		if s.Offset == cur.Offset && s.Limit == cur.Limit {
			if s.ContentHash != "" && s.ContentHash == cur.ContentHash {
				return true, s, true, false
			}
			return false, readSpan{}, false, true
		}
	}
	if cur.Offset < 0 || cur.Limit < 0 || len(cur.LineHashes) == 0 {
		return false, readSpan{}, false, false
	}
	for _, s := range spans {
		idx := cur.Start - s.Start
		if len(s.LineHashes) == 0 || cur.Start < s.Start || cur.End > s.End || idx < 0 || idx+len(cur.LineHashes) > len(s.LineHashes) {
			continue
		}
		if slices.Equal(s.LineHashes[idx:idx+len(cur.LineHashes)], cur.LineHashes) {
			return false, s, true, false
		}
		stale = true
	}
	return false, readSpan{}, false, stale
}

func parseReadInput(toolInput json.RawMessage) (cleanPath string, offset, limit int, ok bool) {
	var in readToolInput
	if len(toolInput) == 0 || json.Unmarshal(toolInput, &in) != nil {
		return "", 0, 0, false
	}
	p := in.FilePath
	if p == "" {
		p = in.Path
	}
	if p == "" {
		return "", 0, 0, false
	}
	return filepath.Clean(p), in.Offset, in.Limit, true
}

func dedupKeyPath(dir, sessionID, cleanPath string, _, _ int) string {
	sessSum := sha256.Sum256([]byte("downshift-read-dedup-session-v1:" + sessionID))
	keySum := sha256.Sum256([]byte(fmt.Sprintf("%s\x00%s", hex.EncodeToString(sessSum[:]), cleanPath)))
	return filepath.Join(dir, hex.EncodeToString(keySum[:])[:24]+".json")
}

func readDedupSnapshot(path string, ttl time.Duration, now time.Time) (readSnapshot, bool) {
	if _, err := paths.RegularFile(path); err != nil {
		return readSnapshot{}, false
	}
	data, err := os.ReadFile(path)
	var snap readSnapshot
	if err != nil || json.Unmarshal(data, &snap) != nil || snap.Size < 0 || snap.At <= 0 {
		return readSnapshot{}, false
	}
	if age := now.Sub(time.Unix(0, snap.At)); age < 0 || (ttl > 0 && age > ttl) {
		_ = os.Remove(path)
		return readSnapshot{}, false
	}
	return snap, true
}

func writeDedupSnapshot(path string, snap readSnapshot, now time.Time) {
	dir := filepath.Dir(path)
	data, err := json.Marshal(snap)
	if err != nil || paths.PrivateDir(dir) != nil {
		return
	}
	tmp, err := os.CreateTemp(dir, ".dedup-*.tmp")
	if err != nil {
		return
	}
	tmpName := tmp.Name()
	_, werr := tmp.Write(data)
	if cerr := tmp.Close(); werr == nil {
		werr = cerr
	}
	if werr == nil {
		werr = os.Chmod(tmpName, paths.FilePerm)
	}
	if werr == nil {
		werr = os.Rename(tmpName, path)
	}
	if werr != nil {
		_ = os.Remove(tmpName)
		return
	}
	_ = os.Chtimes(path, now, now)
}

func pruneDedupDir(dir, keepPath string, ttl time.Duration, now time.Time) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	type item struct {
		path  string
		mtime time.Time
	}
	var live []item
	for _, e := range entries {
		name := e.Name()
		info, err := e.Info()
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		p := filepath.Join(dir, name)
		if (strings.HasPrefix(name, ".dedup-") && now.Sub(info.ModTime()) > readDedupTempAge) ||
			(strings.HasSuffix(name, ".json") && p != keepPath && ttl > 0 && now.Sub(info.ModTime()) > ttl) {
			_ = os.Remove(p)
			continue
		}
		if strings.HasSuffix(name, ".json") {
			live = append(live, item{path: p, mtime: info.ModTime()})
		}
	}
	if len(live) <= readDedupMaxFiles {
		return
	}
	sort.Slice(live, func(i, j int) bool { return live[i].mtime.Before(live[j].mtime) })
	for i := 0; i < len(live)-readDedupMaxFiles; i++ {
		if live[i].path != keepPath {
			_ = os.Remove(live[i].path)
		}
	}
}

func countLines(s string) int {
	if s == "" {
		return 0
	}
	n := strings.Count(s, "\n")
	if !strings.HasSuffix(s, "\n") {
		n++
	}
	return n
}
