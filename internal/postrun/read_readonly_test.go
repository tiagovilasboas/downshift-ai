// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package postrun

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/core"
	"github.com/tiagovilasboas/downshift-ai/internal/execwrap"
	"github.com/tiagovilasboas/downshift-ai/internal/hookport"
	"github.com/tiagovilasboas/downshift-ai/internal/paths"
)

func TestIsReadOnlyPath(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		// Exact basenames (various casings & OS separators)
		{"package-lock", "/repo/package-lock.json", true},
		{"pnpm-lock", "/repo/pnpm-lock.yaml", true},
		{"yarn-lock", "yarn.lock", true},
		{"cargo-lock-upper", "/repo/Cargo.lock", true},
		{"composer-lock", "/repo/composer.lock", true},
		{"poetry-lock", "/repo/poetry.lock", true},
		{"uv-lock", "/repo/uv.lock", true},
		{"go-sum", "/repo/go.sum", true},
		{"gemfile-lock-mixed", "/repo/Gemfile.lock", true},
		{"flake-lock", "/repo/flake.lock", true},
		{"bun-lock", "/repo/bun.lock", true},
		{"bun-lockb", "/repo/bun.lockb", true},
		{"windows-path-lock", `C:\Users\dev\app\PACKAGE-LOCK.JSON`, true},

		// Extensions
		{"jsonl", "/var/data/events.jsonl", true},
		{"log", "/var/log/app.log", true},
		{"min-js", "/dist/vendor.min.js", true},
		{"min-css", "/dist/theme.min.css", true},
		{"sourcemap", "/dist/bundle.js.map", true},
		{"windows-upper-log", `C:\logs\SERVER.LOG`, true},

		// Normal source / non-allowlisted files MUST stay false
		{"empty", "", false},
		{"go-source", "/repo/internal/postrun/read.go", false},
		{"ts-source", "/repo/src/index.ts", false},
		{"py-source", "/repo/scripts/main.py", false},
		{"markdown", "/repo/README.md", false},
		{"package-json", "/repo/package.json", false},
		{"plain-json", "/repo/config.json", false},
		{"plain-js", "/repo/dist/app.js", false},
		{"plain-css", "/repo/styles/main.css", false},
		{"go-mod", "/repo/go.mod", false},
		{"bare-ext-log", "/repo/.log", false},
		{"bare-ext-min-js", "/repo/min.js", false},
		{"unknown-lock", "/repo/custom.lock", false},
		{"lock-backup", "/repo/package-lock.json.bak", false},
		{"prefixed-lock", "/repo/my-package-lock.json", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := isReadOnlyPath(tc.path); got != tc.want {
				t.Errorf("isReadOnlyPath(%q) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}
}

func TestCompactRead_ReadOnlyVsNormalThreshold(t *testing.T) {
	t.Setenv(paths.EnvStateDir, t.TempDir())
	raw20K := makeLockfile(250) // ~21 KiB: > 8 KiB, < 64 KiB
	if len(raw20K) <= ReadOnlyFileThreshold || len(raw20K) >= ReadThreshold {
		t.Fatalf("fixture len = %d, want in (%d, %d)", len(raw20K), ReadOnlyFileThreshold, ReadThreshold)
	}
	rawLines := strings.SplitAfter(raw20K, "\n")
	if rawLines[len(rawLines)-1] == "" {
		rawLines = rawLines[:len(rawLines)-1]
	}

	tests := []struct {
		name        string
		toolInput   json.RawMessage
		content     string
		wantCut     bool
		wantKeepHit bool
	}{
		{
			name:        "package-lock.json over 8 KiB compacted with CCR",
			toolInput:   readInputJSON(t, map[string]any{"file_path": "/repo/package-lock.json"}),
			content:     raw20K,
			wantCut:     true,
			wantKeepHit: true,
		},
		{
			name:        "app.log via path key over 8 KiB compacted with CCR",
			toolInput:   readInputJSON(t, map[string]any{"path": "/var/log/app.log"}),
			content:     raw20K,
			wantCut:     true,
			wantKeepHit: true,
		},
		{
			name:        "events.jsonl over 8 KiB compacted with CCR",
			toolInput:   readInputJSON(t, map[string]any{"file_path": "/tmp/events.jsonl"}),
			content:     raw20K,
			wantCut:     true,
			wantKeepHit: true,
		},
		{
			name:        "bundle.min.js over 8 KiB compacted with CCR",
			toolInput:   readInputJSON(t, map[string]any{"file_path": `C:\app\dist\bundle.min.js`}),
			content:     raw20K,
			wantCut:     true,
			wantKeepHit: true,
		},
		{
			name:        "read-only file at or below 8 KiB stays raw",
			toolInput:   readInputJSON(t, map[string]any{"file_path": "/repo/go.sum"}),
			content:     makeLockfile(80), // ~6.7 KiB <= 8 KiB
			wantCut:     false,
			wantKeepHit: false,
		},
		{
			name:        "normal .go file 20 KiB stays raw",
			toolInput:   readInputJSON(t, map[string]any{"file_path": "/repo/internal/main.go"}),
			content:     raw20K,
			wantCut:     false,
			wantKeepHit: false,
		},
		{
			name:        "normal .ts file 20 KiB stays raw",
			toolInput:   readInputJSON(t, map[string]any{"file_path": "/repo/src/app.ts"}),
			content:     raw20K,
			wantCut:     false,
			wantKeepHit: false,
		},
		{
			name:        "missing toolInput 20 KiB stays raw",
			toolInput:   nil,
			content:     raw20K,
			wantCut:     false,
			wantKeepHit: false,
		},
		{
			name:        "empty file_path 20 KiB stays raw",
			toolInput:   readInputJSON(t, map[string]any{"file_path": ""}),
			content:     raw20K,
			wantCut:     false,
			wantKeepHit: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var stored []byte
			keepCalls := 0
			keep := func(b []byte) (string, bool) {
				keepCalls++
				stored = append([]byte(nil), b...)
				return "ccrreadonly1", true
			}
			in := hookport.ToolOutput{
				Category:   core.CategoryRead,
				ToolInput:  tc.toolInput,
				Content:    tc.content,
				HasContent: true,
			}
			res := Compact(allCaps, in, Options{Keep: keep, Balanced: true})

			if res.Replaced != tc.wantCut || res.Compacted != tc.wantCut {
				t.Fatalf("Replaced=%v Compacted=%v, want %v", res.Replaced, res.Compacted, tc.wantCut)
			}
			if (keepCalls > 0) != tc.wantKeepHit {
				t.Fatalf("keepCalls=%d, wantKeepHit=%v", keepCalls, tc.wantKeepHit)
			}
			if !tc.wantCut {
				if res.Output.Content != tc.content {
					t.Errorf("uncut content was modified")
				}
				return
			}
			if string(stored) != tc.content {
				t.Errorf("CCR stored bytes differ from raw (%d vs %d)", len(stored), len(tc.content))
			}
			if !execwrap.HasMarker(res.Output.Content) {
				t.Errorf("missing CCR marker in cut output")
			}
			if !strings.Contains(res.Output.Content, "downshift retrieve ccrreadonly1") {
				t.Errorf("missing retrieve hint in:\n%s", res.Output.Content)
			}
			// Head and tail kept verbatim within ~2 KiB budgets.
			headSample := strings.Join(rawLines[:10], "")
			tailSample := strings.Join(rawLines[len(rawLines)-10:], "")
			if !strings.HasPrefix(res.Output.Content, headSample) {
				t.Errorf("head lines were altered")
			}
			if !strings.Contains(res.Output.Content, tailSample) {
				t.Errorf("tail lines were altered")
			}
			if res.EmittedBytes >= int64(len(tc.content))/2 {
				t.Errorf("emitted %d B not substantially smaller than raw %d B", res.EmittedBytes, len(tc.content))
			}
		})
	}
}

func TestCompactRead_MiddleCutDoesNotBlockSubsequentSubrangeRead(t *testing.T) {
	t.Setenv(paths.EnvStateDir, t.TempDir())
	filePath := filepath.Join(t.TempDir(), "package-lock.json")
	full := makeLockfile(250) // ~21 KiB > ReadOnlyFileThreshold
	if err := os.WriteFile(filePath, []byte(full), 0o600); err != nil {
		t.Fatal(err)
	}
	sess := strings.Repeat("ef", 32)
	keep := func([]byte) (string, bool) { return "ccrlock00001", true }
	opts := Options{Keep: keep, Balanced: true, Session: sess}

	// First read gets middle-cut (model never saw omitted middle lines).
	inFull := hookport.ToolOutput{
		Category:   core.CategoryRead,
		ToolInput:  readInputJSON(t, map[string]any{"file_path": filePath}),
		Content:    full,
		HasContent: true,
	}
	resFull := Compact(allCaps, inFull, opts)
	if !resFull.Replaced || !strings.Contains(resFull.Output.Content, "omitted") {
		t.Fatalf("expected middle-cut on lockfile, got replaced=%v", resFull.Replaced)
	}

	// Follow-up Read of omitted middle lines (<= 8 KiB) must pass verbatim!
	lines := strings.SplitAfter(full, "\n")
	middle := strings.Join(lines[99:139], "") // lines 100..139 (~3.4 KiB)
	inMid := hookport.ToolOutput{
		Category:   core.CategoryRead,
		ToolInput:  readInputJSON(t, map[string]any{"file_path": filePath, "offset": 100, "limit": 40}),
		Content:    middle,
		HasContent: true,
	}
	resMid := Compact(allCaps, inMid, opts)
	if resMid.Replaced || resMid.Output.Content != middle {
		t.Fatalf("follow-up read of omitted middle lines must not be deduped: replaced=%v content=%q", resMid.Replaced, resMid.Output.Content)
	}

	// Reading a sub-range of the now-seen middle lines SHOULD dedup cleanly.
	sub := strings.Join(lines[109:129], "") // lines 110..129 (~1.7 KiB)
	inSub := hookport.ToolOutput{
		Category:   core.CategoryRead,
		ToolInput:  readInputJSON(t, map[string]any{"file_path": filePath, "offset": 110, "limit": 20}),
		Content:    sub,
		HasContent: true,
	}
	resSub := Compact(allCaps, inSub, opts)
	if !resSub.Replaced || !strings.Contains(resSub.Output.Content, "lines 110-129") || !strings.Contains(resSub.Output.Content, "lines 100-139") {
		t.Fatalf("expected sub-range of seen middle lines to dedup, got replaced=%v content=%q", resSub.Replaced, resSub.Output.Content)
	}
}
