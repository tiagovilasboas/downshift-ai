// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import "fmt"

// Quota units: the routing figure that a token count cannot show.
//
// A lower tier is not fewer tokens, but a token on a lower tier uses less of a
// provider's quota. A quota unit is a token weighted by its tier and class,
// relative to a mid-tier token (= 1). The weights are the ratio of the Claude
// API public list rates read 2026-11-16, the same table the routing study
// froze (docs/evidence/routing-quota-study-protocol-2026-11-16.md). Unitless:
// no price, no currency.
//
// The estimate weighs the tokens a spawn spent twice: at the tier of the
// session model and at the tier it ran on. It assumes the session model would
// have spent the same tokens, which the controlled study did not confirm
// (routed runs can spend more tokens); it is an estimate, labelled as one.
//
// Haiku's long-prompt card (prompts over 100k tokens, weight 0.25) is not
// applied: usage records do not carry the per-request prompt size.

// quotaWeights are hundredths of a mid-tier token, per tier rank, for
// input, output and cache write (billable) and for cache read.
var quotaWeights = map[int]struct{ billable, cacheRead int64 }{
	TierRankSmall:    {billable: 5, cacheRead: 10},
	TierRankMid:      {billable: 100, cacheRead: 100},
	TierRankFrontier: {billable: 200, cacheRead: 200},
}

// QuotaWeightsSource names the weight table in every line that shows units.
const QuotaWeightsSource = "weights small 0.05 (cache read 0.1) · mid 1 · frontier 2, Claude API list rates 2026-11-16"

// QuotaUnits is the quota estimate of the spawns placed against the session
// model: their tokens weighted at the session tier and at the tier they ran on.
type QuotaUnits struct {
	Session float64 `json:"session"`
	Ran     float64 `json:"ran"`
}

// quotaUnitsAt weighs one usage record at a tier; ok is false for a tier the
// table does not know, which is never weighted by default.
func quotaUnitsAt(tier int, c TokenClasses) (float64, bool) {
	w, ok := quotaWeights[tier]
	if !ok {
		return 0, false
	}
	return float64(c.Billable()*w.billable+c.CacheRead*w.cacheRead) / 100, true
}

// addQuota weighs one session-placed record at the session tier and at the
// tier it ran on; a tier the weight table lacks leaves the estimate alone.
func (p *spawnPlacement) addQuota(base, got int, c TokenClasses) {
	session, okS := quotaUnitsAt(base, c)
	ran, okR := quotaUnitsAt(got, c)
	if okS && okR {
		p.quota.add(QuotaUnits{Session: session, Ran: ran})
	}
}

// Observed reports whether any record was weighted.
func (q QuotaUnits) Observed() bool { return q.Session > 0 }

// Saved is session units minus ran units; negative when spawns ran above it.
func (q QuotaUnits) Saved() float64 { return q.Session - q.Ran }

// SavedShare is Saved as a percent of the session units; 0 with none.
func (q QuotaUnits) SavedShare() float64 {
	if q.Session <= 0 {
		return 0
	}
	return 100 * q.Saved() / q.Session
}

func (q *QuotaUnits) add(o QuotaUnits) {
	q.Session += o.Session
	q.Ran += o.Ran
}

func (q QuotaUnits) parts() (dir string, share float64, saved, session string, ok bool) {
	if !q.Observed() {
		return "", 0, "", "", false
	}
	dir, s, pct := "lower", q.Saved(), q.SavedShare()
	if s < 0 {
		dir, s, pct = "higher", -s, -pct
	}
	return dir, pct, formatTokens(int64(s)), formatTokens(int64(q.Session)), true
}

// Line is "quota estimate: 41.0% lower than on the session model (38M of 93M
// mid-tier units) · same tokens assumed · weights ...", or "" with none.
func (q QuotaUnits) Line() string {
	dir, share, saved, session, ok := q.parts()
	if !ok {
		return ""
	}
	return fmt.Sprintf("quota estimate: %.1f%% %s than on the session model (%s of %s mid-tier units) · same tokens assumed · %s",
		share, dir, saved, session, QuotaWeightsSource)
}

// Short is "41.0% lower (38M of 93M mid units)" (or "higher" when negative),
// or "" with none.
func (q QuotaUnits) Short() string {
	dir, share, saved, session, ok := q.parts()
	if !ok {
		return ""
	}
	return fmt.Sprintf("%.1f%% %s (%s of %s mid units)", share, dir, saved, session)
}
