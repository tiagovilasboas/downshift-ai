// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package readout

import (
	"os"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/catalog"
	"github.com/tiagovilasboas/downshift-ai/internal/core"
	"github.com/tiagovilasboas/downshift-ai/internal/telemetry"
)

// The composition root sets DirectionCatalog; tests stand in for it.
func TestMain(m *testing.M) {
	telemetry.DirectionCatalog = catalog.TierViewOf(catalog.Load())
	for _, n := range core.AllHarnessCaps() {
		telemetry.DeclaredHarnesses = append(telemetry.DeclaredHarnesses, telemetry.HarnessSeam{Harness: n.Harness, ChildEndEvent: n.Caps.ChildEndEvent, UsageSource: n.Caps.UsageSource})
	}
	os.Exit(m.Run())
}
