// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/contextopt"
	"github.com/tiagovilasboas/downshift-ai/internal/core"
	"github.com/tiagovilasboas/downshift-ai/internal/paths"
	"github.com/tiagovilasboas/downshift-ai/internal/semantic"
)

func checkCatalog(rep *doctorReport, env doctorEnv) {
	catalogPath, _ := paths.CatalogPath()
	if fileExists(catalogPath) {
		rep.add(doctorCheck{ID: "catalog", Status: doctorOK, Detail: "user override at " + catalogPath})
		return
	}
	rep.add(doctorCheck{ID: "catalog", Status: doctorOK, Detail: "embedded default (override path " + catalogPath + ")"})
}

func checkContext(rep *doctorReport, env doctorEnv) {
	optCfg, _ := contextopt.LoadConfig()
	if optCfg.Enabled {
		detail := "enabled (provider: " + optCfg.Provider + ")"
		if optCfg.Provider == "native" {
			detail += "; observe-only: its hook measures bytes and leaves tool output as is (Claude Code PostToolUse could replace output with updatedToolOutput; Codex PostToolUse cannot)"
		}
		rep.add(doctorCheck{ID: "context optim.", Status: doctorOK, Detail: detail})
	} else {
		rep.add(doctorCheck{ID: "context optim.", Status: doctorOK, Detail: "disabled (enable with `downshift context enable`)"})
	}
	checkCompactorMode(rep)
	checkPostRunReplacementFromLog(rep)
	checkCompanion(rep, env, optCfg)
	if env.Verbose {
		if semantic.Enabled() {
			rep.add(doctorCheck{ID: "semantic boost", Status: doctorOK, Detail: "in-process FNV hash embedder (on by default)"})
		} else {
			rep.add(doctorCheck{ID: "semantic boost", Status: doctorOK, Detail: "disabled (DOWNSHIFT_SEMANTIC_BOOST=0)"})
		}
	}
}

// checkCompanion reports every registered companion: recorded and present is
// ok; recorded but missing, or present but not recorded, is a warn with the
// exact fix. Absent and not recorded prints only in verbose: it is optional.
// The providers come from env.Companions, so doctor names none of them.
func checkCompanion(rep *doctorReport, env doctorEnv, optCfg contextopt.Config) {
	if env.Companions == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	for _, c := range env.Companions() {
		id := c.ID()
		installed, ver, _, err := c.Detect(ctx)
		recorded := optCfg.Companion == id
		label := id
		if ver != "" {
			label += " v" + ver
		}
		switch {
		case err != nil:
			rep.add(doctorCheck{ID: "companion", Status: doctorWarn, Detail: err.Error(), Fix: "run `" + id + " --version` by hand, then `downshift context doctor`"})
		case recorded && installed:
			rep.add(doctorCheck{ID: "companion", Status: doctorOK, Detail: label + " recorded; wrap each harness with its own command (Downshift cannot see whether a session is wrapped)"})
		case recorded:
			rep.add(doctorCheck{ID: "companion", Status: doctorWarn, Detail: id + " recorded but not on PATH", Fix: "downshift context disable " + id})
		case installed:
			rep.add(doctorCheck{ID: "companion", Status: doctorWarn, Detail: label + " detected, not recorded (savings would be attributed to routing alone)", Fix: "downshift context enable " + id})
		case env.Verbose:
			rep.add(doctorCheck{ID: "companion", Status: doctorOK, Detail: "none (optional: " + id + ", docs/guides/companions.md)"})
		}
	}
}

// modelOverrideNotes reports every declared model-override variable that is
// set. A harness-level model setting can take precedence over a hook's model
// rewrite, which would make a Downshift rewrite a silent no-op. The precedence
// comes from outside sources and is not verified here, so the wording stays
// conditional. Nothing about routing changes. Adding a harness's variable is a
// one-line capability change; doctor needs no edit.
func modelOverrideNotes(caps []core.NamedCaps, getenv func(string) string) []string {
	var notes []string
	for _, c := range caps {
		name := c.Caps.ModelOverrideEnv
		if name == "" {
			continue
		}
		v := strings.TrimSpace(getenv(name))
		if v == "" {
			continue
		}
		if len(v) > 64 {
			v = v[:64] + "..."
		}
		notes = append(notes, fmt.Sprintf("model_override_env: %s=%s; %s may apply it instead of a Downshift rewrite (not verified here). Check the resolved model with the post-tool-use hook.", name, v, c.Harness))
	}
	return notes
}
