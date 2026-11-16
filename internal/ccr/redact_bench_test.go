// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package ccr

import (
	"bytes"
	"testing"
)

func BenchmarkRedactTestOutput(b *testing.B) {
	line := []byte("=== RUN   TestSomethingInPackage/subtest_case_01\n--- PASS: TestSomethingInPackage/subtest_case_01 (0.00s)\n")
	buf := bytes.Repeat(line, 20000) // ~2.1 MB
	b.SetBytes(int64(len(buf)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Redact(buf)
	}
}
