// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package kiro

import (
	"encoding/json"

	"github.com/tiagovilasboas/downshift-ai/internal/shellwrap"
)

const shellToolName = "execute_bash"

// ShellCommand returns tool_input.command and true when this call is Kiro's
// execute_bash tool. When tool_input is missing or malformed it returns "", true
// so the shared runner treats it as a shell call and fails open without
// routing it as a subagent spawn.
func (ev Event) ShellCommand() (string, bool) {
	if ev.ToolName != shellToolName {
		return "", false
	}
	var ti map[string]any
	if json.Unmarshal(ev.ToolInput, &ti) != nil {
		return "", true
	}
	command, _ := ti["command"].(string)
	return command, true
}

// ShellBlocked is false: Kiro IDE has no readable shell allow/deny file and
// its CanAdviseShellWrap / CanRewriteShellCommand capabilities stay disabled.
func (ev Event) ShellBlocked(_ string) bool { return false }

// AdviseMessage formats the stderr retry suggestion using the shared wording.
func (ev Event) AdviseMessage(wrappedCommand string) string {
	return shellwrap.AdviseText(wrappedCommand)
}
