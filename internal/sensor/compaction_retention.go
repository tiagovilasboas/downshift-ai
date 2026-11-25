// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package sensor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

func retainCompaction(path string, now time.Time) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	kept, nonEmpty := filterCompactionLines(raw, now)
	var overflow [][]byte
	if len(kept) > compactionMaxRecords {
		cut := len(kept) - compactionMaxRecords
		overflow = kept[:cut]
		kept = kept[cut:]
	}
	if len(overflow) == 0 && len(kept) == nonEmpty {
		return nil
	}
	if len(overflow) > 0 {
		if err := appendCompactionArchive(path+".1", overflow, now); err != nil {
			return err
		}
	}
	return writeCompactionLines(path, kept, now)
}

func filterCompactionLines(raw []byte, now time.Time) ([][]byte, int) {
	lines := bytes.Split(raw, []byte("\n"))
	kept := make([][]byte, 0, len(lines))
	nonEmpty := 0
	for _, line := range lines {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		nonEmpty++
		var rec CompactionRecord
		if err := json.Unmarshal(line, &rec); err != nil {
			kept = append(kept, line)
			continue
		}
		ts, err := time.Parse(time.RFC3339Nano, rec.Timestamp)
		if err == nil && now.Sub(ts) > compactionMaxAge {
			continue
		}
		kept = append(kept, line)
	}
	return kept, nonEmpty
}

func appendCompactionArchive(archivePath string, overflow [][]byte, now time.Time) error {
	var existing [][]byte
	if raw, err := os.ReadFile(archivePath); err == nil {
		existing, _ = filterCompactionLines(raw, now)
	} else if !os.IsNotExist(err) {
		return err
	}
	combined := append(existing, overflow...)
	if len(combined) > compactionMaxRecords {
		combined = combined[len(combined)-compactionMaxRecords:]
	}
	return writeCompactionLines(archivePath, combined, now)
}

func writeCompactionLines(path string, lines [][]byte, now time.Time) error {
	var payload []byte
	if len(lines) > 0 {
		payload = append(bytes.Join(lines, []byte("\n")), '\n')
	}
	tmp := fmt.Sprintf("%s.%d.%d.tmp", path, os.Getpid(), now.UnixNano())
	if err := os.WriteFile(tmp, payload, 0600); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}
