// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"github.com/tiagovilasboas/downshift-ai/internal/core"
	"github.com/tiagovilasboas/downshift-ai/internal/telemetry"
)

// wireVerificationReporter installs the capability-driven child verification
// check on telemetry so the report reads declared harness capabilities.
// An unknown harness is measurable.
func wireVerificationReporter() {
	telemetry.VerificationReported = func(harness string) bool {
		caps, ok := core.CapsFor(harness)
		return !ok || caps.ReportsChildVerification
	}
}
