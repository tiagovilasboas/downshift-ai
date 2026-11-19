// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package kirocrew

import (
	"encoding/json"
	"strings"

	"github.com/tiagovilasboas/downshift-ai/internal/core"
	"github.com/tiagovilasboas/downshift-ai/internal/hookutil"
	"github.com/tiagovilasboas/downshift-ai/internal/quota"
	"github.com/tiagovilasboas/downshift-ai/internal/telemetry"
)

// spawnToolNames are the KiroCrew tools that spawn a subagent. Only these are
// routed; a tool that merely carries model and prompt fields is not a spawn.
var spawnToolNames = map[string]bool{
	"spawn_run":        true,
	"spawn_sub_agents": true,
}

// Event is the JSON a harness sends on stdin for a hook.
// Field names follow KiroCrew's documented hook event schema; the same
// structure is used for preToolUse and postToolUse events.
type Event struct {
	HookEventName string `json:"hook_event_name"`
	// Cwd is the working directory, used to find a project policy file.
	Cwd string `json:"cwd,omitempty"`
	// SessionID is the opaque session identifier supplied by KiroCrew. Enables
	// PostToolUse linkage by session.
	SessionID         string              `json:"session_id,omitempty"`
	CorrelationID     string              `json:"correlation_id,omitempty"`
	ToolName          string              `json:"tool_name"`
	ToolInput         json.RawMessage     `json:"tool_input"`
	ToolResponse      json.RawMessage     `json:"tool_response,omitempty"` // postToolUse only
	ModelMetadata     []core.SessionModel `json:"model_metadata,omitempty"`
	SessionModels     *[]string           `json:"session_models,omitempty"`
	AvailableModels   *[]string           `json:"available_models,omitempty"`
	IncludedModels    *[]string           `json:"included_models,omitempty"`
	UnavailableModels *[]string           `json:"unavailable_models,omitempty"`
	UsageQuota        *quota.Snapshot     `json:"usage_quota,omitempty"`
}

// ProjectDir returns the working directory the harness reported, if any.
func (ev Event) ProjectDir() string { return ev.Cwd }

// SessionIdentifier returns the session ID for telemetry correlation.
func (ev Event) SessionIdentifier() string { return ev.SessionID }

// CorrelationIdentifier returns the optional opaque ID supplied by a caller.
func (ev Event) CorrelationIdentifier() string { return ev.CorrelationID }

// RequestedModel exposes the payload baseline to the shared telemetry runner.
func (ev Event) RequestedModel() string {
	var ti map[string]any
	if json.Unmarshal(ev.ToolInput, &ti) != nil {
		return ""
	}
	return hookutil.StringField(ti, "model")
}

// TaskText returns the subagent task content used for prompt-free feature
// extraction. Common field names across harnesses: "task", "prompt",
// "description". The first non-empty value wins.
func (ev Event) TaskText() string {
	var ti map[string]any
	if json.Unmarshal(ev.ToolInput, &ti) != nil {
		return ""
	}
	return hookutil.TaskText(ti, "task", "prompt", "description")
}

// UsedSkills extracts privacy-safe skill labels when the tool call invokes a
// Skill tool or reads .../skills/<slug>/SKILL.md.
func (ev Event) UsedSkills() []string {
	var ti map[string]any
	if json.Unmarshal(ev.ToolInput, &ti) != nil {
		return nil
	}
	return telemetry.SkillsFromToolCall(canonicalToolName(ev.ToolName), ti)
}

// canonicalToolName drops the MCP server prefix KiroCrew adds when the agent
// reaches a tool through a server: "@kirocrew-core/spawn_run" is spawn_run.
// Any other name is returned unchanged.
func canonicalToolName(name string) string {
	if !strings.HasPrefix(name, "@") {
		return name
	}
	if i := strings.LastIndex(name, "/"); i >= 0 {
		return name[i+1:]
	}
	return name
}

// isSubagentSpawn reports whether the call is a subagent spawn that the router
// should classify: the tool must be a spawn tool in spawnToolNames, and the
// tool_input must carry a model field (something to route) and a
// task/prompt/description field (something to classify).
func isSubagentSpawn(toolName string, ti map[string]any) bool {
	if !spawnToolNames[toolName] {
		return false
	}
	model := hookutil.StringField(ti, "model")
	if model == "" {
		return false
	}
	task := hookutil.TaskText(ti, "task", "prompt", "description")
	return task != ""
}
