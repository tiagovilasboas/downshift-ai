// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package codex

import (
	"encoding/json"
	"strings"

	"github.com/tiagovilasboas/downshift-ai/internal/core"
	"github.com/tiagovilasboas/downshift-ai/internal/hookutil"
	"github.com/tiagovilasboas/downshift-ai/internal/quota"
)

// Event is the JSON Codex sends on stdin for a PreToolUse hook.
type Event struct {
	HookEventName string `json:"hook_event_name"`
	// Cwd is the working directory, used to find a project policy file.
	Cwd           string `json:"cwd,omitempty"`
	CorrelationID string `json:"correlation_id,omitempty"`
	// ToolUseID is the harness id of this tool call (documented in its hook payload).
	ToolUseID string          `json:"tool_use_id,omitempty"`
	SessionID string          `json:"session_id"`
	ToolName  string          `json:"tool_name"`
	ToolInput json.RawMessage `json:"tool_input"`
	Model     string          `json:"model"`
	// Optional allowlist. Nil means the payload did not include one.
	// Codex PreToolUse does not send this field today.
	ModelMetadata   []core.SessionModel `json:"model_metadata,omitempty"`
	SessionModels   *[]string           `json:"session_models,omitempty"`
	AvailableModels *[]string           `json:"available_models,omitempty"`
	// IncludedModels and UnavailableModels are optional quota marks.
	// Absent marks supply no credit evidence. Present marks apply to this call.
	IncludedModels    *[]string       `json:"included_models,omitempty"`
	UnavailableModels *[]string       `json:"unavailable_models,omitempty"`
	UsageQuota        *quota.Snapshot `json:"usage_quota,omitempty"`
	TranscriptPath    string          `json:"transcript_path,omitempty"`
}

// ToolUseIdentifier returns the harness id of this tool call, if any.
func (ev Event) ToolUseIdentifier() string { return ev.ToolUseID }

// SessionIdentifier exposes Codex's stable session ID to the shared hook
// runner without coupling core routing decisions to harness metadata.
func (ev Event) SessionIdentifier() string { return ev.SessionID }

// ProjectDir returns the working directory the harness reported, if any.
func (ev Event) ProjectDir() string { return ev.Cwd }

// CorrelationIdentifier returns the optional opaque ID supplied by a caller.
func (ev Event) CorrelationIdentifier() string { return ev.CorrelationID }

// RequestedReasoningEffort reads an optional hook input field for telemetry.
func (ev Event) RequestedReasoningEffort() string {
	var ti map[string]any
	if json.Unmarshal(ev.ToolInput, &ti) != nil {
		return ""
	}
	return hookutil.StringField(ti, "reasoning_effort")
}

// RequestedModel returns the original model field, never a routing
// recommendation. Tool input takes precedence because it is what the child
// request actually carried before this hook rewrote it.
func (ev Event) RequestedModel() string {
	var ti map[string]any
	if json.Unmarshal(ev.ToolInput, &ti) != nil {
		return ev.Model
	}
	return ev.currentModel(ti)
}

// currentModel is the model the child would run on without a rewrite: the
// spawn "model", the payload "model", then the parent session model.
func (ev Event) currentModel(ti map[string]any) string {
	if model := hookutil.StringField(ti, "model"); model != "" {
		return model
	}
	if ev.Model != "" {
		return ev.Model
	}
	return sessionModelFromTranscript(ev.TranscriptPath)
}

// ParentModelIdentifier is the session model: the payload "model", else the
// last turn_context model in the rollout tail.
func (ev Event) ParentModelIdentifier() (string, string) {
	if ev.Model != "" {
		return ev.Model, "payload"
	}
	if m := sessionModelFromTranscript(ev.TranscriptPath); m != "" {
		return m, "transcript"
	}
	return "", ""
}

// TaskText returns task content for local feature extraction. Raw text is
// never returned from the shared loop's persistence layer.
// Returns empty when the message looks like an encrypted Fernet token so that
// classifiers do not train on ciphertext or leak the task_name as a signal.
func (ev Event) TaskText() string {
	var ti map[string]any
	if json.Unmarshal(ev.ToolInput, &ti) != nil {
		return ""
	}
	message := hookutil.StringField(ti, "message")
	if isOpaqueToken(message) {
		return "" // encrypted instruction, no features
	}
	if taskName := hookutil.StringField(ti, "task_name"); taskName != "" {
		message = strings.TrimSpace(message + " " + taskName)
	}
	return message
}

// isOpaqueToken returns true when s looks like a Fernet-encrypted token that
// the hook has no key to decode. Fernet tokens are URL-safe base64, start with
// "gAAAAA" (version 0x80 + 8-byte timestamp), and are at least 57 bytes.
func isOpaqueToken(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) < 57 {
		return false
	}
	// Fernet version byte 0x80 encodes as "gA" in base64; a realistic token
	// starts with "gAAAAA" (version + all-zero or low timestamp bytes).
	if !strings.HasPrefix(s, "gAAAAA") {
		return false
	}
	// Accept with or without padding; a truncated token is still opaque.
	for _, c := range s {
		if !((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') ||
			(c >= '0' && c <= '9') || c == '-' || c == '_' || c == '=') {
			return false
		}
	}
	return true
}
