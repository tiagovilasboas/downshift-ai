// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package readout

import (
	"fmt"
	"github.com/tiagovilasboas/downshift-ai/internal/telemetry"
	"io"
	"sort"
)

// StatsOptions controls PrintStats output.
type StatsOptions struct {
	Days int // time window; <= 0 means all time
	// Detail adds the full report (honor, quota, usage, child status,
	// complexity breakdown) after the 3-line headline.
	Detail bool
}

// PrintStats writes a human-readable stats summary to w.
func PrintStats(events []telemetry.Event, opts StatsOptions, w io.Writer) {
	filtered := telemetry.FilterByDays(events, opts.Days)
	s := Aggregate(filtered)

	label := fmt.Sprintf("Last %d days", opts.Days)
	if opts.Days <= 0 {
		label = "All time"
	}

	printHeadline(w, s, s.Routing, opts.Days)
	fmt.Fprintf(w, "A cheaper tier is not fewer tokens. The routed figures and the rework per verdict: downshift report\n")
	if !opts.Detail {
		fmt.Fprintf(w, "\nFull report: downshift stats --detail\n")
		return
	}
	fmt.Fprintf(w, "\n%s\n", label)
	fmt.Fprintf(w, "─────────────────────────────────────\n")
	fmt.Fprintf(w, "Subagent decisions    %8d\n", s.Total)
	fmt.Fprintf(w, "\n")
	fmt.Fprintf(w, "  Downshifted         %8d  %5.1f%%  (rewrite applied)\n", s.Downshifted, pct(s.Downshifted, s.Total))
	fmt.Fprintf(w, "  Upshifted           %8d  %5.1f%%  (rewrite applied)\n", s.Upshifted, pct(s.Upshifted, s.Total))
	fmt.Fprintf(w, "  Not applied         %8d  %5.1f%%  (classified shift, spawn unchanged: allow/held/blocked)\n", s.NotApplied, pct(s.NotApplied, s.Total))
	fmt.Fprintf(w, "  Unchanged (OK)      %8d  %5.1f%%\n", s.OK, pct(s.OK, s.Total))
	fmt.Fprintf(w, "  Unknown             %8d  %5.1f%%\n", s.Unknown, pct(s.Unknown, s.Total))
	if s.PolicyFloor > 0 {
		fmt.Fprintf(w, "  Raised by project floor %6d  %5.1f%%  (.downshift/policy.json)\n", s.PolicyFloor, pct(s.PolicyFloor, s.Total))
	}
	printChildStatus(w, s.ChildStatus)
	printAgentTypes(w, s.ByAgentType)
	if s.RespawnAfterFailure > 0 {
		fmt.Fprintf(w, "  Respawned after a failure  %5d  (hint only: may be unrelated work)\n", s.RespawnAfterFailure)
	}
	fmt.Fprintf(w, "\n")
	fmt.Fprintf(w, "By complexity\n")
	for _, cls := range []string{"TRIVIAL", "SIMPLE", "MEDIUM", "COMPLEX"} {
		n := s.ByComplexity[cls]
		fmt.Fprintf(w, "  %-8s            %8d  %5.1f%%\n", cls, n, pct(n, s.Total))
	}
	fmt.Fprintf(w, "\n")
	printEffortMix(w, s.EffortMix, s.Total)

	if s.Corrected > 0 {
		fmt.Fprintf(w, "Safety-held           %8d  (guardrail applied, see corrections)\n", s.Corrected)
	}
	if s.ObservedChildUsage == 0 {
		fmt.Fprintf(w, "Observed child tokens unobserved\n")
	} else {
		fmt.Fprintf(w, "Observed child tokens %12d  (%d usage records)\n", s.ObservedChildTokens, s.ObservedChildUsage)
	}
	fmt.Fprintln(w)
	PrintRouting(w, s.Routing)
	if s.UsageRecords > 0 {
		fmt.Fprintf(w, "Usage records        %8d  (all post-hoc usage records)\n", s.UsageRecords)
		fmt.Fprintf(w, "Usage linked          %8d  (distinct usage matched to decisions in this window)\n", s.UsageLinked)
	}
	if s.Baseline > 0 {
		fmt.Fprintf(w, "Baseline (unrouted)   %8d  (paused sessions and older baseline lines, excluded from rates)\n", s.Baseline)
	}
	if s.Total > 0 {
		fmt.Fprintf(w, "Honor                 observed %d  refuted %d  unobserved %d  (applied rewrites only)\n", s.HonorObserved, s.HonorRefuted, s.HonorUnobserved)
		fmt.Fprintf(w, "Quota                 available %d  held %d  unknown %d\n", s.QuotaAvailable, s.QuotaHeld, s.QuotaUnknown)
		fmt.Fprintf(w, "Usage                 observed %d  unobserved %d\n", s.UsageObserved, s.UsageUnobserved)
		fmt.Fprintf(w, "Frontier decisions    %d  (rewritten on doubt, no deterministic lift: %d)\n", s.FrontierDecisions, s.FrontierOnDoubt)
		printGatewayLine(w, s)
		if n := s.ReasonCodes[telemetry.ReasonSemanticBoost]; n > 0 {
			fmt.Fprintf(w, "Semantic boost        %d of %d decisions (%.0f%%) were raised by the embedder, not a rule\n", n, s.Total, pct(n, s.Total))
		}
		if n := s.ReasonCodes[telemetry.ReasonUnsupportedLanguage]; n > 0 {
			fmt.Fprintf(w, "Language guard        %d of %d decisions (%.0f%%) lacked English/Portuguese rule coverage (%s)\n", n, s.Total, pct(n, s.Total), telemetry.ReasonUnsupportedLanguage)
		}
		if s.FrontierOnDoubt > 0 {
			fmt.Fprintf(w, "WARNING: %d frontier rewrite(s) were applied on an unconfident classification with no deterministic lift; check for silent overspend.\n", s.FrontierOnDoubt)
		}
	}
	fmt.Fprintf(w, "─────────────────────────────────────\n")
}

// printGatewayLine reports the OpenRouter allowlist shadow. It prints nothing
// unless at least one decision would have been served by an allowlisted row.
// The slug shown is the most frequent one; ties break on the slug.
func printGatewayLine(w io.Writer, s Stats) {
	if s.GatewayObserved == 0 {
		return
	}
	top, topN := "", 0
	for slug, n := range s.GatewaySlugs {
		if n > topN || (n == topN && slug < top) {
			top, topN = slug, n
		}
	}
	fmt.Fprintf(w, "shadow: would route %d of %d spawns to %s", s.GatewayObserved, s.Total, top)
	if len(s.GatewaySlugs) > 1 {
		fmt.Fprintf(w, " (most frequent of %d slugs)", len(s.GatewaySlugs))
	}
	fmt.Fprintf(w, "\n")
}

func pct(n, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(n) / float64(total) * 100
}

// childStatusOrder is the fixed print order for "How children ended".
var childStatusOrder = []telemetry.ChildStatus{telemetry.ChildCompleted, telemetry.ChildLaunched, telemetry.ChildInterrupted, telemetry.ChildError, telemetry.ChildStopped, telemetry.ChildUnknown}

// printChildStatus writes the "How children ended" section. It prints nothing
// when no child_ended events were counted, so output without them is unchanged.
func printChildStatus(w io.Writer, counts map[telemetry.ChildStatus]int) {
	sum := 0
	for _, n := range counts {
		sum += n
	}
	if sum == 0 {
		return
	}
	fmt.Fprintf(w, "\n")
	fmt.Fprintf(w, "How children ended\n")
	for _, st := range childStatusOrder {
		if n := counts[st]; n > 0 {
			fmt.Fprintf(w, "  %-11s         %8d\n", st, n)
		}
	}
	fmt.Fprintf(w, "  Note: launched is a background start, not a success.\n")
}

// printEffortMix writes the "By effort" section when at least one decision
// carried a known reasoning effort.
func printEffortMix(w io.Writer, mix map[string]int, total int) {
	if len(mix) == 0 {
		return
	}
	fmt.Fprintf(w, "By effort\n")
	keys := make([]string, 0, len(mix))
	for k := range mix {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		ri, rj := effortLevels[keys[i]], effortLevels[keys[j]]
		if ri != rj {
			return ri < rj
		}
		return keys[i] < keys[j]
	})
	for _, k := range keys {
		n := mix[k]
		fmt.Fprintf(w, "  %-8s            %8d  %5.1f%%\n", k, n, pct(n, total))
	}
	fmt.Fprintf(w, "\n")
}

// printAgentTypes lists decisions per subagent type with how many had no known
// baseline, the type with the most unknown first. Nothing is printed when no
// event carries a type (logs written before the field existed).
func printAgentTypes(w io.Writer, byType map[string]telemetry.AgentTypeCount) {
	if len(byType) == 0 {
		return
	}
	names := make([]string, 0, len(byType))
	for name := range byType {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		a, b := byType[names[i]], byType[names[j]]
		if a.UnknownBaseline != b.UnknownBaseline {
			return a.UnknownBaseline > b.UnknownBaseline
		}
		if a.Decisions != b.Decisions {
			return a.Decisions > b.Decisions
		}
		return names[i] < names[j]
	})
	fmt.Fprintf(w, "\nBaseline by agent type\n")
	for _, name := range names {
		c := byType[name]
		fmt.Fprintf(w, "  %-20s %5d decisions  %5d unknown  (%.0f%%)\n", name, c.Decisions, c.UnknownBaseline, pct(c.UnknownBaseline, c.Decisions))
	}
}
