// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package postrun

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/paths"
)

func makeFileContent(lines int) string {
	var b strings.Builder
	for i := 1; i <= lines; i++ {
		fmt.Fprintf(&b, "line %04d: package postrun sample content for dedup verification\n", i)
	}
	return b.String()
}

func readInputJSON(t *testing.T, fields map[string]any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestDedupRead_FirstAndRepeatWithinTTL(t *testing.T) {
	t.Setenv(paths.EnvStateDir, t.TempDir())
	filePath := filepath.Join(t.TempDir(), "sample.go")
	content := makeFileContent(30) // > 1024 B
	if err := os.WriteFile(filePath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	input := readInputJSON(t, map[string]any{"file_path": filePath})
	sess := strings.Repeat("ab", 32)

	got1, changed1 := DedupRead(sess, input, content, "ccr001122334")
	if changed1 || got1 != content {
		t.Fatalf("first read must pass through: changed=%v len=%d", changed1, len(got1))
	}

	got2, changed2 := DedupRead(sess, input, content, "ccr001122334")
	if !changed2 {
		t.Fatalf("repeat read should be deduped")
	}
	want := fmt.Sprintf("[downshift: unchanged sample.go (30 lines, %d B) already read in this session; downshift retrieve --session=abababababab ccr001122334]\n", len(content))
	if got2 != want {
		t.Errorf("got %q, want %q", got2, want)
	}
	if len(got2) >= len(content) {
		t.Errorf("pointer len %d >= raw len %d", len(got2), len(content))
	}
}

func TestDedupRead_PrivacyAndPermissions(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv(paths.EnvStateDir, stateDir)
	rawSess := "secret-raw-session-token"
	filePath := filepath.Join(t.TempDir(), "secret-file-name.go")
	content := makeFileContent(30)
	if err := os.WriteFile(filePath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	input := readInputJSON(t, map[string]any{"file_path": filePath})

	if _, changed := DedupRead(rawSess, input, content, ""); changed {
		t.Fatal("first read changed output")
	}
	dedupDir := filepath.Join(stateDir, readDedupSubdir)
	entries, err := os.ReadDir(dedupDir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("expected 1 state file in %s, got %d (err=%v)", dedupDir, len(entries), err)
	}
	name := entries[0].Name()
	if strings.Contains(name, rawSess) || strings.Contains(name, "secret-file-name") {
		t.Errorf("state filename leaked raw session or path: %s", name)
	}
	full := filepath.Join(dedupDir, name)
	info, err := os.Stat(full)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != paths.FilePerm {
		t.Errorf("state file perm = %v, want %v", info.Mode().Perm(), paths.FilePerm)
	}
	body, err := os.ReadFile(full)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), rawSess) || strings.Contains(string(body), "secret-file-name") {
		t.Errorf("state file body leaked raw session or path: %s", string(body))
	}
}

func TestDedupRead_FileModifiedInvalidates(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(t.TempDir(), "edit.go")
	v1 := makeFileContent(30)
	if err := os.WriteFile(filePath, []byte(v1), 0o600); err != nil {
		t.Fatal(err)
	}
	t0 := time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)
	if err := os.Chtimes(filePath, t0, t0); err != nil {
		t.Fatal(err)
	}
	input := readInputJSON(t, map[string]any{"file_path": filePath})

	if got, changed := dedupReadAt("s1", input, v1, "ccr111111111", dir, t0); changed || got != v1 {
		t.Fatalf("first read changed=%v", changed)
	}
	if _, changed := dedupReadAt("s1", input, v1, "ccr111111111", dir, t0.Add(time.Minute)); !changed {
		t.Fatal("expected repeat read to dedup")
	}

	// Same size, newer mtime -> must invalidate.
	t1 := t0.Add(2 * time.Minute)
	v2 := strings.Replace(v1, "line 0001", "line 9999", 1)
	if err := os.WriteFile(filePath, []byte(v2), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(filePath, t1, t1); err != nil {
		t.Fatal(err)
	}
	if got, changed := dedupReadAt("s1", input, v2, "ccr222222222", dir, t1); changed || got != v2 {
		t.Fatalf("mtime change must return raw: changed=%v", changed)
	}
	if _, changed := dedupReadAt("s1", input, v2, "ccr222222222", dir, t1.Add(time.Second)); !changed {
		t.Fatal("repeat read after updated snapshot should dedup")
	}

	// Different size even with same mtime -> must invalidate.
	v3 := v2 + "extra line\n"
	if err := os.WriteFile(filePath, []byte(v3), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(filePath, t1, t1); err != nil {
		t.Fatal(err)
	}
	if got, changed := dedupReadAt("s1", input, v3, "ccr333333333", dir, t1.Add(2*time.Second)); changed || got != v3 {
		t.Fatalf("size change must return raw: changed=%v", changed)
	}
}

func TestDedupRead_OffsetLimitSessionIsolationAndTTL(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(t.TempDir(), "slice.go")
	content := makeFileContent(40)
	if err := os.WriteFile(filePath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	t0 := time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)

	fullIn := readInputJSON(t, map[string]any{"file_path": filePath})
	sliceA := readInputJSON(t, map[string]any{"file_path": filePath, "offset": 10, "limit": 20})
	sliceB := readInputJSON(t, map[string]any{"file_path": filePath, "offset": 20, "limit": 20})

	// Seed full read in s1.
	dedupReadAt("s1", fullIn, content, "ccr000000001", dir, t0)

	// Different offset/limit does not match full read.
	if got, changed := dedupReadAt("s1", sliceA, content, "ccr000000001", dir, t0.Add(time.Second)); changed || got != content {
		t.Fatal("different offset/limit should not dedup")
	}
	// Repeat of sliceA dedups, while sliceB does not.
	if _, changed := dedupReadAt("s1", sliceA, content, "ccr000000001", dir, t0.Add(2*time.Second)); !changed {
		t.Fatal("repeat sliceA should dedup")
	}
	if _, changed := dedupReadAt("s1", sliceB, content, "ccr000000001", dir, t0.Add(3*time.Second)); changed {
		t.Fatal("sliceB should not dedup on first read")
	}

	// Different session does not dedup.
	if _, changed := dedupReadAt("s2", fullIn, content, "ccr000000001", dir, t0.Add(4*time.Second)); changed {
		t.Fatal("other session must not dedup")
	}

	// TTL expiry resets snapshot: repeat after > 10m returns raw, then dedups again.
	afterTTL := t0.Add(readDedupTTL + time.Second)
	if got, changed := dedupReadAt("s1", fullIn, content, "ccr000000001", dir, afterTTL); changed || got != content {
		t.Fatal("expired read must return raw")
	}
	if _, changed := dedupReadAt("s1", fullIn, content, "ccr000000001", dir, afterTTL.Add(time.Second)); !changed {
		t.Fatal("read right after TTL refresh should dedup")
	}
}

func TestDedupRead_FailOpenGuards(t *testing.T) {
	dir := t.TempDir()
	t0 := time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)

	filePath := filepath.Join(t.TempDir(), "guard.go")
	big := makeFileContent(30)
	if err := os.WriteFile(filePath, []byte(big), 0o600); err != nil {
		t.Fatal(err)
	}
	input := readInputJSON(t, map[string]any{"file_path": filePath})

	// Empty session never dedups.
	dedupReadAt("", input, big, "ccr000000001", dir, t0)
	if _, changed := dedupReadAt("", input, big, "ccr000000001", dir, t0.Add(time.Second)); changed {
		t.Fatal("empty session deduped")
	}

	// Empty ccrID never dedups, even after prior read.
	dedupReadAt("s1", input, big, "", dir, t0)
	if _, changed := dedupReadAt("s1", input, big, "", dir, t0.Add(time.Second)); changed {
		t.Fatal("empty ccrID deduped")
	}
	// Once ccrID is present on next read, snapshot recorded on first read triggers dedup.
	if _, changed := dedupReadAt("s1", input, big, "ccr000000001", dir, t0.Add(2*time.Second)); !changed {
		t.Fatal("expected dedup once ccrID is provided")
	}

	// Tiny read (< 512 bytes) never dedups.
	tinyPath := filepath.Join(t.TempDir(), "tiny.go")
	tiny := "package main\nfunc main() {}\n"
	if err := os.WriteFile(tinyPath, []byte(tiny), 0o600); err != nil {
		t.Fatal(err)
	}
	tinyIn := readInputJSON(t, map[string]any{"path": tinyPath}) // also verifies "path" fallback
	dedupReadAt("s1", tinyIn, tiny, "ccr000000002", dir, t0)
	if _, changed := dedupReadAt("s1", tinyIn, tiny, "ccr000000002", dir, t0.Add(time.Second)); changed {
		t.Fatal("tiny output deduped")
	}

	// Malformed JSON / missing file_path / missing file / directory fail open.
	for _, bad := range []json.RawMessage{nil, json.RawMessage(`{bad`), json.RawMessage(`{}`)} {
		if got, changed := dedupReadAt("s1", bad, big, "ccr000000001", dir, t0); changed || got != big {
			t.Errorf("bad input %s did not fail open", string(bad))
		}
	}
	missingIn := readInputJSON(t, map[string]any{"file_path": filepath.Join(t.TempDir(), "missing.go")})
	if got, changed := dedupReadAt("s1", missingIn, big, "ccr000000001", dir, t0); changed || got != big {
		t.Error("missing file did not fail open")
	}
	dirIn := readInputJSON(t, map[string]any{"file_path": t.TempDir()})
	if got, changed := dedupReadAt("s1", dirIn, big, "ccr000000001", dir, t0); changed || got != big {
		t.Error("directory did not fail open")
	}

	// Corrupt state file or symlink state file fails open.
	corruptKey := dedupKeyPath(dir, "s-corrupt", filePath, 0, 0)
	if err := os.WriteFile(corruptKey, []byte("{corrupt"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, changed := dedupReadAt("s-corrupt", input, big, "ccr000000001", dir, t0); changed || got != big {
		t.Error("corrupt state file did not fail open")
	}
	symTarget := filepath.Join(t.TempDir(), "target.json")
	_ = os.WriteFile(symTarget, []byte(`{"mtime":1,"size":1,"at":1}`), 0o600)
	symKey := dedupKeyPath(dir, "s-sym", filePath, 0, 0)
	if err := os.Symlink(symTarget, symKey); err == nil {
		if got, changed := dedupReadAt("s-sym", input, big, "ccr000000001", dir, t0); changed || got != big {
			t.Error("symlink state file did not fail open")
		}
	}

	// Pointer >= rawOutput length fails open.
	exactMin := strings.Repeat("x", readDedupMinBytes)
	exactPath := filepath.Join(t.TempDir(), "exact.txt")
	if err := os.WriteFile(exactPath, []byte(exactMin), 0o600); err != nil {
		t.Fatal(err)
	}
	exactIn := readInputJSON(t, map[string]any{"file_path": exactPath})
	longCCRID := strings.Repeat("c", readDedupMinBytes)
	dedupReadAt("s1", exactIn, exactMin, longCCRID, dir, t0)
	if got, changed := dedupReadAt("s1", exactIn, exactMin, longCCRID, dir, t0.Add(time.Second)); changed || got != exactMin {
		t.Error("pointer >= rawOutput should fail open")
	}
}

func TestDedupRead_PrunesExpiredAndBoundsCount(t *testing.T) {
	dir := t.TempDir()
	t0 := time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)

	// Seed stale temp file and expired entry.
	staleTmp := filepath.Join(dir, ".dedup-old.tmp")
	expiredJSON := filepath.Join(dir, "000000000000000000000000.json")
	for _, p := range []string{staleTmp, expiredJSON} {
		if err := os.WriteFile(p, []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
		oldTime := t0.Add(-readDedupTTL - time.Minute)
		if err := os.Chtimes(p, oldTime, oldTime); err != nil {
			t.Fatal(err)
		}
	}

	// Seed more than readDedupMaxFiles live entries.
	for i := 0; i < readDedupMaxFiles+5; i++ {
		p := filepath.Join(dir, fmt.Sprintf("%024x.json", i+1))
		if err := os.WriteFile(p, []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
		mt := t0.Add(time.Duration(i) * time.Millisecond)
		if err := os.Chtimes(p, mt, mt); err != nil {
			t.Fatal(err)
		}
	}

	filePath := filepath.Join(t.TempDir(), "prune.go")
	big := makeFileContent(30)
	if err := os.WriteFile(filePath, []byte(big), 0o600); err != nil {
		t.Fatal(err)
	}
	now := t0.Add(10 * time.Second)
	dedupReadAt("s1", readInputJSON(t, map[string]any{"file_path": filePath}), big, "ccr000000001", dir, now)

	if _, err := os.Stat(staleTmp); !os.IsNotExist(err) {
		t.Error("stale temp file was not pruned")
	}
	if _, err := os.Stat(expiredJSON); !os.IsNotExist(err) {
		t.Error("expired json file was not pruned")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) > readDedupMaxFiles {
		t.Errorf("dir has %d entries, want <= %d", len(entries), readDedupMaxFiles)
	}
}

func TestDedupRead_MidBand512To1024(t *testing.T) {
	t.Setenv(paths.EnvStateDir, t.TempDir())
	filePath := filepath.Join(t.TempDir(), "mid.go")
	content := makeFileContent(12) // ~700 B: in [512, 1024)
	if len(content) < 512 || len(content) >= 1024 {
		t.Fatalf("fixture len = %d, want in [512, 1024)", len(content))
	}
	if err := os.WriteFile(filePath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	input := readInputJSON(t, map[string]any{"file_path": filePath})
	sess := strings.Repeat("ab", 32)

	got1, changed1 := DedupRead(sess, input, content, "ccrmid000001")
	if changed1 || got1 != content {
		t.Fatalf("first read must stay verbatim for Edit: changed=%v", changed1)
	}
	got2, changed2 := DedupRead(sess, input, content, "ccrmid000001")
	if !changed2 || !strings.Contains(got2, "unchanged mid.go") || len(got2) >= len(content) {
		t.Fatalf("repeat mid-band read not deduped to shorter pointer: changed=%v got=%q", changed2, got2)
	}
}
