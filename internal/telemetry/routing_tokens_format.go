// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"fmt"
	"sort"
	"strings"
)

// Line is the short text every live view uses, or "" when there is no spawn:
// "12k of 40k billable below the session model (3 of 5 spawns)". The fallback
// part is its own clause: "against the requested model". Cache reads are not
// in it. A lower tier is not fewer tokens: it says where tokens ran.
func (r RoutingRow) Line() string {
	var parts []string
	if r.Observed() {
		parts = append(parts, fmt.Sprintf("%s of %s billable below the session model (%d of %s)",
			formatTokens(r.BelowBillable), formatTokens(r.Billable), r.BelowSpawns, spawnsText(r.Spawns)))
		if r.RewrittenSpawns > 0 {
			parts = append(parts, fmt.Sprintf("by a Downshift rewrite: %s (%s tokens)", spawnsText(r.RewrittenSpawns), formatTokens(r.RewrittenBillable)))
		}
	}
	if r.FallbackObserved() {
		parts = append(parts, fmt.Sprintf("%s of %s billable below the requested model (%s, no session model)",
			formatTokens(r.FallbackBelowBillable), formatTokens(r.FallbackBillable), spawnsText(r.FallbackSpawns)))
	}
	if r.UnobservedSpawns > 0 {
		parts = append(parts, fmt.Sprintf("%d unobserved", r.UnobservedSpawns))
	}
	return strings.Join(parts, " · ")
}

// spawnsText is "1 spawn" or "3 spawns".
func spawnsText(n int) string {
	if n == 1 {
		return "1 spawn"
	}
	return fmt.Sprintf("%d spawns", n)
}

// HarnessOrder lists the harness ids of a view in print order: declared
// harnesses first, in DeclaredHarnesses order, then any other id, sorted.
func (v RoutingView) HarnessOrder() []string {
	seen := map[string]bool{}
	var out []string
	for _, n := range DeclaredHarnesses {
		if _, ok := v.ByHarness[n.Harness]; ok {
			out = append(out, n.Harness)
			seen[n.Harness] = true
		}
	}
	var rest []string
	for h := range v.ByHarness {
		if !seen[h] {
			rest = append(rest, h)
		}
	}
	sort.Strings(rest)
	return append(out, rest...)
}
