// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package classify

import "testing"

func BenchmarkClassify(b *testing.B) {
	cases := []struct {
		name   string
		prompt string
	}{
		{"trivial", "rename variable foo to bar in main.go"},
		{"simple", "fix off-by-one bug in parseHeader function and add a unit test"},
		{"medium", "implement pagination for the users endpoint across handler, service, and repository"},
		{"complex", "design a distributed rate limiter across multiple regions with zero-downtime migration and rollback plan"},
	}
	// Warm lazy regexes before measuring steady-state classification.
	for _, tc := range cases {
		_ = Classify(tc.prompt)
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				_ = Classify(tc.prompt)
			}
		})
	}
}
