// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/core"
	"github.com/tiagovilasboas/downshift-ai/internal/routing/agenttier"
)

const providersSchema = `{"type":"object","properties":{"harness":{"type":"string","description":"Harness id, for example the one running you."},"models":{"type":"array","items":{"type":"string","maxLength":128},"maxItems":64,"description":"Optional: the model ids your own harness shows. Stored only as a hint; detection wins."},"include_agent_tiers":{"type":"boolean","description":"Optional: include the static agent_default_tiers table (also available from playbook)."}},"required":["harness"],"additionalProperties":false}`

const providersDesc = "Lists the models detected for the given harness right now, each with its tier, reasoning efforts, where Downshift learned it and an untiered flag. Call it before every subagent spawn (the Agent or Task tool), not once per session: it is the only step that shows the live models before you choose one. Fallback is true when only the catalog stand-in is known. session summarizes your spawns so far in this session (counts by tier and agent type, and how many ran above their agent's default tier); known is false when the harness gives this server no session id. You MAY pass the model ids your own harness shows in models: they are stored only as a hint and never override detection."

// ReportOutcome is what the store did with the ids the caller reported.
type ReportOutcome struct {
	Accepted int    `json:"accepted"`
	Written  bool   `json:"written"`
	Reason   string `json:"reason"`
}

// Reporter stores model ids the orchestrator sees for a harness. The store
// decides what is kept; the ids are hints and never task text.
type Reporter interface {
	Report(ctx context.Context, harness string, ids []string) (ReportOutcome, error)
}

// providersData is the providers result: the model list and, when the caller
// reported ids, what happened to them.
type providersData struct {
	ModelList
	Reported any `json:"reported,omitempty"`
	// AgentPrefix is what this harness puts before a squad agent's name.
	AgentPrefix string `json:"agent_prefix,omitempty"`
	// AgentDefaultTiers is the tier each squad agent declares: a spawn of that
	// agent with no model runs on it, and a harness that sends no agent type can
	// choose by it. Omitted by default to keep routine spawn calls compact;
	// pass include_agent_tiers=true (or call playbook) to read it.
	AgentDefaultTiers []agenttier.Entry `json:"agent_default_tiers,omitempty"`
	// Session is this session's spawn pattern (counts only), so the
	// orchestrator can see its own habit before the next spawn.
	Session *SessionSummary `json:"session,omitempty"`
}

// providersNext is the step after choosing: the hint the spawn hook reads, and the dry run.
const providersNext = "For mechanical work, start the spawn's prompt with [tier=small]; hints only move down, pinned/policy wins. Call apply to check a choice first."

func (d Deps) providersTool() Tool {
	fields := []string{"harness", "models", "include_agent_tiers"}
	t := d.build("providers", providersDesc, providersSchema, []string{"harness"}, nil)
	t.Handler = func(ctx context.Context, raw json.RawMessage) (Result, error) {
		rest, ids, err := splitModels(raw, fields)
		if err != nil {
			return Result{}, err
		}
		rest, includeTiers, err := splitBoolArg(rest, "include_agent_tiers", fields)
		if err != nil {
			return Result{}, err
		}
		args, err := parseArgs(rest, fields)
		if err != nil {
			return Result{}, err
		}
		out, err := d.providers(ctx, args["harness"], ids, includeTiers)
		if err != nil {
			return Result{}, err
		}
		return d.envelope(ctx, out)
	}
	return t
}

func (d Deps) providers(ctx context.Context, harness string, ids []string, includeTiers bool) (outcome, error) {
	if err := d.checkHarness(harness); err != nil {
		return outcome{}, err
	}
	var reported any
	if len(ids) > 0 {
		reported = d.report(ctx, harness, ids)
	}
	if d.Models == nil {
		return outcome{}, errors.New("detected models are not available")
	}
	list, err := d.Models.Available(ctx, harness)
	if err != nil {
		return outcome{}, errors.New("could not read the models for this harness")
	}
	data := providersData{
		ModelList:   slimModelList(list),
		Reported:    reported,
		AgentPrefix: squadPrefix(harness),
		Session:     d.sessionSummary(ctx, harness),
	}
	if includeTiers {
		data.AgentDefaultTiers = agenttier.List()
	}
	return outcome{data: data, harnesses: []string{harness}, shrink: shrinkModels, next: providersNext}, nil
}

// splitBoolArg extracts an optional boolean field from raw so parseArgs only
// sees string arguments.
func splitBoolArg(raw json.RawMessage, name string, allowed []string) (json.RawMessage, bool, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || string(trimmed) == "null" {
		return raw, false, nil
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &fields); err != nil {
		return raw, false, nil // parseArgs reports the malformed object
	}
	v, ok := fields[name]
	if !ok {
		return raw, false, nil
	}
	delete(fields, name)
	rest, err := json.Marshal(fields)
	if err != nil {
		return nil, false, errors.New("arguments could not be read")
	}
	if string(bytes.TrimSpace(v)) == "null" {
		return rest, false, nil
	}
	var b bool
	if err := json.Unmarshal(v, &b); err != nil {
		return nil, false, fmt.Errorf("field %q must be a boolean; allowed fields: %s", name, fieldList(allowed))
	}
	return rest, b, nil
}

// slimModelList strips the discovery cache timestamp that a spawner does not
// use so the providers payload stays compact.
func slimModelList(list ModelList) ModelList {
	list.CacheWrittenAt = time.Time{}
	if list.Models == nil {
		list.Models = []Model{}
	}
	return list
}

// report hands the ids to the Reporter; it never fails the call.
func (d Deps) report(ctx context.Context, harness string, ids []string) any {
	unavailable := map[string]bool{"unavailable": true}
	if d.Reporter == nil {
		return unavailable
	}
	res, err := d.Reporter.Report(ctx, harness, ids)
	if err != nil {
		return unavailable
	}
	return res
}

// squadPrefix is the harness's declared prefix of squad agent types.
func squadPrefix(harness string) string {
	caps, _ := core.CapsFor(harness)
	return caps.SquadAgentPrefix
}

func shrinkModels(data any) (any, bool) {
	p, ok := data.(providersData)
	if !ok {
		return nil, false
	}
	var cut bool
	p.Models, cut = halve(p.Models)
	return p, cut
}
