// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// The latest-release lookup behind `doctor --check-latest`. It is the only
// network call doctor can make, it runs only on that flag, and never from a
// hook. Any failure is "unknown", never a failed check.

const (
	latestReleaseURL      = "https://api.github.com/repos/tiagovilasboas/downshift-ai/releases/latest"
	latestCacheTTL        = 24 * time.Hour
	latestRefreshCooldown = 1 * time.Hour
	latestAttemptFile     = "latest_release.attempt"
	latestTimeout         = 2 * time.Second
)

var (
	latestRefreshSpawn        = spawnLatestRefresh
	allowLatestRefreshInTests bool
)

// latestLookup asks the releases API for the newest tag, caching the answer in
// CacheFile for latestCacheTTL. Client and URL are injectable for tests.
type latestLookup struct {
	Client    *http.Client
	URL       string
	CacheFile string
	Now       func() time.Time
}

type latestCache struct {
	Tag       string    `json:"tag"`
	CheckedAt time.Time `json:"checked_at"`
}

func defaultLatestLookup(cacheFile string) latestLookup {
	return latestLookup{Client: &http.Client{Timeout: latestTimeout},
		URL: latestReleaseURL, CacheFile: cacheFile, Now: time.Now}
}

// cachedLatest returns the cached release tag if present and still fresh.
// It never makes a network request.
func (l latestLookup) cachedLatest() (string, bool) {
	if c, ok := l.readCache(); ok && fresh(l.Now().Sub(c.CheckedAt)) {
		return c.Tag, true
	}
	return "", false
}

// Latest returns the newest release tag and whether it is known.
func (l latestLookup) Latest() (string, bool) {
	if tag, ok := l.cachedLatest(); ok {
		return tag, true
	}
	tag, ok := l.fetch()
	if !ok {
		return "", false
	}
	l.writeCache(latestCache{Tag: tag, CheckedAt: l.Now()})
	return tag, true
}

func (l latestLookup) fetch() (string, bool) {
	req, err := http.NewRequest(http.MethodGet, l.URL, nil)
	if err != nil {
		return "", false
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := l.Client.Do(req)
	if err != nil {
		return "", false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", false
	}
	var doc struct {
		Tag string `json:"tag_name"`
	}
	if json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&doc) != nil || doc.Tag == "" {
		return "", false
	}
	return doc.Tag, true
}

func (l latestLookup) readCache() (latestCache, bool) {
	raw, err := os.ReadFile(l.CacheFile)
	if err != nil {
		return latestCache{}, false
	}
	var c latestCache
	if json.Unmarshal(raw, &c) != nil || c.Tag == "" {
		return latestCache{}, false
	}
	return c, true
}

// writeCache is best effort: a read-only state dir only costs a refetch.
func (l latestLookup) writeCache(c latestCache) {
	raw, err := json.Marshal(c)
	if err != nil {
		return
	}
	if os.MkdirAll(filepath.Dir(l.CacheFile), 0o700) != nil {
		return
	}
	_ = os.WriteFile(l.CacheFile, raw, 0o600)
}

// fresh reports whether a cache entry of this age is still usable. A negative
// age (a clock moved back, or a cache from the future) is not fresh.
func fresh(age time.Duration) bool { return age >= 0 && age < latestCacheTTL }

// maybeRefreshLatestCache triggers a throttled, detached background refresh of
// latest_release.json when the cache is missing or expired. It is fail-open and
// never blocks or makes inline HTTP requests.
func maybeRefreshLatestCache(cacheFile string, now func() time.Time) {
	defer func() { _ = recover() }()
	if os.Getenv("DOWNSHIFT_NO_NETWORK") == "1" || latestRefreshSpawn == nil {
		return
	}
	if now == nil {
		now = time.Now
	}
	attemptPath := filepath.Join(filepath.Dir(cacheFile), latestAttemptFile)
	ok, err := acquireLatestAttempt(attemptPath, now())
	if err != nil || !ok {
		return
	}
	_ = latestRefreshSpawn()
}

// acquireLatestAttempt atomically claims the cooldown marker for
// latestRefreshCooldown, so offline or frequent panel ticks do not respawn.
func acquireLatestAttempt(path string, now time.Time) (bool, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return false, err
	}
	for attempt := 0; attempt < 2; attempt++ {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			_, werr := fmt.Fprintf(f, "%s\n", now.UTC().Format(time.RFC3339Nano))
			cerr := f.Close()
			return werr == nil && cerr == nil, nil
		}
		if !errors.Is(err, fs.ErrExist) {
			return false, err
		}
		var ts time.Time
		var haveTS bool
		if raw, rerr := os.ReadFile(path); rerr == nil {
			if parsed, perr := time.Parse(time.RFC3339Nano, strings.TrimSpace(string(raw))); perr == nil {
				ts, haveTS = parsed, true
			}
		}
		if !haveTS {
			fi, serr := os.Stat(path)
			if errors.Is(serr, fs.ErrNotExist) {
				continue
			}
			if serr == nil {
				ts, haveTS = fi.ModTime(), true
			}
		}
		if haveTS {
			if age := now.Sub(ts); age >= 0 && age < latestRefreshCooldown {
				return false, nil
			}
		}
		if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return false, err
		}
	}
	return false, nil
}

// spawnLatestRefresh runs `downshift doctor --refresh-latest-cache` detached
// with null-device stdio. It is disabled under `go test` unless enabled.
func spawnLatestRefresh() error {
	if flag.Lookup("test.v") != nil && !allowLatestRefreshInTests {
		return nil
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command(exe, "doctor", "--refresh-latest-cache")
	cmd.Stdin, cmd.Stdout, cmd.Stderr = nil, nil, nil
	detachProcess(cmd)
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}
