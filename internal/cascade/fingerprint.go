// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package cascade

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"

	"github.com/tiagovilasboas/downshift-ai/internal/paths"
)

// fingerprintPrefix marks a keyed fingerprint. Entries written before the
// salt existed carry a bare unsalted hash that a dictionary can reverse; they
// lack the prefix and are dropped on read.
const fingerprintPrefix = "k1:"

const saltBytes = 32

// TaskFingerprint returns a keyed (HMAC-SHA256, per-install random salt)
// 16-hex digest of a task or command, equal for equal text within one
// install and meaningless elsewhere. It returns "" for blank text or when the
// salt cannot be read or created (fail open: nothing is recorded).
func TaskFingerprint(text string) string {
	cleaned := strings.TrimSpace(strings.ToLower(text))
	if cleaned == "" {
		return ""
	}
	salt := installSalt()
	if salt == nil {
		return ""
	}
	m := hmac.New(sha256.New, salt)
	m.Write([]byte(cleaned))
	return fingerprintPrefix + hex.EncodeToString(m.Sum(nil)[:8])
}

// blank reports whether text carries no task.
func blank(text string) bool { return strings.TrimSpace(text) == "" }

// installSalt reads the random salt under the state dir, creating it (0600)
// on first use. It returns nil when it can do neither.
func installSalt() []byte {
	p, err := paths.Join("dead_ends.salt")
	if err != nil {
		return nil
	}
	if b := readSalt(p); b != nil {
		return b
	}
	if os.MkdirAll(filepath.Dir(p), 0700) != nil {
		return nil
	}
	raw := make([]byte, saltBytes)
	if _, err := rand.Read(raw); err != nil {
		return nil
	}
	f, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return readSalt(p) // another process won the race
	}
	_, werr := f.WriteString(hex.EncodeToString(raw))
	if cerr := f.Close(); werr != nil || cerr != nil {
		_ = os.Remove(p)
		return nil
	}
	return readSalt(p)
}

func readSalt(p string) []byte {
	data, err := os.ReadFile(p)
	if err != nil {
		return nil
	}
	b, err := hex.DecodeString(strings.TrimSpace(string(data)))
	if err != nil || len(b) != saltBytes {
		return nil
	}
	return b
}
