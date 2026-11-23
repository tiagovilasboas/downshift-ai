// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

// AgentTypeCount is how many decisions a subagent type had, and how many of
// those did not know the model the child would have run on.
type AgentTypeCount struct {
	Decisions       int `json:"decisions"`
	UnknownBaseline int `json:"unknown_baseline"`
}

// builtinAgentTypes are the subagent types Claude Code ships. They are safe to
// record by name: they say nothing about the user's work.
var builtinAgentTypes = map[string]bool{
	"general-purpose":   true,
	"claude":            true,
	"Explore":           true,
	"Plan":              true,
	"statusline-setup":  true,
	"claude-code-guide": true,
}

// AgentTypeLabel turns a subagent type into what may be stored and exported.
// stats --export is pasted into public issues, and a custom agent's name (a
// reviewer for one product, say) can say something about the user's work, so
// only the built-in names are kept. A call with no type is "unspecified";
// everything else is "custom". Names are case-sensitive, as in Claude Code.
func AgentTypeLabel(raw string) string {
	switch {
	case raw == "":
		return "unspecified"
	case builtinAgentTypes[raw]:
		return raw
	default:
		return "custom"
	}
}
