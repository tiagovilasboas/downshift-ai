// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"github.com/tiagovilasboas/downshift-ai/internal/core"
	"github.com/tiagovilasboas/downshift-ai/internal/telemetry"
)

// The report reads which harness can report a child's check from the declared
// capabilities, so no harness is named here. An unknown harness is measurable.
func init() {
	telemetry.VerificationReported = func(harness string) bool {
		caps, ok := core.CapsFor(harness)
		return !ok || caps.ReportsChildVerification
	}
}
