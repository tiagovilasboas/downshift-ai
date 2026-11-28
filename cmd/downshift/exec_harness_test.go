// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/core"
	"github.com/tiagovilasboas/downshift-ai/internal/execwrap"
	"github.com/tiagovilasboas/downshift-ai/internal/paths"
)

func mapGetenv(vals map[string]string) func(string) string {
	return func(k string) string { return vals[k] }
}

func TestInferExecHarness(t *testing.T) {
	t.Run("zero matching env vars returns empty", func(t *testing.T) {
		if got := inferExecHarness(mapGetenv(nil)); got != "" {
			t.Fatalf("got %q, want empty", got)
		}
	})

	var withSession []core.NamedCaps
	for _, n := range core.AllHarnessCaps() {
		if n.Caps.SessionEnv != "" {
			withSession = append(withSession, n)
			t.Run("single SessionEnv/"+n.Harness, func(t *testing.T) {
				got := inferExecHarness(mapGetenv(map[string]string{n.Caps.SessionEnv: "sess-1"}))
				if got != n.Harness {
					t.Fatalf("got %q, want %q", got, n.Harness)
				}
			})
		}
		if n.Caps.ShellEnv != "" {
			t.Run("single ShellEnv/"+n.Harness, func(t *testing.T) {
				got := inferExecHarness(mapGetenv(map[string]string{n.Caps.ShellEnv: "shell-1"}))
				if got != n.Harness {
					t.Fatalf("got %q, want %q", got, n.Harness)
				}
			})
		}
		if n.Caps.SessionEnv != "" && n.Caps.ShellEnv != "" {
			t.Run("both SessionEnv and ShellEnv of same harness/"+n.Harness, func(t *testing.T) {
				got := inferExecHarness(mapGetenv(map[string]string{
					n.Caps.SessionEnv: "sess-1",
					n.Caps.ShellEnv:   "shell-1",
				}))
				if got != n.Harness {
					t.Fatalf("got %q, want %q", got, n.Harness)
				}
			})
		}
	}

	if len(withSession) < 2 {
		t.Fatalf("need at least 2 harnesses with SessionEnv to test ambiguity, got %d", len(withSession))
	}
	t.Run("multiple matching SessionEnvs returns empty", func(t *testing.T) {
		got := inferExecHarness(mapGetenv(map[string]string{
			withSession[0].Caps.SessionEnv: "sess-a",
			withSession[1].Caps.SessionEnv: "sess-b",
		}))
		if got != "" {
			t.Fatalf("got %q, want empty when %s and %s are both set",
				got, withSession[0].Harness, withSession[1].Harness)
		}
	})
}

func TestRunExec_HarnessInference(t *testing.T) {
	var withSession []core.NamedCaps
	for _, n := range core.AllHarnessCaps() {
		if n.Caps.SessionEnv != "" {
			withSession = append(withSession, n)
		}
	}
	if len(withSession) < 2 {
		t.Fatalf("need at least 2 harnesses with SessionEnv, got %d", len(withSession))
	}

	cases := []struct {
		name        string
		args        []string
		envs        map[string]string
		wantHarness string
	}{
		{
			name:        "explicit --harness wins over SessionEnv",
			args:        []string{"--harness=cursor", "--"},
			envs:        map[string]string{withSession[0].Caps.SessionEnv: "sess-1"},
			wantHarness: "cursor",
		},
		{
			name:        "single matching SessionEnv infers harness",
			args:        []string{"--"},
			envs:        map[string]string{withSession[1].Caps.SessionEnv: "sess-2"},
			wantHarness: withSession[1].Harness,
		},
		{
			name:        "zero matching SessionEnvs falls back to unknown",
			args:        []string{"--"},
			envs:        nil,
			wantHarness: "unknown",
		},
		{
			name: "multiple matching SessionEnvs do not misattribute",
			args: []string{"--"},
			envs: map[string]string{
				withSession[0].Caps.SessionEnv: "sess-1",
				withSession[1].Caps.SessionEnv: "sess-2",
			},
			wantHarness: "unknown",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(paths.EnvStateDir, t.TempDir())
			t.Setenv(execwrap.DepthEnv, "")
			clearHarnessEnv(t)
			for k, v := range tc.envs {
				t.Setenv(k, v)
			}

			var out, errb bytes.Buffer
			runExec(append(tc.args, "downshift-test-no-such-program"), strings.NewReader(""), &out, &errb)

			path, err := execwrap.EventsPath()
			if err != nil {
				t.Fatal(err)
			}
			evs, err := execwrap.ReadEvents(path)
			if err != nil || len(evs) != 1 {
				t.Fatalf("events = %+v, err = %v", evs, err)
			}
			if evs[0].Harness != tc.wantHarness {
				t.Errorf("harness = %q, want %q", evs[0].Harness, tc.wantHarness)
			}
		})
	}
}
