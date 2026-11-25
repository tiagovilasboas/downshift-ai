// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package claudecode_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/adapters/claudecode"
	"github.com/tiagovilasboas/downshift-ai/internal/paths"
)

const toolEvidence = "ok  github.com/acme/a  0.1s\nok  github.com/acme/b  0.2s\nKEEP_TOOL_EVIDENCE_LINE\n"

func TestPostToolUse_CompressExitFollowsNativeFlag(t *testing.T) {
	t.Run("off", func(t *testing.T) {
		dir := isolateDownshiftState(t)
		writeContextConfig(t, dir, false)
		out := postTool(t, allOKToolOutput(), 0)
		assertToolEvidenceKept(t, out)
		if _, err := os.Stat(filepath.Join(dir, "context-compactions.jsonl")); !os.IsNotExist(err) {
			t.Fatal("CompressExit was recorded while the native provider flag was off")
		}
	})

	t.Run("on observes exit 0 without rewriting the tool", func(t *testing.T) {
		dir := isolateDownshiftState(t)
		writeContextConfig(t, dir, true)
		out := postTool(t, allOKToolOutput(), 0)
		assertToolEvidenceKept(t, out)
		rec := readCompaction(t, dir)
		if rec.Harness != "claude-code" || !rec.Applied || rec.ReducedBytes <= 0 || rec.OutputBytes != int64(len(allOKToolOutput())) {
			t.Fatalf("record = %+v, want an observe-mode reduction for exit 0", rec)
		}
		sensorRaw, err := os.ReadFile(filepath.Join(dir, "context-sensor.json"))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(sensorRaw), "github.com/acme/a") {
			t.Fatal("sensor stored tool output")
		}
	})

	t.Run("on keeps a non-zero exit uncompressed", func(t *testing.T) {
		dir := isolateDownshiftState(t)
		writeContextConfig(t, dir, true)
		out := postTool(t, allOKToolOutput(), 2)
		assertToolEvidenceKept(t, out)
		rec := readCompaction(t, dir)
		if rec.Applied || rec.ReducedBytes != 0 || rec.OutputBytes != int64(len(allOKToolOutput())) {
			t.Fatalf("record = %+v, want the real exit code to preserve bytes", rec)
		}
	})

	t.Run("on does not drop a line the hook cannot rewrite", func(t *testing.T) {
		dir := isolateDownshiftState(t)
		writeContextConfig(t, dir, true)
		out := postTool(t, toolEvidence, 0)
		assertToolEvidenceKept(t, out)
		encoded, err := json.Marshal(out)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(encoded), "Go test: PASS") {
			t.Fatalf("hook replaced tool evidence with a summary: %s", encoded)
		}
		if strings.Contains(string(encoded), "content") && !strings.Contains(string(encoded), "KEEP_TOOL_EVIDENCE_LINE") {
			t.Fatalf("rewritten tool result dropped evidence: %s", encoded)
		}
	})
}

func isolateDownshiftState(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv(paths.EnvStateDir, dir)
	t.Setenv("DOWNSHIFT_EVENT_LOG", filepath.Join(dir, "events.jsonl"))
	return dir
}

func writeContextConfig(t *testing.T, dir string, enabled bool) {
	t.Helper()
	provider := ""
	if enabled {
		provider = "native"
	}
	body, err := json.Marshal(map[string]any{"enabled": enabled, "provider": provider})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "context-optimization.json"), append(body, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
}

func allOKToolOutput() string {
	var b strings.Builder
	for _, p := range []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"} {
		b.WriteString("ok  github.com/acme/" + p + "  0.2s\n")
	}
	return b.String()
}

func postTool(t *testing.T, content string, exitCode int) claudecode.Output {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"session_id": "sess-qa-compress",
		"tool_name":  "Bash",
		"tool_response": map[string]any{
			"content":   content,
			"exit_code": exitCode,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	out, _, _ := claudecode.HandlePostToolUse(payload, "test", cat)
	return out
}

func assertToolEvidenceKept(t *testing.T, out claudecode.Output) {
	t.Helper()
	encoded, err := json.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "Go test: PASS") {
		t.Fatalf("hook rewrote the tool result: %s", encoded)
	}
}

type compactionLine struct {
	Harness      string `json:"harness"`
	Tool         string `json:"tool"`
	OutputBytes  int64  `json:"output_bytes"`
	ReducedBytes int64  `json:"reduced_bytes"`
	Applied      bool   `json:"applied"`
	Error        bool   `json:"error"`
}

func TestPostToolUse_SensorInfersCleanExitAndMCPSavings(t *testing.T) {
	genericText := strings.Repeat("INFO service started successfully on port 8080\n", 12)

	t.Run("bash without exit_code infers exit 0 and records generic savings", func(t *testing.T) {
		dir := isolateDownshiftState(t)
		writeContextConfig(t, dir, true)
		payload, err := json.Marshal(map[string]any{
			"session_id": "sess-bash-no-exit",
			"tool_name":  "Bash",
			"tool_response": map[string]any{
				"stdout":      genericText,
				"stderr":      "",
				"interrupted": false,
				"isImage":     false,
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		claudecode.HandlePostToolUse(payload, "test", cat)
		rec := readCompaction(t, dir)
		if rec.Tool != "Bash" || !rec.Applied || rec.ReducedBytes <= 0 || rec.Error || rec.OutputBytes != int64(len(genericText)) {
			t.Fatalf("record = %+v, want applied=true reduced_bytes>0 error=false", rec)
		}
	})

	t.Run("bash interrupted without exit_code preserves bytes as error", func(t *testing.T) {
		dir := isolateDownshiftState(t)
		writeContextConfig(t, dir, true)
		payload, err := json.Marshal(map[string]any{
			"session_id": "sess-bash-interrupted",
			"tool_name":  "Bash",
			"tool_response": map[string]any{
				"stdout":      genericText,
				"stderr":      "",
				"interrupted": true,
				"isImage":     false,
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		claudecode.HandlePostToolUse(payload, "test", cat)
		rec := readCompaction(t, dir)
		if rec.Applied || rec.ReducedBytes != 0 || !rec.Error {
			t.Fatalf("record = %+v, want applied=false reduced_bytes=0 error=true", rec)
		}
	})

	t.Run("mcp array response and long tool name record output and savings", func(t *testing.T) {
		dir := isolateDownshiftState(t)
		writeContextConfig(t, dir, true)
		const longTool = "mcp__plugin_playwright_playwright__browser_take_screenshot"
		payload, err := json.Marshal(map[string]any{
			"session_id": "sess-mcp-array",
			"tool_name":  longTool,
			"tool_response": []any{
				map[string]any{"type": "text", "text": genericText},
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		claudecode.HandlePostToolUse(payload, "test", cat)
		rec := readCompaction(t, dir)
		if rec.Tool != longTool || rec.OutputBytes != int64(len(genericText)) || !rec.Applied || rec.ReducedBytes <= 0 {
			t.Fatalf("record = %+v, want tool=%q output=%d applied=true reduced>0", rec, longTool, len(genericText))
		}
	})

	t.Run("mcp string response records output_bytes", func(t *testing.T) {
		dir := isolateDownshiftState(t)
		writeContextConfig(t, dir, true)
		const tool = "mcp__datadog__list_datadog_skills"
		payload, err := json.Marshal(map[string]any{
			"session_id":    "sess-mcp-string",
			"tool_name":     tool,
			"tool_response": genericText,
		})
		if err != nil {
			t.Fatal(err)
		}
		claudecode.HandlePostToolUse(payload, "test", cat)
		rec := readCompaction(t, dir)
		if rec.Tool != tool || rec.OutputBytes != int64(len(genericText)) || !rec.Applied || rec.ReducedBytes <= 0 {
			t.Fatalf("record = %+v, want tool=%q output=%d applied=true reduced>0", rec, tool, len(genericText))
		}
	})
}

func readCompaction(t *testing.T, dir string) compactionLine {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, "context-compactions.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var rec compactionLine
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(raw))), &rec); err != nil {
		t.Fatal(err)
	}
	return rec
}
