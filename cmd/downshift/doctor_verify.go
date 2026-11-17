// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/core"
	"github.com/tiagovilasboas/downshift-ai/internal/execwrap"
	"github.com/tiagovilasboas/downshift-ai/internal/telemetry"
)

const verifyDefaultWindow = 15 * time.Minute

type verifyEnv struct {
	Now          func() time.Time
	ReadRouting  func() ([]telemetry.Event, error)
	ReadExec     func() ([]execwrap.Event, error)
	Window       time.Duration
	PollInterval time.Duration
	PollTimeout  time.Duration
	Sleep        func(time.Duration)
}

type verifyEventMatch struct {
	Found     bool   `json:"found"`
	Timestamp string `json:"timestamp,omitempty"`
	Detail    string `json:"detail,omitempty"`
}

type verifyResult struct {
	Harness      string           `json:"harness"`
	OK           bool             `json:"ok"`
	Since        string           `json:"since"`
	SpawnEvent   verifyEventMatch `json:"spawn_event"`
	ShellEvent   verifyEventMatch `json:"shell_event"`
	Instructions []string         `json:"instructions,omitempty"`
}

func defaultVerifyEnv() verifyEnv {
	return verifyEnv{
		Now: time.Now,
		ReadRouting: func() ([]telemetry.Event, error) {
			return telemetry.ReadEventsTail(telemetry.DefaultTailBytes)
		},
		ReadExec: func() ([]execwrap.Event, error) {
			p, err := execwrap.EventsPath()
			if err != nil {
				return nil, err
			}
			return execwrap.ReadEvents(p)
		},
		Window: verifyDefaultWindow,
		Sleep:  time.Sleep,
	}
}

func isSupportedHarness(name string) bool {
	for _, nc := range core.AllHarnessCaps() {
		if nc.Harness == name {
			return true
		}
	}
	return false
}

func runDoctorVerify(harness string, asJSON bool, w, errW io.Writer, env verifyEnv) int {
	if env.Now == nil {
		env.Now = time.Now
	}
	if env.Window <= 0 {
		env.Window = verifyDefaultWindow
	}
	since := env.Now().UTC().Add(-env.Window)
	res, err := confirmVerifyEvents(harness, since, env)
	if err != nil {
		fmt.Fprintf(errW, "doctor --verify %s: %v\n", harness, err)
		return 1
	}
	if !res.OK {
		res.Instructions = verifyInstructions(harness)
	}
	if asJSON {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		_ = enc.Encode(res)
	} else {
		writeVerifyText(w, res)
	}
	if !res.OK {
		return 1
	}
	return 0
}

func confirmVerifyEvents(harness string, since time.Time, env verifyEnv) (verifyResult, error) {
	start := env.Now()
	for {
		res, err := checkVerifyOnce(harness, since, env)
		if err != nil {
			return res, err
		}
		if res.OK || env.PollTimeout <= 0 || env.Now().Sub(start) >= env.PollTimeout {
			return res, nil
		}
		if env.Sleep != nil && env.PollInterval > 0 {
			env.Sleep(env.PollInterval)
		}
	}
}

func checkVerifyOnce(harness string, since time.Time, env verifyEnv) (verifyResult, error) {
	routing, err := env.ReadRouting()
	if err != nil {
		return verifyResult{}, fmt.Errorf("read routing events: %w", err)
	}
	execEvs, err := env.ReadExec()
	if err != nil {
		return verifyResult{}, fmt.Errorf("read exec events: %w", err)
	}
	res := verifyResult{
		Harness: harness,
		Since:   since.UTC().Format(time.RFC3339),
	}
	if ev, ts, ok := findRecentDecision(routing, harness, since); ok {
		res.SpawnEvent = verifyEventMatch{
			Found:     true,
			Timestamp: ts.UTC().Format(time.RFC3339),
			Detail:    fmt.Sprintf("%s (%s -> %s)", ev.Verdict, ev.FromModel, ev.ToModel),
		}
	}
	if ev, ts, ok := findRecentExec(execEvs, harness, since); ok {
		res.ShellEvent = verifyEventMatch{
			Found:     true,
			Timestamp: ts.UTC().Format(time.RFC3339),
			Detail:    fmt.Sprintf("family=%s exit=%s compacted=%t", ev.Family, ev.ExitClass, ev.Compacted),
		}
	}
	res.OK = res.SpawnEvent.Found && res.ShellEvent.Found
	return res, nil
}

func findRecentDecision(events []telemetry.Event, harness string, since time.Time) (telemetry.Event, time.Time, bool) {
	var best telemetry.Event
	var bestTS time.Time
	for _, ev := range events {
		if ev.Harness != harness || ev.Source != "hook" || !isDecision(ev) {
			continue
		}
		ts, err := time.Parse(time.RFC3339Nano, ev.Timestamp)
		if err != nil || ts.Before(since) {
			continue
		}
		if ts.After(bestTS) {
			best, bestTS = ev, ts
		}
	}
	return best, bestTS, !bestTS.IsZero()
}

func findRecentExec(events []execwrap.Event, harness string, since time.Time) (execwrap.Event, time.Time, bool) {
	var best execwrap.Event
	var bestTS time.Time
	for _, ev := range events {
		if ev.Harness != harness {
			continue
		}
		ts, err := time.Parse(time.RFC3339Nano, ev.TS)
		if err != nil || ts.Before(since) {
			continue
		}
		if ts.After(bestTS) {
			best, bestTS = ev, ts
		}
	}
	return best, bestTS, !bestTS.IsZero()
}

func writeVerifyText(w io.Writer, res verifyResult) {
	fmt.Fprintf(w, "downshift doctor --verify %s\n\n", res.Harness)
	if res.SpawnEvent.Found {
		fmt.Fprintf(w, "  ✓ spawn/decision  %s (%s)\n", res.SpawnEvent.Detail, res.SpawnEvent.Timestamp)
	} else {
		fmt.Fprintf(w, "  ✗ spawn/decision  none recorded since %s\n", res.Since)
	}
	if res.ShellEvent.Found {
		fmt.Fprintf(w, "  ✓ shell/exec      %s (%s)\n", res.ShellEvent.Detail, res.ShellEvent.Timestamp)
	} else {
		fmt.Fprintf(w, "  ✗ shell/exec      none recorded since %s\n", res.Since)
	}
	if res.OK {
		fmt.Fprintf(w, "\nPASS: verified spawn and shell events for %s\n", res.Harness)
		return
	}
	fmt.Fprintf(w, "\nSteps to complete verification in %s:\n", res.Harness)
	for i, step := range res.Instructions {
		fmt.Fprintf(w, "  %d. %s\n", i+1, step)
	}
	fmt.Fprintf(w, "\nFAIL: missing required events for %s\n", res.Harness)
}

func verifyInstructions(harness string) []string {
	caps, _ := core.CapsFor(harness)
	shellStep := fmt.Sprintf("Run one shell command in %s (for example: git status)", harness)
	if !caps.CanRewriteShellCommand {
		shellStep = fmt.Sprintf("Run one compacted shell command in %s (for example: downshift compact -- git status)", harness)
	}
	return []string{
		fmt.Sprintf("Open or restart %s so hooks are loaded for the session", harness),
		fmt.Sprintf("Trigger one subagent spawn in %s (ask it to delegate a small task)", harness),
		shellStep,
		fmt.Sprintf("Re-run: downshift doctor --verify %s", harness),
	}
}
