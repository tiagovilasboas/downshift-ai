// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"strings"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/lazyre"
)

// OutcomeSkillUsed marks one skill activation (for example, Claude Code's Skill
// tool). It is an adoption observation, not a routing decision and not a token
// saving: Aggregate, BuildTasks, IsDecisionRow, and RoutingTokens skip it.
const OutcomeSkillUsed = "skill_used"

var skillSlugPattern = lazyre.New(`^[a-z0-9][a-z0-9-]{0,47}$`)

// SkillNameLabel turns a raw skill identifier into a privacy-safe label for
// durable logging. Only public Downshift/ponytail and superpowers:* names with
// safe slug characters are kept; any other non-empty name collapses to "other"
// so private project or skill names, paths, and arguments never reach the log.
func SkillNameLabel(raw string) string {
	if raw == "" {
		return ""
	}
	trimmed := strings.TrimPrefix(raw, "downshift:")
	switch {
	case trimmed == "ponytail":
		return "ponytail"
	case strings.HasPrefix(trimmed, "downshift-") && skillSlugPattern.MatchString(trimmed):
		return trimmed
	case strings.HasPrefix(raw, "superpowers:") && skillSlugPattern.MatchString(strings.TrimPrefix(raw, "superpowers:")):
		return raw
	default:
		return "other"
	}
}

// SkillUsed builds the record of one skill activation. The skill name is
// bounded through SkillNameLabel, and sessionHash must already be hashed via
// HashSessionID. It records counts only — never token savings.
func SkillUsed(harness, sessionHash, skill, binaryVersion string) Event {
	return Event{
		CorrelationID: NewCorrelationID(),
		Timestamp:     time.Now().UTC().Format(time.RFC3339Nano),
		Source:        "hook",
		Agent:         SkillNameLabel(skill),
		Harness:       harness,
		SessionID:     sessionHash,
		Outcome:       OutcomeSkillUsed,
		BinaryVersion: binaryVersion,
	}
}
