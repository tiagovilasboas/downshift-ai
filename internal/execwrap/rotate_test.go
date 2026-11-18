// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestRotateWithLimitRetainsMultipleGenerationsInOrder(t *testing.T) {
	const limit = 100
	dir := t.TempDir()
	path := filepath.Join(dir, "exec-events.jsonl")

	// Write 8 batches of ~110 bytes each, rotating after each batch exceeds limit,
	// then write a final smaller active batch that stays in path.
	const totalRotatedBatches = maxRotatedGenerations + 2 // 8 batches -> 1 & 2 dropped
	padding := strings.Repeat("x", 96)

	var expected []string
	for batch := 1; batch <= totalRotatedBatches; batch++ {
		line := fmt.Sprintf(`{"batch":%d,"pad":"%s"}`, batch, padding)
		if err := appendLine(path, []byte(line)); err != nil {
			t.Fatalf("append batch %d: %v", batch, err)
		}
		if batch > totalRotatedBatches-maxRotatedGenerations {
			expected = append(expected, line)
		}
		rotateWithLimit(path, limit)
	}

	activeLine := `{"batch":9,"pad":"active"}`
	if err := appendLine(path, []byte(activeLine)); err != nil {
		t.Fatalf("append active: %v", err)
	}
	expected = append(expected, activeLine)

	// Generations .1 through .6 must exist; .7 must never be created.
	for gen := 1; gen <= maxRotatedGenerations; gen++ {
		if _, err := os.Stat(rotatedPath(path, gen)); err != nil {
			t.Fatalf("expected rotated generation .%d to exist: %v", gen, err)
		}
	}
	if _, err := os.Stat(rotatedPath(path, maxRotatedGenerations+1)); !os.IsNotExist(err) {
		t.Fatalf("expected generation .%d not to exist, got err=%v", maxRotatedGenerations+1, err)
	}

	var got []string
	if err := scanLog(path, func(line []byte) {
		got = append(got, string(line))
	}); err != nil {
		t.Fatalf("scanLog: %v", err)
	}
	if !reflect.DeepEqual(got, expected) {
		t.Fatalf("scanLog order/content mismatch:\ngot:  %v\nwant: %v", got, expected)
	}
}

func TestRotateWithLimitSkipsWhenUnderLimitAndHandlesSparseGenerations(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "exec-skips.jsonl")

	small := `{"k":"small"}`
	if err := appendLine(path, []byte(small)); err != nil {
		t.Fatal(err)
	}
	rotateWithLimit(path, 100)
	if _, err := os.Stat(rotatedPath(path, 1)); !os.IsNotExist(err) {
		t.Fatalf("should not rotate under limit, got err=%v", err)
	}

	// Force over limit, rotate twice, verify .1 and .2 exist while .3 does not.
	big := fmt.Sprintf(`{"k":"%s"}`, strings.Repeat("a", 120))
	if err := appendLine(path, []byte(big)); err != nil {
		t.Fatal(err)
	}
	rotateWithLimit(path, 100)

	second := fmt.Sprintf(`{"k":"%s"}`, strings.Repeat("b", 120))
	if err := appendLine(path, []byte(second)); err != nil {
		t.Fatal(err)
	}
	rotateWithLimit(path, 100)

	var got []string
	if err := scanLog(path, func(line []byte) {
		got = append(got, string(line))
	}); err != nil {
		t.Fatal(err)
	}
	want := []string{small, big, second}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("scanLog = %v, want %v", got, want)
	}
	if _, err := os.Stat(rotatedPath(path, 3)); !os.IsNotExist(err) {
		t.Fatalf("expected .3 not to exist, got err=%v", err)
	}
}
