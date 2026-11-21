// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package ccr

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testSampler(t *testing.T, n uint64) Sampler {
	t.Helper()
	return Sampler{Dir: filepath.Join(t.TempDir(), "s"), TTL: time.Hour, MaxTotal: 1 << 20,
		MinBytes: 100, N: n, Now: time.Now}
}

func body(i int) []byte { return []byte(strings.Repeat(fmt.Sprintf("line %d of output\n", i), 20)) }

func TestSamplerHashPicksAboutOneInN(t *testing.T) {
	s := testSampler(t, 4)
	hit := 0
	for i := 0; i < 200; i++ {
		if s.Maybe(body(i), "none", "git") {
			hit++
		}
	}
	if hit < 20 || hit > 90 {
		t.Fatalf("1 in 4 of 200 stored %d", hit)
	}
	// deterministic: the same content gives the same decision
	for i := 0; i < 20; i++ {
		a := testSampler(t, 4).Maybe(body(i), "none", "git")
		b := testSampler(t, 4).Maybe(body(i), "none", "git")
		if a != b {
			t.Fatalf("decision for %d is not stable", i)
		}
	}
}

func TestSamplerSizeFloor(t *testing.T) {
	s := testSampler(t, 1)
	if s.Maybe([]byte("tiny"), "none", "git") {
		t.Fatal("output under the floor was sampled")
	}
	if !s.Maybe(body(1), "none", "git") {
		t.Fatal("output over the floor was not sampled")
	}
}

func TestSamplerMetadataHoldsOnlyCodes(t *testing.T) {
	s := testSampler(t, 1)
	if !s.Maybe(body(1), "none", "git status /Users/x/secret path") {
		t.Fatal("not stored")
	}
	m := s.Samples()
	if len(m) != 1 || m[0].Bytes == 0 || m[0].Family != "none" {
		t.Fatalf("meta = %+v", m)
	}
	if strings.ContainsAny(m[0].Label, " /") {
		t.Fatalf("label carries text: %q", m[0].Label)
	}
	b, err := os.ReadFile(filepath.Join(s.Dir, m[0].ID+".json"))
	if err != nil || strings.Contains(string(b), "/Users") || strings.Contains(string(b), "line 1") {
		t.Fatalf("sidecar leaks: %s", b)
	}
	raw, err := s.Raw(m[0].ID)
	if err != nil || !strings.Contains(string(raw), "line 1 of output") {
		t.Fatalf("raw = %q, %v", raw, err)
	}
}

func TestSamplerCapPrunesOldest(t *testing.T) {
	s := testSampler(t, 1)
	now := time.Now()
	s.Now = func() time.Time { return now }
	s.MaxTotal = 3000
	var first string
	for i := 0; i < 8; i++ {
		now = now.Add(time.Second)
		s.Maybe(body(i), "none", "git")
		if i == 0 {
			first = s.Samples()[0].ID
		}
	}
	var total int64
	files, _ := os.ReadDir(s.Dir)
	for _, f := range files {
		info, _ := f.Info()
		total += info.Size()
	}
	if total > s.MaxTotal {
		t.Fatalf("total %d over cap %d", total, s.MaxTotal)
	}
	if _, err := s.Raw(first); err == nil {
		t.Fatal("oldest sample survived the cap")
	}
	if len(s.Samples()) == 0 {
		t.Fatal("everything was pruned")
	}
}

func TestSamplerTTLExpires(t *testing.T) {
	s := testSampler(t, 1)
	now := time.Now()
	s.Now = func() time.Time { return now }
	s.Maybe(body(1), "none", "git")
	now = now.Add(2 * time.Hour)
	s.Maybe(body(2), "none", "git")
	if got := s.Samples(); len(got) != 1 {
		t.Fatalf("samples after TTL = %d, want 1", len(got))
	}
}
