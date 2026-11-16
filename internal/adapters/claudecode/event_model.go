// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package claudecode

import (
	"encoding/json"

	"github.com/tiagovilasboas/downshift-ai/internal/core"
	"github.com/tiagovilasboas/downshift-ai/internal/hookutil"
	"github.com/tiagovilasboas/downshift-ai/internal/telemetry"
)

// RequestedReasoningEffort is the effort the child would run at without a
// rewrite, in Claude Code's order: the Agent call's `effort`, the agent
// definition's `effort:`, then the session's. The Downshift mod's
// `reasoning_effort` counts as a per-call value. With neither, the child
// inherits the session effort, which the hook does not see: "inherit".
func (ev Event) RequestedReasoningEffort() string {
	var ti map[string]any
	if json.Unmarshal(ev.ToolInput, &ti) != nil {
		return ""
	}
	if e := hookutil.StringField(ti, "effort"); e != "" {
		return e
	}
	if e := hookutil.StringField(ti, "reasoning_effort"); e != "" {
		return e
	}
	if def := lookupAgentDefinition(ev.Cwd, hookutil.StringField(ti, "subagent_type")); def.Effort != "" {
		return def.Effort
	}
	return telemetry.EffortInherit
}

// RequestedModel returns the model received before Downshift emitted a rewrite.
func (ev Event) RequestedModel() string {
	var ti map[string]any
	if json.Unmarshal(ev.ToolInput, &ti) != nil {
		return ev.Model
	}
	return ev.currentModel(ti)
}

// ParentModelIdentifier is the model the parent session ran on at this spawn,
// whether or not the call names its own model, and where it came from: "mod"
// (the mod's agent.spawn event) or "transcript" (the assistant line that issued
// this call, a bounded read). Empty when neither knows it. Data for telemetry
// only; the routing baseline is resolveBaseline.
func (ev Event) ParentModelIdentifier() (string, string) {
	if ev.ParentModel != "" {
		return ev.ParentModel, "mod"
	}
	if !core.ClaudeCodeCaps.TranscriptNamesSpawner(ev.SpawnedFromSubagent()) {
		return "", ""
	}
	if m := parentModelAtSpawn(ev.TranscriptPath, ev.ToolUseID); m != "" {
		return m, "transcript"
	}
	return "", ""
}

// TaskText returns the task content used for local, prompt-free feature
// extraction by the shared engineering loop.
func (ev Event) TaskText() string {
	var ti map[string]any
	if json.Unmarshal(ev.ToolInput, &ti) != nil {
		return ""
	}
	return hookutil.TaskText(ti, "prompt", "description")
}
