// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// maxOutput bounds what a source reads from a CLI or file.
const maxOutput = 4 << 20

// CodexCache reads the model list Codex keeps for the signed-in account
// (~/.codex/models_cache.json). Only entries Codex itself shows in its picker
// (visibility "list") are reported.
type CodexCache struct{ Path string }

func (CodexCache) Harness() string { return "codex" }
func (CodexCache) Name() string    { return "codex models_cache.json" }

func (c CodexCache) Discover(context.Context) ([]Model, error) {
	path := c.Path
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		path = filepath.Join(home, ".codex", "models_cache.json")
	}
	data, err := readBounded(path)
	if err != nil {
		return nil, err
	}
	return parseCodexCache(data)
}

func parseCodexCache(data []byte) ([]Model, error) {
	var f struct {
		Models []struct {
			Slug       string `json:"slug"`
			Display    string `json:"display_name"`
			Visibility string `json:"visibility"`
			Tier       string `json:"tier"`
			Efforts    []struct {
				Effort string `json:"effort"`
			} `json:"supported_reasoning_levels"`
		} `json:"models"`
	}
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("codex cache: %w", err)
	}
	var out []Model
	for _, m := range f.Models {
		if m.Slug == "" || m.Visibility != "list" {
			continue
		}
		model := Model{ID: m.Slug, Display: m.Display, Tier: m.Tier}
		for _, level := range m.Efforts {
			if level.Effort != "" {
				model.Efforts = append(model.Efforts, level.Effort)
			}
		}
		out = append(out, model)
	}
	if len(out) == 0 {
		return nil, errors.New("codex cache: no listed models")
	}
	return out, nil
}

// Defaults is the source registry `models discover` and `setup` use when no
// harness is named. Every harness with a verified listing mechanism belongs
// here, whatever its maturity; a harness without one (Antigravity) is absent
// and relies on the hook payload or the operator file. The OpenRouter
// public list is added only when DOWNSHIFT_DISCOVERY_OPENROUTER=1, so the
// default run makes no new network call. Failed sources preserve their cache;
// discovery does not imply included credit.
//
// KiroFromKiroCrew comes after KiroCLI so KiroCrew is discovered first, then
// Kiro IDE mirrors it (same AWS/Kiro account, same models).
func Defaults() []Source {
	sources := []Source{AnthropicAPI{}, CodexCache{}, CursorCLI{}, GrokCLI{}, KiroCLI{}, KiroFromKiroCrew{}, OpenCodeCLI{}}
	if openRouterEnabled() {
		sources = append(sources, OpenRouterAPI{})
	}
	return sources
}
