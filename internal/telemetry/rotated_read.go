// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"os"
	"path/filepath"
)

// DefaultRotatedMaxBytes bounds total bytes read across rotated backups and
// the live event log by ReadEvents.
const DefaultRotatedMaxBytes int64 = 64 << 20

type logSlice struct {
	path     string
	maxBytes int64
}

// ReadEventsWithRotated reads events from rotated backups and the default
// event log in chronological order, bounded to at most maxBytes of raw log
// data (keeping the newest events when total size exceeds maxBytes). A
// non-positive maxBytes reads without a byte cap.
func ReadEventsWithRotated(maxBytes int64) ([]Event, error) {
	return ReadEventsWithRotatedFrom(defaultEventPath(), maxBytes)
}

// ReadEventsWithRotatedFrom is ReadEventsWithRotated for an explicit log path.
func ReadEventsWithRotatedFrom(path string, maxBytes int64) ([]Event, error) {
	slices, err := collectLogSlices(path, maxBytes)
	if err != nil {
		return nil, err
	}
	if len(slices) == 0 {
		return nil, nil
	}
	var events []Event
	for _, s := range slices {
		part, err := readEventsTailFrom(s.path, s.maxBytes, parseEventsRaw)
		events = append(events, part...)
		if err != nil {
			linkLateUsage(events)
			return events, err
		}
	}
	linkLateUsage(events)
	return events, nil
}

// collectLogSlices returns the log files to read in chronological order
// (oldest backup first, live log last), trimming or skipping older files
// once maxBytes is reached.
func collectLogSlices(path string, maxBytes int64) ([]logSlice, error) {
	backups, err := listBackups(path)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	dir := filepath.Dir(path)
	candidates := make([]string, 0, len(backups)+1)
	for _, b := range backups {
		candidates = append(candidates, filepath.Join(dir, b.Name()))
	}
	candidates = append(candidates, path)

	if maxBytes <= 0 {
		out := make([]logSlice, len(candidates))
		for i, c := range candidates {
			out[i] = logSlice{path: c}
		}
		return out, nil
	}

	remaining := maxBytes
	var rev []logSlice
	for i := len(candidates) - 1; i >= 0 && remaining > 0; i-- {
		info, err := os.Stat(candidates[i])
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		size := info.Size()
		if size == 0 {
			continue
		}
		if size <= remaining {
			rev = append(rev, logSlice{path: candidates[i]})
			remaining -= size
			continue
		}
		rev = append(rev, logSlice{path: candidates[i], maxBytes: remaining})
		remaining = 0
	}

	// Reverse so the returned slices are in chronological order.
	for i, j := 0, len(rev)-1; i < j; i, j = i+1, j-1 {
		rev[i], rev[j] = rev[j], rev[i]
	}
	return rev, nil
}
