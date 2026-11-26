// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/paths"
)

// The panel carries the running binary's version (set at build time by
// -X main.buildVersion), so the agents band shows the release in use and never
// a number written into the mod.
func TestPanel_CarriesBuildVersion(t *testing.T) {
	panelFixtureEnv(t)

	for _, opt := range []panelOptions{{NoReport: true}, {}} {
		var out, errOut bytes.Buffer
		if rc := writePanel(nil, "abc", opt, &out, &errOut); rc != 0 {
			t.Fatalf("writePanel(%+v) rc=%d: %s", opt, rc, errOut.String())
		}
		var got struct {
			Version string `json:"version"`
		}
		if err := json.Unmarshal(out.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if got.Version == "" {
			t.Fatalf("panel %+v has no version: %s", opt, out.String())
		}
	}
}

func writeLatestCacheFixture(t *testing.T, raw []byte) string {
	t.Helper()
	p, err := paths.Join("latest_release.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestPanel_UpdateNoticeFromCache(t *testing.T) {
	now := time.Now().UTC()
	freshRaw, err := json.Marshal(latestCache{Tag: "v0.24.0", CheckedAt: now.Add(-1 * time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	staleRaw, err := json.Marshal(latestCache{Tag: "v0.24.0", CheckedAt: now.Add(-25 * time.Hour)})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("newer cached release sets latest and update_available on both panel shapes", func(t *testing.T) {
		panelFixtureEnv(t)
		writeLatestCacheFixture(t, freshRaw)

		for _, opt := range []panelOptions{{NoReport: true}, {}} {
			var out, errOut bytes.Buffer
			if rc := writePanelWithVersion(nil, "abc", opt, "v0.23.0", &out, &errOut); rc != 0 {
				t.Fatalf("writePanelWithVersion(%+v) rc=%d: %s", opt, rc, errOut.String())
			}
			var got struct {
				Version         string `json:"version"`
				Latest          string `json:"latest"`
				UpdateAvailable bool   `json:"update_available"`
			}
			if err := json.Unmarshal(out.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if got.Version != "v0.23.0" || got.Latest != "v0.24.0" || !got.UpdateAvailable {
				t.Fatalf("opt=%+v got %+v, raw=%s", opt, got, out.String())
			}
		}
	})

	cases := []struct {
		name    string
		current string
		raw     []byte
	}{
		{"current equals latest", "v0.24.0", freshRaw},
		{"current newer than latest", "v0.25.0", freshRaw},
		{"dev build", "dev", freshRaw},
		{"stale cache", "v0.23.0", staleRaw},
		{"corrupt cache", "v0.23.0", []byte("{not json")},
		{"missing cache", "v0.23.0", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			panelFixtureEnv(t)
			if tc.raw != nil {
				writeLatestCacheFixture(t, tc.raw)
			}
			for _, opt := range []panelOptions{{NoReport: true}, {}} {
				var out, errOut bytes.Buffer
				if rc := writePanelWithVersion(nil, "abc", opt, tc.current, &out, &errOut); rc != 0 {
					t.Fatalf("writePanelWithVersion(%+v) rc=%d: %s", opt, rc, errOut.String())
				}
				var rawMap map[string]any
				if err := json.Unmarshal(out.Bytes(), &rawMap); err != nil {
					t.Fatal(err)
				}
				if _, ok := rawMap["latest"]; ok {
					t.Fatalf("%s (%+v): expected latest omitted, got %v", tc.name, opt, rawMap["latest"])
				}
				if _, ok := rawMap["update_available"]; ok {
					t.Fatalf("%s (%+v): expected update_available omitted, got %v", tc.name, opt, rawMap["update_available"])
				}
			}
		})
	}
}

func TestPanel_CachedUpdateNoticeFromIsPureOffline(t *testing.T) {
	var spawns atomic.Int32
	old := latestRefreshSpawn
	latestRefreshSpawn = func() error { spawns.Add(1); return nil }
	t.Cleanup(func() { latestRefreshSpawn = old })

	cache := filepath.Join(t.TempDir(), "latest_release.json")
	if tag, ok := cachedUpdateNoticeFrom(cache, "v0.23.0", time.Now); ok || tag != "" {
		t.Fatalf("missing cache got %q, %v", tag, ok)
	}
	if spawns.Load() != 0 {
		t.Fatalf("cachedUpdateNoticeFrom must never spawn refresh, got %d", spawns.Load())
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(cache), latestAttemptFile)); !os.IsNotExist(err) {
		t.Fatalf("cachedUpdateNoticeFrom must not touch attempt file, err=%v", err)
	}
}

func TestPanel_DetachedRefreshTriggeredAndThrottledByCooldown(t *testing.T) {
	panelFixtureEnv(t)
	t.Setenv("DOWNSHIFT_NO_NETWORK", "")

	var spawns atomic.Int32
	old := latestRefreshSpawn
	latestRefreshSpawn = func() error { spawns.Add(1); return nil }
	t.Cleanup(func() { latestRefreshSpawn = old })

	cache, err := paths.Join("latest_release.json")
	if err != nil {
		t.Fatal(err)
	}

	// Missing cache triggers one detached refresh, still returning ("", false).
	if tag, ok := cachedUpdateNotice("v0.23.0"); ok || tag != "" {
		t.Fatalf("missing cache got %q, %v", tag, ok)
	}
	if spawns.Load() != 1 {
		t.Fatalf("first missing cache spawns=%d, want 1", spawns.Load())
	}

	// Second call within cooldown window must not respawn.
	if tag, ok := cachedUpdateNotice("v0.23.0"); ok || tag != "" {
		t.Fatalf("within cooldown got %q, %v", tag, ok)
	}
	if spawns.Load() != 1 {
		t.Fatalf("within cooldown spawns=%d, want 1", spawns.Load())
	}

	// After cooldown expires, maybeRefreshLatestCache spawns again.
	now := time.Now().UTC().Add(latestRefreshCooldown + time.Minute)
	maybeRefreshLatestCache(cache, func() time.Time { return now })
	if spawns.Load() != 2 {
		t.Fatalf("after cooldown spawns=%d, want 2", spawns.Load())
	}

	// Fresh cache never triggers a refresh even without cooldown.
	freshRaw, _ := json.Marshal(latestCache{Tag: "v0.24.0", CheckedAt: time.Now().UTC()})
	writeLatestCacheFixture(t, freshRaw)
	_ = os.Remove(filepath.Join(filepath.Dir(cache), latestAttemptFile))
	if tag, ok := cachedUpdateNotice("v0.24.0"); ok || tag != "" {
		t.Fatalf("up-to-date fresh cache got %q, %v", tag, ok)
	}
	if spawns.Load() != 2 {
		t.Fatalf("fresh cache must not spawn, spawns=%d", spawns.Load())
	}
}

func TestPanel_DetachedRefreshSuppressedByNoNetwork(t *testing.T) {
	panelFixtureEnv(t)
	t.Setenv("DOWNSHIFT_NO_NETWORK", "1")

	var spawns atomic.Int32
	old := latestRefreshSpawn
	latestRefreshSpawn = func() error { spawns.Add(1); return nil }
	t.Cleanup(func() { latestRefreshSpawn = old })

	if tag, ok := cachedUpdateNotice("v0.23.0"); ok || tag != "" {
		t.Fatalf("got %q, %v", tag, ok)
	}
	if spawns.Load() != 0 {
		t.Fatalf("DOWNSHIFT_NO_NETWORK=1 must suppress refresh, got %d", spawns.Load())
	}
	cache, err := paths.Join("latest_release.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(cache), latestAttemptFile)); !os.IsNotExist(err) {
		t.Fatalf("DOWNSHIFT_NO_NETWORK=1 must not create attempt file, err=%v", err)
	}
}
