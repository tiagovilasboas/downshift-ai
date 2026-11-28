// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package postrun

import (
	"encoding/json"
	"path/filepath"
	"strings"
)

// ReadOnlyFileThreshold is the lower Read result size (8 KiB) before
// middle-range compaction triggers on read-only, generated, lock, or log files.
const ReadOnlyFileThreshold = 8 << 10

const (
	readOnlyHeadBudget = 2 << 10
	readOnlyTailBudget = 2 << 10
)

// readOnlyBasenames lists exact lowercase filenames that are generated or
// read-only lockfiles where keeping 2 KiB head + 2 KiB tail with full CCR
// storage is safe without impacting Edit on normal source files.
var readOnlyBasenames = map[string]bool{
	"package-lock.json": true,
	"pnpm-lock.yaml":    true,
	"yarn.lock":         true,
	"cargo.lock":        true,
	"composer.lock":     true,
	"poetry.lock":       true,
	"uv.lock":           true,
	"go.sum":            true,
	"gemfile.lock":      true,
	"flake.lock":        true,
	"bun.lock":          true,
	"bun.lockb":         true,
}

// readOnlySuffixes lists lowercase extensions of generated, minified, map, or
// append-only log files eligible for ReadOnlyFileThreshold.
var readOnlySuffixes = []string{
	".jsonl",
	".log",
	".min.js",
	".min.css",
	".map",
}

// isReadOnlyPath reports whether path matches the conservative allowlist of
// read-only, generated, lock, or log files using OS-agnostic basename matching.
func isReadOnlyPath(path string) bool {
	if path == "" {
		return false
	}
	norm := filepath.FromSlash(strings.ReplaceAll(path, "\\", "/"))
	base := strings.ToLower(filepath.Base(norm))
	if base == "" || base == "." || base == string(filepath.Separator) {
		return false
	}
	if readOnlyBasenames[base] {
		return true
	}
	for _, ext := range readOnlySuffixes {
		if len(base) > len(ext) && strings.HasSuffix(base, ext) {
			return true
		}
	}
	return false
}

// readCutLimits returns the middle-cut threshold and head/tail budgets for a
// Read tool call. Normal source files and calls without a matching path keep
// ReadThreshold (64 KiB) so verbatim Edit matches are unaffected.
func readCutLimits(toolInput json.RawMessage) (threshold, headBudget, tailBudget int) {
	if path, _, _, ok := parseReadInput(toolInput); ok && isReadOnlyPath(path) {
		return ReadOnlyFileThreshold, readOnlyHeadBudget, readOnlyTailBudget
	}
	return ReadThreshold, readHeadBudget, readTailBudget
}
