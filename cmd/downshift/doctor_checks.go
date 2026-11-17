// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"

	"github.com/tiagovilasboas/downshift-ai/internal/paths"
	"github.com/tiagovilasboas/downshift-ai/internal/telemetry"
)

// buildProvenanceCheck reports the VCS stamp of the running binary. A binary
// built from a dirty tree warns; a clean build shows its short commit; a
// binary without VCS data (release builds) is unknown and never warns. Doctor
// cannot tell whether a commit is on main, so it reports only what the stamp says.
func buildProvenanceCheck(env doctorEnv) doctorCheck {
	const id = "build provenance"
	var info *debug.BuildInfo
	if env.BuildInfo != nil {
		info, _ = env.BuildInfo()
	}
	var rev, modified string
	if info != nil {
		for _, s := range info.Settings {
			switch s.Key {
			case "vcs.revision":
				rev = s.Value
			case "vcs.modified":
				modified = s.Value
			}
		}
	}
	switch {
	case modified == "true":
		return doctorCheck{ID: id, Status: doctorWarn, Detail: "built from a dirty tree",
			Fix: "rebuild from a clean checkout (git status must be empty), then reinstall the binary"}
	case rev != "":
		short := rev
		if len(short) > 7 {
			short = short[:7]
		}
		return doctorCheck{ID: id, Status: doctorOK, Detail: short}
	default:
		return doctorCheck{ID: id, Status: doctorOK, Detail: "unknown"}
	}
}

func fileExists(p string) bool { _, err := os.Stat(p); return err == nil }

func dirExists(p string) bool { fi, err := os.Stat(p); return err == nil && fi.IsDir() }

func resolvedPath(p string) string {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return filepath.Clean(p)
}

// checkBinary reports whether downshift is on PATH and whether it is the
// running binary. It returns the PATH location ("" when absent).
func checkBinary(rep *doctorReport, env doctorEnv) string {
	exe, _ := env.Executable()
	pathBin, err := env.LookPath("downshift")
	if err != nil {
		fix := "install downshift, or link this binary: ln -sf " + exe + " " + filepath.Join(env.Home, ".local", "bin", "downshift")
		rep.add(doctorCheck{ID: "binary on PATH", Status: doctorFail, Detail: "downshift is not on PATH", Fix: fix})
		return ""
	}
	if exe != "" && resolvedPath(exe) != resolvedPath(pathBin) {
		rep.add(doctorCheck{ID: "binary on PATH", Status: doctorWarn,
			Detail: fmt.Sprintf("PATH has %s; this run is %s", pathBin, exe),
			Fix:    fmt.Sprintf("install -m 0755 %s %s", exe, pathBin)})
		return pathBin
	}
	rep.add(doctorCheck{ID: "binary on PATH", Status: doctorOK, Detail: pathBin})
	return pathBin
}

// checkState reports the state directory and a legacy directory that still
// exists beside it. It returns the events of the effective state directory.
func checkState(rep *doctorReport, env doctorEnv) []telemetry.Event {
	dir, src, err := paths.Resolve()
	if err != nil {
		rep.add(doctorCheck{ID: "state dir", Status: doctorFail, Detail: err.Error(),
			Fix: "set " + paths.EnvStateDir + " to a writable absolute path"})
		return nil
	}
	rep.add(doctorCheck{ID: "state dir", Status: doctorOK, Detail: fmt.Sprintf("%s (%s)", dir, src)})
	events, _ := telemetry.ReadEventsTail(telemetry.DefaultTailBytes)

	if src == paths.SourceEnv {
		return events
	}
	newDir := filepath.Join(env.Home, paths.DirName)
	legacyDir := filepath.Join(env.Home, paths.LegacyName)
	switch {
	case src == paths.SourceLegacy:
		rep.add(doctorCheck{ID: "legacy dir", Status: doctorWarn,
			Detail: legacyDir + " is in use; the current location is " + newDir,
			Fix:    fmt.Sprintf("cp -R %s %s", legacyDir, newDir)})
	case dirExists(newDir) && dirExists(legacyDir):
		legacyEvents, _ := telemetry.ReadEventsTailFrom(filepath.Join(legacyDir, "events.jsonl"), telemetry.DefaultTailBytes)
		if eventsDiverge(events, legacyEvents) {
			rep.add(doctorCheck{ID: "legacy dir", Status: doctorWarn,
				Detail: fmt.Sprintf("%s also exists with different events; only %s is read", legacyDir, newDir),
				Fix:    fmt.Sprintf("mv %s %s.bak", legacyDir, legacyDir)})
		} else {
			rep.add(doctorCheck{ID: "legacy dir", Status: doctorOK, Detail: legacyDir + " exists, same events, ignored"})
		}
	}
	return events
}

func eventsDiverge(a, b []telemetry.Event) bool {
	if len(a) == 0 && len(b) == 0 {
		return false
	}
	if len(a) != len(b) {
		return true
	}
	return a[len(a)-1].CorrelationID != b[len(b)-1].CorrelationID
}
