// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package accounting

import (
	"sort"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/execwrap"
)

// CompactionDetail breaks the Compactor down per harness: by command family,
// by why a run was not compacted, and by why the hook left a command unwrapped.
// Counts and estimated tokens only, never a figure summed across harnesses.
type CompactionDetail struct {
	Harnesses []HarnessCompaction `json:"harnesses"`
}

// HarnessCompaction is one harness's Compactor detail. Families is sorted by
// raw tokens, largest first; the reason lists by count, largest first.
type HarnessCompaction struct {
	Harness      string             `json:"harness"`
	Commands     int64              `json:"commands"`
	Compacted    int64              `json:"compacted"`
	Families     []FamilyCompaction `json:"families"`
	NotCompacted []ReasonCount      `json:"not_compacted"`
	Skipped      []ReasonCount      `json:"skipped"`
	// PassthroughDetails refines the unrecognized_output rows by detail code
	// (danger_word, ansi, ...); SkipMetachars splits the metachar skips by
	// metacharacter class. Both are absent when no row carries the field.
	PassthroughDetails []ReasonCount `json:"passthrough_details,omitempty"`
	SkipMetachars      []ReasonCount `json:"skip_metachars,omitempty"`
	// UnknownReasons says why the rows of the unknown harness have no harness
	// (Unknown* codes). Present only on that row.
	UnknownReasons []ReasonCount `json:"unknown_reasons,omitempty"`
	// Other splits the `other` family by program; absent when it has no run.
	Other *OtherBreakdown `json:"other,omitempty"`
}

// FamilyCompaction is one command family in one harness. RawTokens is the
// output the commands printed and AvoidedTokens what the Compactor kept out of
// the context; both are bytes / execwrap.BytesPerToken, an estimate.
type FamilyCompaction struct {
	Family        string `json:"family"`
	Commands      int64  `json:"commands"`
	Compacted     int64  `json:"compacted"`
	RawTokens     int64  `json:"raw_tokens"`
	AvoidedTokens int64  `json:"avoided_tokens"`
	RawBytes      int64  `json:"-"`
	SavedBytes    int64  `json:"-"`
}

// ReasonCount is a fixed reason code and how many rows carry it.
type ReasonCount struct {
	Reason string `json:"reason"`
	Count  int64  `json:"count"`
}

// BuildCompactionDetail scopes the exec and skip logs like Compute and groups
// them per harness. Skip rows carry no session, so a session scope has none.
func BuildCompactionDetail(execEvents []execwrap.Event, skips []execwrap.SkipEvent, scope Scope, now time.Time) CompactionDetail {
	acc := map[string]*detailAcc{}
	get := func(h string) *detailAcc {
		if h == "" {
			h = UnknownHarness
		}
		if acc[h] == nil {
			acc[h] = &detailAcc{families: map[string]*FamilyCompaction{}, rawBytes: map[string]int64{}, savedBytes: map[string]int64{}, notCompacted: map[string]int64{}, skipped: map[string]int64{}, details: map[string]int64{}, metachars: map[string]int64{}, other: otherAcc{}}
		}
		return acc[h]
	}
	for _, ev := range scopeExec(execEvents, scope, now) {
		a := get(ev.Harness)
		a.addRun(ev)
		if isUnknownHarness(ev.Harness) {
			a.addUnknown(UnknownHarnessReason(ev.Entry))
		}
	}
	for _, sk := range scopeSkips(skips, scope, now) {
		a := get(sk.Harness)
		a.skipped[sk.Reason]++
		if isUnknownHarness(sk.Harness) {
			a.addUnknown(UnknownSkipRow)
		}
		if sk.Metachar != "" {
			a.metachars[sk.Metachar]++
		}
	}
	out := CompactionDetail{Harnesses: []HarnessCompaction{}}
	for h, a := range acc {
		out.Harnesses = append(out.Harnesses, a.row(h))
	}
	sort.Slice(out.Harnesses, func(i, j int) bool {
		a, b := out.Harnesses[i], out.Harnesses[j]
		if a.Commands != b.Commands {
			return a.Commands > b.Commands
		}
		return a.Harness < b.Harness
	})
	return out
}

// detailAcc gathers one harness's rows before they are sorted.
type detailAcc struct {
	commands, compacted   int64
	rawBytes, savedBytes  map[string]int64
	families              map[string]*FamilyCompaction
	notCompacted, skipped map[string]int64
	details, metachars    map[string]int64
	unknownWhy            map[string]int64
	other                 otherAcc
}

func (a *detailAcc) addRun(ev execwrap.Event) {
	fam := string(ev.Family)
	if fam == "" {
		fam = "other"
	}
	f := a.families[fam]
	if f == nil {
		f = &FamilyCompaction{Family: fam}
		a.families[fam] = f
	}
	if fam == "other" {
		a.other.add(ev)
	}
	a.commands++
	f.Commands++
	a.rawBytes[fam] += ev.RawBytes
	a.savedBytes[fam] += ev.NetSavedBytes()
	if ev.Compacted {
		a.compacted++
		f.Compacted++
		return
	}
	reason := ev.Passthrough
	if reason == "" {
		reason = execwrap.PassthroughNotRecorded
	}
	a.notCompacted[reason]++
	if ev.PassthroughDetail != "" {
		a.details[ev.PassthroughDetail]++
	}
}

func (a *detailAcc) row(h string) HarnessCompaction {
	r := HarnessCompaction{Harness: h, Commands: a.commands, Compacted: a.compacted, Families: []FamilyCompaction{}}
	for name, f := range a.families {
		f.RawBytes = a.rawBytes[name]
		f.SavedBytes = a.savedBytes[name]
		f.RawTokens = f.RawBytes / execwrap.BytesPerToken
		f.AvoidedTokens = f.SavedBytes / execwrap.BytesPerToken
		r.Families = append(r.Families, *f)
	}
	sort.Slice(r.Families, func(i, j int) bool {
		a, b := r.Families[i], r.Families[j]
		if a.RawTokens != b.RawTokens {
			return a.RawTokens > b.RawTokens
		}
		return a.Family < b.Family
	})
	r.Other = a.other.breakdown()
	r.NotCompacted = sortedReasons(a.notCompacted)
	r.Skipped = sortedReasons(a.skipped)
	if len(a.details) > 0 {
		r.PassthroughDetails = sortedReasons(a.details)
	}
	if len(a.metachars) > 0 {
		r.SkipMetachars = sortedReasons(a.metachars)
	}
	if len(a.unknownWhy) > 0 {
		r.UnknownReasons = sortedReasons(a.unknownWhy)
	}
	return r
}

// sortedReasons lists the counts largest first, then by reason code.
func sortedReasons(m map[string]int64) []ReasonCount {
	out := []ReasonCount{}
	for k, v := range m {
		out = append(out, ReasonCount{Reason: k, Count: v})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Reason < out[j].Reason
	})
	return out
}

// scopeSkips keeps the skip rows of scope. Skip rows hold no session, so a
// session scope keeps none; a timestamp that does not parse is outside every
// day window, as in scopeExec.
func scopeSkips(skips []execwrap.SkipEvent, scope Scope, now time.Time) []execwrap.SkipEvent {
	switch scope.Kind {
	case ScopeSession:
		return nil
	case ScopeDays:
		if scope.Days <= 0 {
			return skips
		}
		cutoff := now.Add(-time.Duration(scope.Days) * 24 * time.Hour)
		var out []execwrap.SkipEvent
		for _, sk := range skips {
			if t, err := time.Parse(time.RFC3339, sk.TS); err == nil && t.After(cutoff) {
				out = append(out, sk)
			}
		}
		return out
	}
	return skips
}
