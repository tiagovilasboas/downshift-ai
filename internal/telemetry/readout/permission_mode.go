// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package readout

import (
	"fmt"
	"io"
	"sort"

	"github.com/tiagovilasboas/downshift-ai/internal/telemetry"
)

// CountPermissionModes counts decisions per permission mode. Decisions with no
// recorded mode add nothing, so logs written before the field count as none.
func CountPermissionModes(events []telemetry.Event) map[string]int {
	counts := map[string]int{}
	for _, ev := range events {
		if ev.PermissionMode != "" && telemetry.IsDecisionRow(ev) {
			counts[ev.PermissionMode]++
		}
	}
	return counts
}

// printPermissionModes lists decisions per permission mode, most used first.
// Nothing is printed when no decision carries a mode.
func printPermissionModes(w io.Writer, counts map[string]int) {
	if len(counts) == 0 {
		return
	}
	names := make([]string, 0, len(counts))
	for name := range counts {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		if counts[names[i]] != counts[names[j]] {
			return counts[names[i]] > counts[names[j]]
		}
		return names[i] < names[j]
	})
	fmt.Fprintf(w, "Decisions by permission mode\n")
	for _, name := range names {
		fmt.Fprintf(w, "  %-18s %5d\n", name, counts[name])
	}
	fmt.Fprintln(w)
}
