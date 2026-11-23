// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import "fmt"

// CacheObserved reports whether any prompt-cache token class (read or write)
// was recorded for this row.
func (r RoutingRow) CacheObserved() bool {
	return r.CacheRead > 0 || r.CacheWrite > 0
}

// CacheHitKnown reports whether CacheHitPct has a non-read denominator
// (CacheWrite or uncached Input), so legacy rows with only CacheRead set
// do not invent a misleading 100% cache hit rate.
func (r RoutingRow) CacheHitKnown() bool {
	return r.CacheObserved() && (r.CacheWrite > 0 || r.Input > 0)
}

// CacheHitPct returns CacheRead / (CacheRead + CacheWrite + Input) as a
// percentage when CacheHitKnown is true, or 0 otherwise.
func (r RoutingRow) CacheHitPct() float64 {
	if !r.CacheHitKnown() {
		return 0
	}
	total := r.CacheRead + r.CacheWrite + r.Input
	if total <= 0 {
		return 0
	}
	return 100 * float64(r.CacheRead) / float64(total)
}

// cacheReadText formats the cache summary line for plain-text or Markdown
// routing reports, appending cache hit rate and cache writes when known.
func (r RoutingRow) cacheReadText(markdown bool) string {
	if !r.CacheObserved() {
		return ""
	}
	if markdown {
		base := fmt.Sprintf("cache read %s, apart from the tokens above", ShortTokens(r.CacheRead))
		if r.CacheHitKnown() {
			return fmt.Sprintf("%s (%.1f%% cache hit; cache write %s)", base, r.CacheHitPct(), ShortTokens(r.CacheWrite))
		}
		return base
	}
	base := fmt.Sprintf("cache read  %s (apart from billable)", formatTokens(r.CacheRead))
	if r.CacheHitKnown() {
		return fmt.Sprintf("%s · %.1f%% cache hit (cache write %s)", base, r.CacheHitPct(), formatTokens(r.CacheWrite))
	}
	return base
}
