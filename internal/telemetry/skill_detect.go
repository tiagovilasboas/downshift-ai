// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"strings"

	"github.com/tiagovilasboas/downshift-ai/internal/lazyre"
)

var promptSkillTokenPattern = lazyre.New(`[/$]((?:(?:downshift|superpowers):)?[a-zA-Z0-9][a-zA-Z0-9-]{0,47})`)

var skillReadTools = map[string]bool{
	"read":      true,
	"read_file": true,
	"fs_read":   true,
	"view_file": true,
}

// SkillsFromToolCall extracts privacy-safe skill labels from a tool invocation
// across harnesses: direct Skill tool calls ("Skill" / "skill") and file-read
// tools opening .../skills/<slug>/SKILL.md. Every returned label is bounded
// through SkillNameLabel and deduplicated per call.
func SkillsFromToolCall(toolName string, toolInput map[string]any) []string {
	if toolName == "" || len(toolInput) == 0 {
		return nil
	}
	if strings.EqualFold(toolName, "Skill") {
		raw := firstStringField(toolInput, "skill", "name")
		if label := SkillNameLabel(raw); label != "" {
			return []string{label}
		}
		return nil
	}
	if !skillReadTools[strings.ToLower(toolName)] {
		return nil
	}
	var out []string
	seen := map[string]bool{}
	for _, p := range toolInputPaths(toolInput) {
		slug := skillSlugFromPath(p)
		if slug == "" {
			continue
		}
		label := SkillNameLabel(slug)
		if label == "" || seen[label] {
			continue
		}
		seen[label] = true
		out = append(out, label)
	}
	return out
}

// SkillsFromPrompt extracts allowlisted public skill labels (/ponytail,
// $ponytail, /downshift-*, $downshift-*, /superpowers:*) from user prompt
// text. Unlike tool calls, unrecognized /cmd or $VAR tokens never emit "other"
// so ordinary slash commands, paths, and shell variables are ignored.
func SkillsFromPrompt(prompt string) []string {
	if prompt == "" {
		return nil
	}
	matches := promptSkillTokenPattern.Get().FindAllStringSubmatchIndex(prompt, -1)
	if len(matches) == 0 {
		return nil
	}
	var out []string
	seen := map[string]bool{}
	for _, m := range matches {
		start, end, capStart, capEnd := m[0], m[1], m[2], m[3]
		if start > 0 && !isPromptLeftBoundary(prompt[start-1]) {
			continue
		}
		if end < len(prompt) && !isPromptRightBoundary(prompt[end]) {
			continue
		}
		label := SkillNameLabel(strings.ToLower(prompt[capStart:capEnd]))
		if label == "" || label == "other" || seen[label] {
			continue
		}
		seen[label] = true
		out = append(out, label)
	}
	return out
}

// RecordSkillsFromToolCall records one SkillUsed event per detected skill in a
// tool call. sessionHash must already be hashed via HashSessionID.
func RecordSkillsFromToolCall(harness, sessionHash, binaryVersion, toolName string, toolInput map[string]any) {
	for _, skill := range SkillsFromToolCall(toolName, toolInput) {
		Record(SkillUsed(harness, sessionHash, skill, binaryVersion))
	}
}

// RecordSkillsFromPrompt records one SkillUsed event per allowlisted skill
// mentioned in a prompt. sessionHash must already be hashed via HashSessionID.
func RecordSkillsFromPrompt(harness, sessionHash, binaryVersion, prompt string) {
	for _, skill := range SkillsFromPrompt(prompt) {
		Record(SkillUsed(harness, sessionHash, skill, binaryVersion))
	}
}

func toolInputPaths(ti map[string]any) []string {
	var paths []string
	for _, key := range []string{"file_path", "filePath", "path", "target_file", "AbsolutePath"} {
		if s, ok := ti[key].(string); ok && s != "" {
			paths = append(paths, s)
		}
	}
	if list, ok := ti["paths"].([]any); ok {
		for _, item := range list {
			if s, ok := item.(string); ok && s != "" {
				paths = append(paths, s)
			}
		}
	}
	if ops, ok := ti["operations"].([]any); ok {
		for _, raw := range ops {
			if op, ok := raw.(map[string]any); ok {
				if s := firstStringField(op, "path", "file_path", "filePath"); s != "" {
					paths = append(paths, s)
				}
			}
		}
	}
	return paths
}

func skillSlugFromPath(path string) string {
	norm := strings.ReplaceAll(strings.TrimSpace(path), `\`, "/")
	norm = strings.TrimRight(norm, "/")
	parts := strings.Split(norm, "/")
	if len(parts) < 3 {
		return ""
	}
	if !strings.EqualFold(parts[len(parts)-1], "SKILL.md") {
		return ""
	}
	if parts[len(parts)-3] != "skills" || parts[len(parts)-2] == "" {
		return ""
	}
	return parts[len(parts)-2]
}

func firstStringField(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if s, ok := m[k].(string); ok && s != "" {
			return s
		}
	}
	return ""
}

func isPromptLeftBoundary(b byte) bool {
	switch b {
	case ' ', '\t', '\n', '\r', '(', '[', '{', '"', '\'':
		return true
	}
	return false
}

func isPromptRightBoundary(b byte) bool {
	switch b {
	case ' ', '\t', '\n', '\r', ',', '.', ':', ';', '!', '?', ')', ']', '}', '"', '\'':
		return true
	}
	return false
}
