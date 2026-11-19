// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package opencode

import "github.com/tiagovilasboas/downshift-ai/internal/telemetry"

func invalidSkillField(req Request) string {
	checks := []struct {
		field string
		ok    bool
	}{
		{"v", req.V == 1},
		{"kind", req.Kind == kindSkill},
		{"harness.id", req.Harness.ID == harnessID},
		{"event", req.Event == eventSkill},
		{"harness.version", len(req.Harness.Version) <= maxVersionLen},
		{"correlation.child", childIDPattern.MatchString(req.Correlation.Child)},
		{"tool", len(req.Tool) <= maxToolLen},
		{"skill", len(req.Skill) <= maxSkillLen},
		{"path", len(req.Path) <= maxPathLen},
		{"taskText", len(req.TaskText) <= maxTaskTextBytes},
		{"tsUtc", len(req.TsUtc) <= maxTsUtcLen},
		{"signal", req.Tool != "" || req.TaskText != ""},
	}
	for _, c := range checks {
		if !c.ok {
			return c.field
		}
	}
	return ""
}

func handleSkill(req Request) Response {
	if field := invalidSkillField(req); field != "" {
		resp := failOpen()
		resp.InvalidField = field
		return resp
	}
	return allow(reasonSkillObserved)
}

// UsedSkills extracts privacy-safe skill labels from a validated "skill"
// observation request via the shared telemetry helpers.
func (r Request) UsedSkills() []string {
	if r.Kind != kindSkill || invalidSkillField(r) != "" {
		return nil
	}
	if r.Tool != "" {
		ti := map[string]any{
			"skill":    r.Skill,
			"name":     r.Skill,
			"path":     r.Path,
			"filePath": r.Path,
		}
		return telemetry.SkillsFromToolCall(r.Tool, ti)
	}
	return telemetry.SkillsFromPrompt(r.TaskText)
}
