// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/tiagovilasboas/downshift-ai/internal/paths"
)

const antiLoopThreshold = 3

type antiLoopState struct {
	LastID string `json:"last_id"`
	Count  int    `json:"count"`
}

// checkAntiLoop tracks consecutive identical failing CCR digests per session
// and warns on stderr when the same failure repeats at least antiLoopThreshold
// times. All disk errors are ignored (fail-open).
func checkAntiLoop(stderr io.Writer, session string, exitCode int, keptID string) {
	path, ok := antiLoopPath(session)
	if !ok {
		return
	}
	if exitCode == 0 {
		_ = os.Remove(path)
		return
	}
	if keptID == "" {
		return
	}
	st := readAntiLoopState(path)
	if st.LastID == keptID {
		st.Count++
	} else {
		st = antiLoopState{LastID: keptID, Count: 1}
	}
	writeAntiLoopState(path, st)
	if st.Count >= antiLoopThreshold && stderr != nil {
		fmt.Fprintf(stderr, "⚠️ [downshift ccr] Identical failure detected %dx (digest: %s). Context loop detected. Check approach or review previous run with 'downshift retrieve %s'.\n", st.Count, keptID, keptID)
	}
}

func antiLoopPath(session string) (string, bool) {
	ref := SessionRef(session)
	if ref == "" {
		ref = "default"
	}
	p, err := paths.Join("ccr-loop-" + ref + ".json")
	if err != nil {
		return "", false
	}
	return p, true
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
