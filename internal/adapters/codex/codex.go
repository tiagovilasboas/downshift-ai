// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package codex adapts core routing decisions to OpenAI Codex CLI's hook
// protocol under multi_agent_v2.
//
// Codex spawns subagents through a reserved spawn_agent tool. A PreToolUse
// hook can inject the model and reasoning_effort into the tool input via
// hookSpecificOutput.updatedInput before the local spawn handler creates
// the child.
//
// reasoning_effort is translated from core.Effort via the catalog's
// effort_map for the target model — no hardcoded switch in this adapter.
package codex

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/core"
	"github.com/tiagovilasboas/downshift-ai/internal/hookutil"
	"github.com/tiagovilasboas/downshift-ai/internal/quota"
)

const harnessID = "codex"

// Output is the JSON we print on stdout to steer Codex's PreToolUse.
type Output struct {
	HookSpecificOutput *HookSpecificOutput `json:"hookSpecificOutput,omitempty"`
}

// HookSpecificOutput carries the PreToolUse decision.
type HookSpecificOutput struct {
	HookEventName            string          `json:"hookEventName"`
	PermissionDecision       string          `json:"permissionDecision"`
	PermissionDecisionReason string          `json:"permissionDecisionReason,omitempty"`
	UpdatedInput             json.RawMessage `json:"updatedInput,omitempty"`
}

// Handle processes a PreToolUse event. Accepts an optional catalog.Resolver
// for model lookup and effort translation. Falls back to legacy core.Catalog
// when nil (tests that don't inject a resolver).
// Returns the hook output, a human-readable note, and the full routing Decision
// so callers can record telemetry without re-classifying the prompt.
func Handle(ev Event, r ...core.Resolver) (Output, string, core.Decision) {
	var ti map[string]any
	if err := json.Unmarshal(ev.ToolInput, &ti); err != nil {
		return allow(), "", core.Decision{}
	}

	// Structural detection: a Codex subagent spawn carries "message" and/or
	// "task_name" in tool_input. Tool name is deliberately not checked.
	subPrompt := hookutil.StringField(ti, "message")

	// If the message looks like a Fernet-encrypted token, it is opaque: we
	// cannot read or classify the task, so we leave the spawn unchanged and
	// return a Decision that records the observation without any features.
	if isOpaqueToken(subPrompt) {
		d := core.Decision{
			Harness:        harnessID,
			TaskNotVisible: true,
			NoSignal:       true,
			Confident:      false,
		}
		return allow(), "", d
	}

	if tn := hookutil.StringField(ti, "task_name"); tn != "" {
		subPrompt = strings.TrimSpace(subPrompt + " " + tn)
	}
	if subPrompt == "" {
		return allow(), "", core.Decision{}
	}

	currentModel := ev.currentModel(ti)

	var res core.Resolver
	if len(r) > 0 {
		res = r[0]
	}
	session := core.ResolveSessionForID(harnessID, ev.SessionID, ev.SessionModels, ev.AvailableModels)
	session = session.WithHookQuota(ev.IncludedModels, ev.UnavailableModels)
	usage := ev.UsageQuota
	if usage == nil && ev.TranscriptPath != "" {
		usage = &quota.Snapshot{Harness: harnessID}
		if observed, err := quota.CollectCodex(ev.TranscriptPath, 5*time.Minute); err == nil {
			usage = &observed
		}
	}
	session = session.WithUsageQuota(harnessID, usage)
	if ev.ModelMetadata != nil {
		session.Metadata = ev.ModelMetadata
	}
	res = core.ResolveSessionModels(harnessID, session, res)
	decision := core.RouteSpawn(core.Request{
		Prompt: subPrompt, Harness: harnessID, AgentType: hookutil.StringField(ti, "agent_type"),
		ModelInherited: hookutil.StringField(ti, "model") == "", CurrentModelID: currentModel,
	}, res)
	decision.Baseline.RequestedID = currentModel

	plan := decision.PlanSession(core.CodexCaps, res, session, currentModel)
	if plan.HoldForeign || plan.PreserveExplicit || (!plan.RewriteModel && !plan.ApplyEffort) {
		return allow(), "", decision
	}
	if plan.RewriteModel && !core.CanWriteSessionID(harnessID, plan.Model.ID, plan.Session, res) {
		return allow(), "", decision
	}
	effort, effortOK, effortChanged := core.ConstrainEffort(res, harnessID, plan.Model.ID, decision.Effort)
	effortValue := effort.String()
	if res != nil {
		effortValue = res.EffortFor(harnessID, plan.Model.ID, effort)
	}
	requestedEffort := hookutil.StringField(ti, "reasoning_effort")
	applyEffort := plan.ApplyEffort && effortOK && hookutil.EffortAllowed(decision, requestedEffort, effortValue, plan.RewriteModel)
	if plan.ApplyEffort && effortChanged {
		decision.Corrections = append(decision.Corrections, core.RuleEffortUnsupported)
	}
	if !plan.RewriteModel && (!applyEffort || effortValue == requestedEffort) {
		return allow(), "", decision
	}

	note := ""
	if plan.RewriteModel {
		decision.Model = plan.Model
		ti["model"] = plan.WriteName
		note = decision.Summary()
	} else {
		// Effort-only change: the model stays as requested, and the event
		// must record the model the child actually runs on.
		decision.Model = decision.Baseline.CurrentModel
		note = fmt.Sprintf("%s task → keep %s, reasoning effort %s → %s",
			decision.Complexity, decision.Baseline.CurrentModel.ID, effortOrInherit(requestedEffort), effortValue)
	}
	if applyEffort {
		ti["reasoning_effort"] = effortValue
	}
	updated, err := json.Marshal(ti)
	if err != nil {
		return allow(), "", decision
	}

	out := Output{
		HookSpecificOutput: &HookSpecificOutput{
			HookEventName:      "PreToolUse",
			PermissionDecision: "allow",
			UpdatedInput:       updated,
		},
	}
	return out, note, decision
}

func effortOrInherit(v string) string {
	if v == "" {
		return "inherit"
	}
	return v
}

// allow answers "no opinion": an empty object, no permissionDecision. An
// explicit "allow" would override the user's own ask/deny permission rules for
// the spawning tool, and Codex rejects a bare allow without updatedInput. The
// harness then applies its normal permission flow to the unchanged call.
func allow() Output { return Output{} }
