// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/ccr"
	"github.com/tiagovilasboas/downshift-ai/internal/execwrap"
	"github.com/tiagovilasboas/downshift-ai/internal/postrun"
)

func TestMaybeSampleOnlyUncompressedPassthrough(t *testing.T) {
	var got []string
	sink := func(raw []byte, family, label string) { got = append(got, family+"|"+label) }
	pass := postrun.Result{Considered: true, Detail: execwrap.DetailNoCompressor, Head: "git"}
	maybeSample(sink, pass, "x")
	maybeSample(sink, postrun.Result{Considered: true, Replaced: true, Detail: execwrap.DetailNoCompressor}, "x")
	maybeSample(sink, postrun.Result{Considered: true, Detail: execwrap.DetailUnknownShape}, "x")
	maybeSample(sink, postrun.Result{Detail: execwrap.DetailNoCompressor}, "x")
	maybeSample(nil, pass, "x")
	if len(got) != 1 || got[0] != "|git" {
		t.Fatalf("sampled %v", got)
	}
	inner := pass
	inner.Inner = "make"
	got = nil
	maybeSample(sink, inner, "x")
	if got[0] != "|make" {
		t.Fatalf("inner label: %v", got)
	}
}

func TestPrintSamplesTable(t *testing.T) {
	s := ccr.Sampler{Dir: filepath.Join(t.TempDir(), "s"), TTL: time.Hour, MaxTotal: 1 << 20, MinBytes: 10, N: 1}
	s.Maybe([]byte(strings.Repeat("same line\n", 100)), "none", "git")
	var out bytes.Buffer
	if printSamples(s, &out) != 0 {
		t.Fatal("non-zero exit")
	}
	if !strings.Contains(out.String(), "git") || !strings.Contains(out.String(), "LOSSLESS CUT") {
		t.Fatalf("table = %q", out.String())
	}
	var empty bytes.Buffer
	printSamples(ccr.Sampler{Dir: filepath.Join(t.TempDir(), "none")}, &empty)
	if !strings.Contains(empty.String(), "no samples") {
		t.Fatalf("empty = %q", empty.String())
	}
}

func TestRunCCRUsage(t *testing.T) {
	var out, errb bytes.Buffer
	if runCCR([]string{"x"}, &out, &errb) != 2 {
		t.Fatal("want usage exit 2")
	}
}
