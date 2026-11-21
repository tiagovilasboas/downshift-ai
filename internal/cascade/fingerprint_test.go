// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package cascade

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFingerprint_StableWithinInstall(t *testing.T) {
	t.Setenv("DOWNSHIFT_STATE_DIR", t.TempDir())
	a, b := TaskFingerprint("Run the tests"), TaskFingerprint("  run the TESTS ")
	if a == "" || a != b {
		t.Fatalf("same text, same install: %q vs %q", a, b)
	}
	if TaskFingerprint("other task") == a {
		t.Error("different text gave the same key")
	}
}

func TestFingerprint_DifferentSaltDifferentKey(t *testing.T) {
	t.Setenv("DOWNSHIFT_STATE_DIR", t.TempDir())
	a := TaskFingerprint("same task")
	t.Setenv("DOWNSHIFT_STATE_DIR", t.TempDir())
	if b := TaskFingerprint("same task"); a == "" || a == b {
		t.Errorf("two installs share a key: %q %q", a, b)
	}
}

func TestFingerprint_SaltIsPrivateFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DOWNSHIFT_STATE_DIR", dir)
	TaskFingerprint("x")
	info, err := os.Stat(filepath.Join(dir, "dead_ends.salt"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0077 != 0 && filepath.Separator == '/' {
		t.Errorf("salt mode %v, want owner-only", info.Mode().Perm())
	}
}

func TestFingerprint_MissingSaltRecordsNothing(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DOWNSHIFT_STATE_DIR", filepath.Join(blocker, "state")) // cannot be created
	if fp := TaskFingerprint("a task"); fp != "" {
		t.Fatalf("fingerprint without a salt: %q", fp)
	}
	RecordFailure("sess", "a task", "m", "failed")
	if _, found := HasRecentFailure("sess", "a task", time.Minute); found {
		t.Error("a failure was recorded or found without a salt")
	}
}

func TestDeadEnds_NoRawTextOrPlainHashOnDisk(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DOWNSHIFT_STATE_DIR", dir)
	task := "deploy the secret project"
	RecordFailure("sess", task, "m", "failed")
	data, err := os.ReadFile(filepath.Join(dir, "dead_ends.json"))
	if err != nil {
		t.Fatal(err)
	}
	plain := sha256.Sum256([]byte(task))
	if strings.Contains(string(data), task) || strings.Contains(string(data), hex.EncodeToString(plain[:8])) {
		t.Errorf("task text or its plain hash is on disk: %s", data)
	}
}

func TestDeadEnds_LegacyUnsaltedEntriesDropped(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DOWNSHIFT_STATE_DIR", dir)
	task := "old task"
	plain := sha256.Sum256([]byte(task))
	legacy := `[{"session_id":"s","fingerprint":"` + hex.EncodeToString(plain[:8]) +
		`","reason":"r","timestamp":"` + time.Now().UTC().Format(time.RFC3339) + `"}]`
	if err := os.WriteFile(filepath.Join(dir, "dead_ends.json"), []byte(legacy), 0600); err != nil {
		t.Fatal(err)
	}
	if _, found := HasRecentFailure("s", task, time.Minute); found {
		t.Error("a legacy unsalted entry was honored")
	}
	RecordFailure("s", "new task", "m", "failed")
	data, _ := os.ReadFile(filepath.Join(dir, "dead_ends.json"))
	if strings.Contains(string(data), hex.EncodeToString(plain[:8])) {
		t.Error("the legacy hash survived a rewrite")
	}
}
