// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package postrun

import (
	"path"
	"strings"
)

// isRetrieveCommand reports whether a shell command invokes `downshift retrieve`
// (bare, via a full/Windows path, or after leading NAME=value env prefixes).
// Its output is already-stored CCR content and must never be re-compacted.
func isRetrieveCommand(command string) bool {
	words := strings.Fields(command)
	for len(words) > 0 && isShellEnvAssignment(words[0]) {
		words = words[1:]
	}
	if len(words) < 2 {
		return false
	}
	prog := strings.Trim(words[0], `"'`)
	prog = strings.ReplaceAll(prog, "\\", "/")
	base := strings.TrimSuffix(strings.ToLower(path.Base(prog)), ".exe")
	return base == "downshift" && words[1] == "retrieve"
}

func isShellEnvAssignment(word string) bool {
	name, _, ok := strings.Cut(word, "=")
	if !ok || name == "" {
		return false
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		isLetter := (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_'
		isDigit := c >= '0' && c <= '9'
		if i == 0 && !isLetter {
			return false
		}
		if !isLetter && !isDigit {
			return false
		}
	}
	return true
}

// isRetrieveTool reports whether toolName names Downshift's MCP retrieve tool
// (for example "mcp__downshift__retrieve" or bare "retrieve").
func isRetrieveTool(toolName string) bool {
	if toolName == "" {
		return false
	}
	lower := strings.ToLower(toolName)
	if lower == "retrieve" {
		return true
	}
	return strings.Contains(lower, "downshift") &&
		(strings.HasSuffix(lower, "__retrieve") ||
			strings.HasSuffix(lower, ":retrieve") ||
			strings.HasSuffix(lower, "/retrieve") ||
			strings.HasSuffix(lower, ".retrieve"))
}
