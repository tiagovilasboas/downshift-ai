// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"fmt"
	"os"
	"strings"
)

// This file is the one place the text shown to the developer inside a harness
// (child-end line) is built. Adapters and the CLI bridge only attach it to
// their wire output. Nothing here reads prompt text: it renders routing
// metadata that is already in the event log.

// Quiet reports the operator opt-out DOWNSHIFT_QUIET=1: no routing text is
// shown inside the harness (spawn message and child-end line). Logging to the
// event log and stderr notes are unchanged.
func Quiet() bool { return os.Getenv("DOWNSHIFT_QUIET") == "1" }

// ChildLineEnabled combines a harness capability (CanShowChildLine) with the
// operator opt-out.
func ChildLineEnabled(canShow bool) bool { return canShow && !Quiet() }

// ShortModel is the family name a developer recognises: claude-haiku-4-5-2025
// becomes haiku. Ids that do not start with "claude-" are returned unchanged,
// as is an id that has no non-numeric part left.
func ShortModel(id string) string {
	if !strings.HasPrefix(id, "claude-") {
		return id
	}
	var keep []string
	for _, p := range strings.Split(strings.TrimPrefix(id, "claude-"), "-") {
		if p == "" || (p[0] >= '0' && p[0] <= '9') {
			continue
		}
		keep = append(keep, p)
	}
	if len(keep) == 0 {
		return id
	}
	return strings.Join(keep, "-")
}

// ChildEndLine renders the one-line message shown when a child ends, from the
// usage record built for it: the model it ran on (final_model), its work and
// cache tokens and, when a different requested model is known, whether those
// tokens were saved from it (ran on a cheaper model) or ran on a pricier one.
// No dollars: the live line shows tokens only. Missing parts are omitted,
// never guessed. It returns "" when nothing is known.
func ChildEndLine(ev Event) string {
	var parts []string
	ran := ""
	if ev.ToModel != "" && ev.ToModel != "unknown" {
		ran = ShortModel(ev.ToModel)
		parts = append(parts, "finished on "+ran)
	}
	if HasChildTokens(ev) {
		c := ClassesOf(ev.TokenUsageOrZero())
		parts = append(parts, fmt.Sprintf("%s tokens (work %s, cache %s)", FormatTokens(c.Total), FormatTokens(c.Input+c.Output), FormatTokens(c.Cache())))
	}
	if want := ev.FromModel; want != "" && want != "unknown" && ShortModel(want) != ran && ev.HasTokenUsage() {
		guess := ""
		if ev.BaselineSource == BaselineInferred {
			guess = " (inferred baseline)"
		}
		switch TierDirection(ev.Harness, want, ev.ToModel) {
		case TokenCheaper:
			parts = append(parts, "lower tier than "+ShortModel(want)+guess)
		case TokenPricier:
			parts = append(parts, "higher tier than "+ShortModel(want)+guess)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return "↳ downshift: " + strings.Join(parts, " · ")
}

// FormatTokens formats a token count into a compact human string (e.g. 1.2k, 12k, 1.5M).
func FormatTokens(n int64) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1e6)
	case n >= 10_000:
		return fmt.Sprintf("%dk", (n+500)/1000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fk", float64(n)/1e3)
	}
	return fmt.Sprintf("%d", n)
}
