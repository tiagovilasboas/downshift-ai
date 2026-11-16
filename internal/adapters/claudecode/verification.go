// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package claudecode

import (
	"bytes"
	"encoding/json"

	"github.com/tiagovilasboas/downshift-ai/internal/lazyre"
)

// Verification values recorded on a usage event.
const (
	verificationPassed = "passed"
	verificationFailed = "failed"
	verificationNone   = "none"
)

// verifyCommand matches the shell commands that count as a subagent checking
// its own work: test runners, linters and type checkers. It is deliberately a
// short allowlist; an unrecognised command counts as no verification.
var verifyCommand = lazyre.New(`(?:^|[\s;&|(])(?:go\s+(?:test|vet)|pytest|python3?\s+-m\s+(?:pytest|unittest)|npm\s+(?:run\s+)?(?:test|lint)|pnpm\s+(?:run\s+)?(?:test|lint)|yarn\s+(?:test|lint)|vitest|jest|cargo\s+(?:test|clippy)|tsc|eslint|ruff|golangci-lint|make\s+(?:test|lint|check))(?:\s|$)`)

// verificationTracker tracks whether a subagent verified its own work across
// transcript lines. It pairs each recognised check command (a Bash tool_use)
// with the matching tool_result and reads only the is_error flag; the command
// text and the output are never kept. The last check wins, since a subagent
// that fixes a failure and reruns the check ends on its final state.
type verificationTracker struct {
	pending map[string]bool // tool_use id of a check awaiting its result
	result  string
}

func newVerificationTracker() verificationTracker {
	return verificationTracker{
		pending: map[string]bool{},
		result:  verificationNone,
	}
}

func (v *verificationTracker) observe(content json.RawMessage) {
	trimmed := bytes.TrimSpace(content)
	if len(trimmed) == 0 || trimmed[0] != '[' {
		return
	}
	var blocks []struct {
		Type      string `json:"type"`
		ID        string `json:"id"`
		Name      string `json:"name"`
		ToolUseID string `json:"tool_use_id"`
		IsError   bool   `json:"is_error"`
		Input     struct {
			Command string `json:"command"`
		} `json:"input"`
	}
	if json.Unmarshal(trimmed, &blocks) != nil {
		return
	}
	for _, b := range blocks {
		switch b.Type {
		case "tool_use":
			if b.Name == "Bash" && b.ID != "" && verifyCommand.MatchString(b.Input.Command) {
				v.pending[b.ID] = true
			}
		case "tool_result":
			if v.pending[b.ToolUseID] {
				delete(v.pending, b.ToolUseID)
				if b.IsError {
					v.result = verificationFailed
				} else {
					v.result = verificationPassed
				}
			}
		}
	}
}

func (v *verificationTracker) outcome() string {
	if v.result == "" {
		return verificationNone
	}
	return v.result
}
