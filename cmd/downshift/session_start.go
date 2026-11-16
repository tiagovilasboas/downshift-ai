// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"fmt"
	"io"
)

// sessionStartContext is the line put into the session's context at start. It is
// a short, factual pointer to the orchestrator's own choice, the only channel
// that reaches the model before it decides how to delegate.
const sessionStartContext = "Downshift is installed. You choose the model and effort of each subagent, by tier (small, mid, frontier), never by model name. " +
	"Mechanical work: start its prompt with [tier=small] (moves down only). With the downshift MCP, call providers before each spawn."

// runClaudeCodeSessionStart answers the SessionStart hook with the context line.
// It reads nothing from its input and fails open: any write error is ignored,
// because a hook that prints nothing leaves the session as it was.
func runClaudeCodeSessionStart(out io.Writer) int {
	payload := map[string]any{
		"hookSpecificOutput": map[string]string{
			"hookEventName":     "SessionStart",
			"additionalContext": sessionStartContext,
		},
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return 0
	}
	_, _ = fmt.Fprintln(out, string(b))
	return 0
}

// runSessionStart answers the SessionStart hook for harnesses with an
// additionalContext wire (Kiro, Codex). Falls back to empty output if unknown.
func runSessionStart(out io.Writer, harness string) int {
	// Kiro and Codex use the same wire format as Claude Code.
	return runClaudeCodeSessionStart(out)
}
