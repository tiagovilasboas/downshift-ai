// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/tiagovilasboas/downshift-ai/internal/lazyre"
)

// KiroCLI runs `kiro-cli chat --list-models --format json`, which reports the
// models the Kiro account can select. "auto" is Kiro's own router, not a
// model, and is skipped.
type KiroCLI struct{ Bin string }

func (KiroCLI) Harness() string { return "kirocrew" }
func (KiroCLI) Name() string    { return "kiro-cli chat --list-models" }

func (k KiroCLI) Discover(ctx context.Context) ([]Model, error) {
	bin := k.Bin
	if bin == "" {
		bin = defaultKiroCLI()
	}
	// The list command opens a sign-in page when the CLI has no session in
	// this context, so it runs only after whoami confirms one. Any other
	// binary, including the PATH name, is checked the same way.
	if !signedInKiroCLI(bin) {
		return nil, errKiroNotSignedIn
	}
	out, err := runCLI(ctx, bin, "chat", "--list-models", "--format", "json")
	if err != nil {
		return nil, err
	}
	return parseKiroList(out)
}

func parseKiroList(data []byte) ([]Model, error) {
	var f struct {
		Models []struct {
			ID   string `json:"model_id"`
			Name string `json:"model_name"`
		} `json:"models"`
	}
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("kiro list: %w", err)
	}
	var out []Model
	for _, m := range f.Models {
		if m.ID == "" || m.ID == "auto" {
			continue
		}
		out = append(out, Model{ID: m.ID, Display: m.Name})
	}
	if len(out) == 0 {
		return nil, errors.New("kiro list: no models")
	}
	return out, nil
}

// GrokCLI runs `grok models`, which lists without authentication.
type GrokCLI struct{ Bin string }

func (GrokCLI) Harness() string { return "grok" }
func (GrokCLI) Name() string    { return "grok models" }

func (g GrokCLI) Discover(ctx context.Context) ([]Model, error) {
	bin := g.Bin
	if bin == "" {
		bin = "grok"
	}
	out, err := runCLI(ctx, bin, "models")
	if err != nil {
		return nil, err
	}
	return parseGrokList(out)
}

var grokLine = lazyre.New(`^\s*[*-]\s+(\S+)`)

// parseGrokList reads the "Available models:" block: lines like
// "  * grok-4.6 (default)" and "  - grok-4.5".
func parseGrokList(data []byte) ([]Model, error) {
	var out []Model
	inBlock := false
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(strings.TrimSpace(line), "Available models") {
			inBlock = true
			continue
		}
		if !inBlock {
			continue
		}
		if m := grokLine.FindStringSubmatch(line); m != nil {
			out = append(out, Model{ID: m[1]})
		}
	}
	if len(out) == 0 {
		return nil, errors.New("grok list: no models")
	}
	return out, nil
}
