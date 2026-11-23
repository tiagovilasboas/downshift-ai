// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/compressor/learning"
	"github.com/tiagovilasboas/downshift-ai/internal/core"
	"github.com/tiagovilasboas/downshift-ai/internal/quota"
)

// wireRuntime connects all cross-package runtime hooks from the composition root.
func wireRuntime() {
	core.WireSession()
	quota.Register()
	core.SetDecider(routingDecider{now: time.Now})
	wireShellRewrite()
	learning.Register()
	wireVerificationReporter()
}
