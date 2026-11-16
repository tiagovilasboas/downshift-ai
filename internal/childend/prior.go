// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package childend

import (
	"github.com/tiagovilasboas/downshift-ai/internal/core"
	"github.com/tiagovilasboas/downshift-ai/internal/hookport"
	"github.com/tiagovilasboas/downshift-ai/internal/telemetry"
)

// priorTailBytes bounds how much of the event log a record reads to find its
// decision and to dedupe. The recorder runs inside a hook, so reading the whole
// log made the hook slower as the log grew. 4 MiB holds several thousand
// events, far more than the decisions and usage lines of one spawn.
//
// Window limits: a decision older than the window is not found, so its usage is
// recorded unlinked (counted as unrouted). A priced record older than the
// window is not seen, so a repeat of the same call id, or a later stop of the
// same agent that old, is recorded again. Both need thousands of events between
// a spawn and its end, which log rotation and the 24h link window make rare.
const priorTailBytes int64 = 4 << 20

// billableUsage is the usage a record may price: the harness must have a
// proven usage source (unverified tokens could belong to the parent thread).
func billableUsage(ce hookport.ChildEnd) telemetry.TokenUsage {
	caps, ok := core.CapsFor(ce.Harness)
	if ok && caps.UsageSource != "" && caps.UsageSource != core.UsageNone && ce.Usage != nil {
		return *ce.Usage
	}
	return telemetry.TokenUsage{}
}

// needsPrior reports whether Record can write anything that depends on earlier
// events. When it cannot, the log is not read at all.
func needsPrior(ce hookport.ChildEnd, usage telemetry.TokenUsage, sessionHash, callHash, agentHash string) bool {
	if ce.Source == hookport.SourceTranscript {
		return usage != (telemetry.TokenUsage{}) && agentHash != ""
	}
	if usage != (telemetry.TokenUsage{}) {
		return true
	}
	if sessionHash == "" && callHash == "" {
		return false
	}
	return ce.ResolvedModel != "" || (ce.Status != "" && ce.Status != telemetry.ChildUnknown)
}

// readPrior reads the recent events a record links and dedupes against. A
// missing or unreadable log links nothing.
func readPrior() []telemetry.Event {
	prior, _ := telemetry.ReadEventsTail(priorTailBytes)
	return prior
}
