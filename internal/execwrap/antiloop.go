// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/paths"
)

const (
	antiLoopThreshold = 3
	antiLoopPrefix    = "ccr-loop-"
	// antiLoopWindow is how long a streak stays alive: an older state counts
	// as fresh, so a failure hours later starts again at 1.
	antiLoopWindow = 30 * time.Minute
	// antiLoopStaleAge is when a leftover state file (a session whose last run
	// failed) is deleted by a later write; antiLoopSweepCap bounds that sweep.
	antiLoopStaleAge = 24 * time.Hour
	antiLoopSweepCap = 64
)

type antiLoopState struct {
	LastID string `json:"last_id"`
	Count  int    `json:"count"`
	At     int64  `json:"at"` // unix seconds of the last failure
}

// checkAntiLoop tracks consecutive identical failing CCR digests per session
// and warns on stderr when the same failure repeats at least antiLoopThreshold
// times within antiLoopWindow. A run with no session is not tracked: sessionless
// runs share nothing that would tell them apart. All disk errors are ignored
// (fail-open).
//
// The read-modify-write is unlocked: parallel runs in one session can only
// undercount (a missed warning), never warn falsely, so no lock is taken.
func checkAntiLoop(stderr io.Writer, session string, exitCode int, keptID string) {
	checkAntiLoopAt(stderr, session, exitCode, keptID, time.Now())
}

// checkAntiLoopAt is checkAntiLoop with the clock passed in.
func checkAntiLoopAt(stderr io.Writer, session string, exitCode int, keptID string, now time.Time) {
	ref := SessionRef(session)
	if ref == "" {
		return
	}
	path, err := paths.Join(antiLoopPrefix + ref + ".json")
	if err != nil {
		return
	}
	if exitCode == 0 {
		// Stat first: the common passing run with no streak touches nothing.
		if _, err := os.Lstat(path); err == nil {
			_ = os.Remove(path)
		}
		return
	}
	if keptID == "" {
		return
	}
	st := readAntiLoopState(path)
	if st.LastID == keptID && now.Sub(time.Unix(st.At, 0)) <= antiLoopWindow {
		st.Count++
	} else {
		st = antiLoopState{LastID: keptID, Count: 1}
	}
	st.At = now.Unix()
	writeAntiLoopState(path, st)
	sweepStaleAntiLoop(filepath.Dir(path), path, now)
	if st.Count >= antiLoopThreshold && stderr != nil {
		fmt.Fprintf(stderr, "⚠️ [downshift ccr] Identical failure detected %dx (digest: %s). Context loop detected. Check approach or review previous run with 'downshift retrieve --session=%s %s'.\n", st.Count, keptID, ref, keptID)
	}
}

func readAntiLoopState(path string) antiLoopState {
	if _, err := paths.RegularFile(path); err != nil {
		return antiLoopState{}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return antiLoopState{}
	}
	var st antiLoopState
	if json.Unmarshal(data, &st) != nil || st.Count < 0 {
		return antiLoopState{}
	}
	return st
}

func writeAntiLoopState(path string, st antiLoopState) {
	dir := filepath.Dir(path)
	if err := paths.PrivateDir(dir); err != nil {
		return
	}
	data, err := json.Marshal(st)
	if err != nil {
		return
	}
	tmp, err := os.CreateTemp(dir, ".ccr-loop-*.tmp")
	if err != nil {
		return
	}
	name := tmp.Name()
	_, werr := tmp.Write(data)
	if cerr := tmp.Close(); werr == nil {
		werr = cerr
	}
	if werr == nil {
		werr = os.Chmod(name, paths.FilePerm)
	}
	if werr == nil {
		werr = os.Rename(name, path)
	}
	if werr != nil {
		_ = os.Remove(name)
	}
}

// sweepStaleAntiLoop deletes ccr-loop-*.json files untouched for
// antiLoopStaleAge, checking the age of at most antiLoopSweepCap of them. It
// runs only on a failing run's write, never deletes keep (the file just
// written) and ignores every error.
func sweepStaleAntiLoop(dir, keep string, now time.Time) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	checked := 0
	for _, e := range entries {
		name := e.Name()
		if !e.Type().IsRegular() || !strings.HasPrefix(name, antiLoopPrefix) || !strings.HasSuffix(name, ".json") {
			continue
		}
		p := filepath.Join(dir, name)
		if p == keep {
			continue
		}
		if checked++; checked > antiLoopSweepCap {
			return
		}
		fi, err := e.Info()
		if err != nil || now.Sub(fi.ModTime()) < antiLoopStaleAge {
			continue
		}
		_ = os.Remove(p)
	}
}
