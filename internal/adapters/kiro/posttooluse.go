// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package kiro

import (
	"encoding/json"

	"github.com/tiagovilasboas/downshift-ai/internal/childend"
	"github.com/tiagovilasboas/downshift-ai/internal/contextopt"
	"github.com/tiagovilasboas/downshift-ai/internal/core"
	"github.com/tiagovilasboas/downshift-ai/internal/mainwork"
	"github.com/tiagovilasboas/downshift-ai/internal/sensor"
	"github.com/tiagovilasboas/downshift-ai/internal/telemetry"
)

// HandlePostToolUse processes one Kiro IDE PostToolUse stdin payload: it
// feeds the context sensor (execute_bash / spawn output), records main-session
// mechanical work (execute_bash, read_file, grep_search, file_search), and
// forwards invoke_sub_agent completions to childend.Record.
//
// Fail-open: any parse or store error returns the neutral {} response with
// linked=false.
func HandlePostToolUse(raw []byte, binaryVersion string, r core.Resolver) (any, string, bool) {
	neutral := struct{}{}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return neutral, "", false
	}
	if ev := firstString(payload, "hook_event_name", "hookEventName"); ev != "" && ev != "PostToolUse" {
		return neutral, "", false
	}

	toolName := firstString(payload, "tool_name", "toolName")
	sessionHash := telemetry.HashSessionID(firstString(payload, "session_id", "sessionId", "sessionID"))

	if sensor.Observes(core.KiroCaps.CategoryOf(toolName)) {
		observeSensor(payload, toolName, sessionHash)
	}
	observeMainWork(payload, toolName, sessionHash)
	toolInput, _ := payload["tool_input"].(map[string]any)
	telemetry.RecordSkillsFromToolCall(harnessID, sessionHash, binaryVersion, toolName, toolInput)

	ce, ok := ParseChildEnd(raw)
	if !ok {
		return neutral, "", false
	}
	res := childend.Record(ce, binaryVersion, r)
	return neutral, res.Note, res.Linked
}

func observeSensor(payload map[string]any, toolName, sessionHash string) {
	if sessionHash == "" || !nativeContextObserveOn() {
		return
	}
	st, err := sensor.DefaultStore()
	if err != nil {
		return
	}
	if toolName == "" {
		toolName = "tool"
	}
	outputBytes, exitCode, isError := extractToolOutput(payload["tool_response"])
	_ = st.RecordToolResult(sessionHash, harnessID, toolName, outputBytes, exitCode, isError)
}

func observeMainWork(payload map[string]any, toolName, sessionHash string) {
	if toolName == "" {
		return
	}
	if _, listed := core.KiroCaps.MainToolCategories[toolName]; !listed {
		return
	}
	if firstString(payload, "agent_id", "agentId") != "" {
		return
	}
	mainwork.Observe(mainwork.Call{
		Harness:     harnessID,
		Tool:        toolName,
		Session:     sessionHash,
		ResultBytes: stringBytes(payload["tool_response"]),
	}, core.KiroCaps)
}

func nativeContextObserveOn() bool {
	cfg, err := contextopt.LoadConfig()
	if err != nil || !cfg.Enabled {
		return false
	}
	return cfg.Provider == "" || cfg.Provider == "native"
}

func extractToolOutput(rawResp any) ([]byte, int, bool) {
	switch v := rawResp.(type) {
	case string:
		return []byte(v), -1, false
	case map[string]any:
		isError := false
		if status, ok := v["status"].(string); ok && (status == "error" || status == "failed") {
			isError = true
		}
		if flag, ok := v["is_error"].(bool); ok && flag {
			isError = true
		}
		if flag, ok := v["isError"].(bool); ok && flag {
			isError = true
		}
		exitCode, _ := exitCodeFrom(v)

		var b []byte
		if out, ok := v["stdout"].(string); ok {
			b = append(b, out...)
		}
		if errOut, ok := v["stderr"].(string); ok && errOut != "" {
			if len(b) > 0 && b[len(b)-1] != '\n' {
				b = append(b, '\n')
			}
			b = append(b, errOut...)
		}
		if b != nil {
			return b, exitCode, isError
		}
		if c, ok := v["content"].(string); ok {
			return []byte(c), exitCode, isError
		}
		if out, ok := v["output"].(string); ok {
			return []byte(out), exitCode, isError
		}
		return nil, exitCode, isError
	}
	return nil, -1, false
}

func stringBytes(v any) int {
	n := 0
	switch t := v.(type) {
	case string:
		return len(t)
	case []any:
		for _, e := range t {
			n += stringBytes(e)
		}
	case map[string]any:
		for _, e := range t {
			n += stringBytes(e)
		}
	}
	return n
}

func exitCodeFrom(resp map[string]any) (int, bool) {
	for _, key := range []string{"exit_code", "exitCode"} {
		n, ok := resp[key].(float64)
		if !ok || n < 0 || n > 255 || n != float64(int(n)) {
			continue
		}
		return int(n), true
	}
	return -1, false
}

func firstString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if s, ok := m[k].(string); ok && s != "" {
			return s
		}
	}
	return ""
}
