// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package claudecode

import (
	"encoding/json"
	"strings"

	"github.com/tiagovilasboas/downshift-ai/internal/core"
	"github.com/tiagovilasboas/downshift-ai/internal/hookport"
)

// Claude Code's PostToolUse payload carries the finished tool's result in
// tool_response, in that tool's own output shape, and the hook may answer
// hookSpecificOutput.updatedToolOutput (built-in tools) or
// hookSpecificOutput.updatedMCPToolOutput (mcp__* tools) with a value of the
// same shape (hooks reference, "PostToolUse decision control"). Nothing here
// decides what to cut: internal/postrun does.

const (
	rawShapeKey    = "__downshiftRawShape"
	rawShapeString = "string"
	rawShapeArray  = "array"
	rawShapeJSON   = "json"
)

// ParseToolOutput reads a PostToolUse payload into the shared ToolOutput.
func ParseToolOutput(raw []byte) (hookport.ToolOutput, bool) {
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return hookport.ToolOutput{}, false
	}
	if ev, ok := payload["hook_event_name"].(string); ok && ev != "PostToolUse" {
		return hookport.ToolOutput{}, false
	}
	toolName := stringFieldAny(payload, "tool_name", "toolName")
	out := hookport.ToolOutput{
		Category:    core.ClaudeCodeCaps.CategoryOf(toolName),
		VerboseKind: verboseKindOf(toolName),
		Session:     stringFieldAny(payload, "session_id", "sessionId"),
		ExitCode:    -1,
	}
	if out.Session == "" {
		out.Session = sessionFromTranscript(stringFieldAny(payload, "transcript_path"))
	}
	if input, ok := payload["tool_input"].(map[string]any); ok {
		out.Command, _ = input["command"].(string)
	}
	resp, ok := payload["tool_response"].(map[string]any)
	if !ok {
		return parseNonMapToolResponse(out, payload["tool_response"]), true
	}
	out.Native = resp
	out.Stdout, out.HasStdout = resp["stdout"].(string)
	out.Stderr, out.HasStderr = resp["stderr"].(string)
	out.Content, out.HasContent = resp["content"].(string)
	if !out.HasContent {
		if file, ok := resp["file"].(map[string]any); ok {
			out.Content, out.HasContent = file["content"].(string)
		}
	}
	out.Paths, out.HasPaths, out.Opaque = stringList(resp["filenames"])
	if code, ok := exitCodeField(resp); ok {
		out.ExitCode = code
	}
	if b, _ := resp["interrupted"].(bool); b {
		out.Opaque = true
	}
	if b, _ := resp["isImage"].(bool); b {
		out.Opaque = true
	}
	if b, _ := resp["isError"].(bool); b {
		out.ExitCode = 1
	}
	if out.VerboseKind != "" && !out.HasContent && !out.Opaque {
		out = parseVerboseMapFields(out, resp)
	}
	return out, true
}

func verboseKindOf(tool string) string {
	switch {
	case strings.HasPrefix(tool, "mcp__"):
		return "mcp"
	case tool == "WebFetch":
		return "fetch"
	default:
		return ""
	}
}

func parseNonMapToolResponse(out hookport.ToolOutput, raw any) hookport.ToolOutput {
	if out.VerboseKind == "" {
		out.Opaque = true
		return out
	}
	switch v := raw.(type) {
	case string:
		out.Content = v
		out.HasContent = true
		out.Native = map[string]any{rawShapeKey: rawShapeString}
	case []any:
		text, opaque := extractContentBlocks(v)
		if opaque {
			out.Opaque = true
			return out
		}
		out.Content = text
		out.HasContent = true
		out.Native = map[string]any{rawShapeKey: rawShapeArray}
	default:
		out.Opaque = true
	}
	return out
}

func parseVerboseMapFields(out hookport.ToolOutput, resp map[string]any) hookport.ToolOutput {
	if blocks, ok := resp["content"].([]any); ok {
		text, opaque := extractContentBlocks(blocks)
		if opaque {
			out.Opaque = true
			return out
		}
		out.Content, out.HasContent = text, true
		return out
	}
	if t, ok := resp["text"].(string); ok {
		out.Content, out.HasContent = t, true
		return out
	}
	if r, ok := resp["result"].(string); ok {
		out.Content, out.HasContent = r, true
		return out
	}
	if len(resp) > 0 {
		if b, err := json.Marshal(resp); err == nil {
			out.Content, out.HasContent = string(b), true
			out.Native = map[string]any{rawShapeKey: rawShapeJSON}
		}
	}
	return out
}

func extractContentBlocks(blocks []any) (string, bool) {
	var parts []string
	for _, b := range blocks {
		m, ok := b.(map[string]any)
		if !ok {
			return "", true
		}
		if typ, _ := m["type"].(string); typ == "image" {
			return "", true
		}
		text, ok := m["text"].(string)
		if !ok {
			return "", true
		}
		parts = append(parts, text)
	}
	return strings.Join(parts, "\n"), false
}

// stringList reads a JSON array of strings. A missing field is (nil, false,
// false); an array holding anything but strings is opaque.
func stringList(v any) (list []string, has, opaque bool) {
	arr, ok := v.([]any)
	if !ok {
		return nil, false, v != nil
	}
	list = make([]string, 0, len(arr))
	for _, e := range arr {
		s, ok := e.(string)
		if !ok {
			return nil, false, true
		}
		list = append(list, s)
	}
	return list, true, false
}

// ReplaceToolOutput answers PostToolUse with updatedToolOutput (built-in
// tools) or updatedMCPToolOutput (mcp__* tools), preserving the tool's own
// result shape. prev is kept (a systemMessage, for one). A result that cannot
// be encoded leaves prev unchanged.
func ReplaceToolOutput(prev any, out hookport.ToolOutput) any {
	if out.Native == nil {
		return prev
	}
	switch shape, _ := out.Native[rawShapeKey].(string); shape {
	case rawShapeString, rawShapeJSON:
		encoded, err := json.Marshal(out.Content)
		if err != nil {
			return prev
		}
		return setUpdatedToolOutput(prev, out.VerboseKind, encoded)
	case rawShapeArray:
		encoded, err := json.Marshal([]any{map[string]any{"type": "text", "text": out.Content}})
		if err != nil {
			return prev
		}
		return setUpdatedToolOutput(prev, out.VerboseKind, encoded)
	}
	updated := make(map[string]any, len(out.Native))
	for k, v := range out.Native {
		updated[k] = v
	}
	if out.HasStdout {
		updated["stdout"] = out.Stdout
	}
	if out.HasStderr {
		updated["stderr"] = out.Stderr
	}
	if out.HasContent {
		replaceMapContent(updated, out.Content)
	}
	if out.HasPaths {
		updated["filenames"] = out.Paths
	}
	encoded, err := json.Marshal(updated)
	if err != nil {
		return prev
	}
	return setUpdatedToolOutput(prev, out.VerboseKind, encoded)
}

func replaceMapContent(updated map[string]any, content string) {
	if _, ok := updated["content"].(string); ok {
		updated["content"] = content
		return
	}
	if _, ok := updated["content"].([]any); ok {
		updated["content"] = []any{map[string]any{"type": "text", "text": content}}
		return
	}
	if file, ok := updated["file"].(map[string]any); ok {
		fileCopy := make(map[string]any, len(file))
		for k, v := range file {
			fileCopy[k] = v
		}
		fileCopy["content"] = content
		updated["file"] = fileCopy
		return
	}
	if _, ok := updated["text"].(string); ok {
		updated["text"] = content
		return
	}
	if _, ok := updated["result"].(string); ok {
		updated["result"] = content
		return
	}
	updated["content"] = content
}

func setUpdatedToolOutput(prev any, verboseKind string, encoded json.RawMessage) any {
	answer, _ := prev.(Output)
	hso := HookSpecificOutput{HookEventName: "PostToolUse"}
	if answer.HookSpecificOutput != nil {
		hso = *answer.HookSpecificOutput
	}
	if verboseKind == "mcp" {
		hso.UpdatedMCPToolOutput = encoded
	} else {
		hso.UpdatedToolOutput = encoded
	}
	answer.HookSpecificOutput = &hso
	return answer
}
