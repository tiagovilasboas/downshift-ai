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
)

type readToolInput struct {
	FilePath string `json:"file_path"`
	Path     string `json:"path"`
	Offset   int    `json:"offset"`
	Limit    int    `json:"limit"`
}

type readSnapshot struct {
	MTime int64  `json:"mtime"`
	Size  int64  `json:"size"`
	At    int64  `json:"at"`
	ID    string `json:"id,omitempty"` // CCR handle (read-seen only)
}

// DedupRead suppresses redundant repeat file-read outputs within the same
// session when the underlying file's mtime and size are unchanged within
// readDedupTTL, rawOutput is at least readDedupMinBytes, and ccrID is set so
// the model can still recover the full text via `downshift retrieve`.
//
// First reads, modified files, distinct offset/limit slices, missing sessions,
// stat failures, and any storage error fail open and return (rawOutput, false)
// so Edit calls never break.
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
	mtime := info.ModTime().UnixNano()
	size := info.Size()

	if prev, hit := readDedupSnapshot(keyPath, readDedupTTL, now); hit && prev.MTime == mtime && prev.Size == size {
		if len(rawOutput) >= readDedupMinBytes && ccrID != "" {
			ptr := fmt.Sprintf("[downshift: unchanged %s (%d lines, %d B) already read in this session; downshift retrieve %s]\n",
				filepath.Base(cleanPath), countLines(rawOutput), len(rawOutput), readRetrieveArg(sessionID, ccrID))
			if len(ptr) < len(rawOutput) {
				return ptr, true
			}
		}
	}

	writeDedupSnapshot(keyPath, readSnapshot{MTime: mtime, Size: size, At: now.UnixNano()}, now)
	pruneDedupDir(dir, keyPath, readDedupTTL, now)
	return rawOutput, false
}

func parseReadInput(toolInput json.RawMessage) (cleanPath string, offset, limit int, ok bool) {
	if len(toolInput) == 0 {
		return "", 0, 0, false
	}
	var in readToolInput
	if err := json.Unmarshal(toolInput, &in); err != nil {
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

func dedupKeyPath(dir, sessionID, cleanPath string, offset, limit int) string {
	sessSum := sha256.Sum256([]byte("downshift-read-dedup-session-v1:" + sessionID))
	sessHash := hex.EncodeToString(sessSum[:])
	scope := fmt.Sprintf("%s\x00%s\x00%d\x00%d", sessHash, cleanPath, offset, limit)
	keySum := sha256.Sum256([]byte(scope))
	return filepath.Join(dir, hex.EncodeToString(keySum[:])[:24]+".json")
}

func readDedupSnapshot(path string, ttl time.Duration, now time.Time) (readSnapshot, bool) {
	if _, err := paths.RegularFile(path); err != nil {
		return readSnapshot{}, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return readSnapshot{}, false
	}
	var snap readSnapshot
	if err := json.Unmarshal(data, &snap); err != nil || snap.Size < 0 || snap.At <= 0 {
		return readSnapshot{}, false
	}
	age := now.Sub(time.Unix(0, snap.At))
	if age < 0 || (ttl > 0 && age > ttl) {
		_ = os.Remove(path)
		return readSnapshot{}, false
	}
	return snap, true
}

func writeDedupSnapshot(path string, snap readSnapshot, now time.Time) {
	dir := filepath.Dir(path)
	if err := paths.PrivateDir(dir); err != nil {
		return
	}
	data, err := json.Marshal(snap)
	if err != nil {
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
		if strings.HasPrefix(name, ".dedup-") && now.Sub(info.ModTime()) > readDedupTempAge {
			_ = os.Remove(p)
			continue
		}
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		if p != keepPath && ttl > 0 && now.Sub(info.ModTime()) > ttl {
			_ = os.Remove(p)
			continue
		}
		live = append(live, item{path: p, mtime: info.ModTime()})
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
