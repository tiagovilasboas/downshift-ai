// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package main

import (
	"bytes"
	"encoding/json"
	"testing"
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
