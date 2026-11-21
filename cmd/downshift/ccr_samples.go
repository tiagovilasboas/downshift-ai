// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"

	"github.com/tiagovilasboas/downshift-ai/internal/ccr"
	"github.com/tiagovilasboas/downshift-ai/internal/contextopt"
	"github.com/tiagovilasboas/downshift-ai/internal/execwrap"
	"github.com/tiagovilasboas/downshift-ai/internal/postrun"
)

// sampleSink returns the port that keeps a local raw sample of an output no
// compressor handled, or nil when sampling is off or the directory cannot be
// resolved. Fail open: a sampling error never touches the hook's answer.
func sampleSink() func(raw []byte, family, label string) {
	cfg, err := contextopt.LoadConfig()
	if !contextopt.CCRSamplesEnabled(cfg, err, os.Getenv("DOWNSHIFT_CCR_SAMPLES"), os.Getenv("DOWNSHIFT_CCR")) {
		return nil
	}
	s, err := ccr.DefaultSampler()
	if err != nil {
		return nil
	}
	return func(raw []byte, family, label string) { s.Maybe(raw, family, label) }
}

// sampleLabel is the code telemetry already logs for a run with no compressor:
// the inner program when the policy refused a metacharacter, else the head.
func sampleLabel(res postrun.Result) string {
	if res.Inner != "" {
		return res.Inner
	}
	return res.Head
}

// maybeSample hands an unchanged, uncompressed shell output to sink.
func maybeSample(sink func(raw []byte, family, label string), res postrun.Result, stdout string) {
	if sink == nil || !res.Considered || res.Replaced || res.Detail != execwrap.DetailNoCompressor {
		return
	}
	sink([]byte(stdout), string(res.Family), sampleLabel(res))
}
