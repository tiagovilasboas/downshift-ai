// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// LiveState is the file a panel reads: the last spawn plus per-state counts for
// that session. It is rewritten atomically on every spawn.
type LiveState struct {
	Updated string         `json:"updated"`
	Session string         `json:"session,omitempty"`
	Counts  map[string]int `json:"counts"`
	Last    SpawnRecord    `json:"last"`
}

// Active reports whether the last spawn is within ActiveWindow of now.
func (l LiveState) Active(now time.Time) bool {
	t, err := time.Parse(time.RFC3339Nano, l.Updated)
	return err == nil && now.Sub(t) < ActiveWindow && now.Sub(t) > -time.Minute
}

// LivePath is live.json next to the event log, so DOWNSHIFT_EVENT_LOG moves
// both together.
func LivePath() string {
	return filepath.Join(filepath.Dir(defaultEventPath()), "live.json")
}

// WriteLive folds rec into live.json. Counts reset when the session changes.
// It is best effort: the caller ignores the error. A busy lock can cost the
// spawn up to lockTimeout (500 ms), after which the record is skipped.
func WriteLive(rec SpawnRecord) error {
	path := LivePath()
	lock, err := LockFile(path, lockTimeout)
	if err != nil {
		return err
	}
	defer lock.Close()
	cur, _ := ReadLive()
	if cur.Counts == nil || cur.Session != rec.Session {
		cur = LiveState{Counts: map[string]int{}}
	}
	cur.Session, cur.Last = rec.Session, rec
	cur.Counts[rec.State]++
	cur.Updated = time.Now().UTC().Format(time.RFC3339Nano)
	data, err := json.Marshal(cur)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// maxLiveBytes caps what ReadLive accepts; a real record is under 1 KB.
const maxLiveBytes = 64 << 10

// ReadLive reads live.json; a missing, oversized or unreadable file is the zero value.
func ReadLive() (LiveState, error) {
	var l LiveState
	f, err := os.Open(LivePath())
	if err != nil {
		return l, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxLiveBytes+1))
	if err != nil {
		return l, err
	}
	if len(data) > maxLiveBytes {
		return l, errors.New("live.json too large")
	}
	err = json.Unmarshal(data, &l)
	return l, err
}

// ActivityLine says whether Downshift is working for the session, from its
// event log: the last spawn's state and age, or that nothing was routed yet.
func ActivityLine(events []Event, sessionHash string, now time.Time) string {
	var last *Event
	for _, ev := range sessionEvents(events, sessionHash) {
		if IsDecisionRow(ev) {
			e := ev
			last = &e
		}
	}
	if last == nil {
		return "○ downshift on · no subagent routed yet"
	}
	age := now.Sub(evTime(*last)).Round(time.Second)
	if age < 0 {
		age = 0
	}
	rec := NewSpawnRecord(*last)
	if age < ActiveWindow {
		return fmt.Sprintf("● downshift active · last spawn %s ago: %s", age, rec.Line())
	}
	return fmt.Sprintf("○ downshift idle · last spawn %s ago: %s", age, rec.Line())
}
