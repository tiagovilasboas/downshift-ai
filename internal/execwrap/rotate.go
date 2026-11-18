// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"bufio"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/telemetry"
)

// rotateLockWait bounds how long an append waits for the rotation lock.
const rotateLockWait = 50 * time.Millisecond

// LogRotateBytes is the size past which an exec log (exec-events.jsonl,
// exec-skips.jsonl) rotates before the next append. Up to maxRotatedGenerations
// older generations (.1 through .6) are retained; anything older is dropped.
const LogRotateBytes = 10 << 20

const (
	rotatedSuffix         = ".1"
	maxRotatedGenerations = 6
)

// rotateMu makes the size check and the rename one step inside a process, so
// concurrent appenders cannot each rotate the fresh file over the old one.
// Across processes a race can still drop one generation, never cut a line.
var rotateMu sync.Mutex

// appendLine appends one short line (an atomic O_APPEND write), rotating the
// log first when it passed the cap. A writer that raced a rotation and still
// holds the renamed file appends to the older generation; a line is never cut
// in two. Two rotations racing may drop one generation, never corrupt a line.
func appendLine(path string, line []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	rotateIfLarge(path, LogRotateBytes)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(line, '\n'))
	return err
}

// rotateIfLarge shifts rotated generations and moves path to path+".1" when it
// is larger than limit. Best effort: a failure leaves the log growing, which
// is still correct.
func rotateIfLarge(path string, limit int64) {
	rotateWithLimit(path, limit)
}

// rotateWithLimit shifts existing generations (.5->.6 .. .1->.2) and renames
// path to path+".1" when path exceeds limit.
func rotateWithLimit(path string, limit int64) {
	rotateMu.Lock()
	defer rotateMu.Unlock()
	if !overLimit(path, limit) {
		return
	}
	lock, err := telemetry.LockFile(path, rotateLockWait)
	if err != nil {
		return // another process is rotating, or the lock is unavailable: append anyway
	}
	defer func() { _ = lock.Unlock() }()
	// Re-check under the lock: another process may have rotated already, and
	// renaming its fresh file over .1 would drop a generation.
	if !overLimit(path, limit) {
		return
	}
	for gen := maxRotatedGenerations - 1; gen >= 1; gen-- {
		src := rotatedPath(path, gen)
		if _, err := os.Stat(src); err != nil {
			continue
		}
		replaceFile(src, rotatedPath(path, gen+1))
	}
	replaceFile(path, rotatedPath(path, 1))
}

func rotatedPath(path string, gen int) string {
	return path + "." + strconv.Itoa(gen)
}

func replaceFile(src, dst string) {
	// Remove dst first so os.Rename replaces existing files on Windows as well as Unix.
	_ = os.Remove(dst)
	_ = os.Rename(src, dst)
}

func overLimit(path string, limit int64) bool {
	info, err := os.Stat(path)
	return err == nil && info.Size() > limit
}

// scanLog calls fn for every line of the log in chronological order from the
// oldest rotated generation (.6) through the newest (.1) to the current file.
// A rotation between reads can under-count for a moment, never double count.
// A missing file is empty, not an error.
func scanLog(path string, fn func(line []byte)) error {
	for gen := maxRotatedGenerations; gen >= 1; gen-- {
		if err := scanFile(rotatedPath(path, gen), fn); err != nil {
			return err
		}
	}
	return scanFile(path, fn)
}

func scanFile(path string, fn func(line []byte)) error {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fn(sc.Bytes())
	}
	return sc.Err()
}
