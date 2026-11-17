// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package headroom is the companion provider for Headroom
// (https://github.com/chopratejas/headroom), an Apache-2.0 context
// compression proxy that sits between a coding harness and the model API.
//
// Downshift does not install, launch, wrap or unwrap Headroom. It detects
// the binary on PATH, lets the user record that it is in use
// (`downshift context enable headroom`), reports it in `doctor`, and points
// to Headroom's own `headroom wrap <tool>` command per harness. Savings
// measured while a companion is active are not routing savings alone; see
// docs/guides/companions.md.
package headroom

import (
	"context"
	"os/exec"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/contextopt"
)

const (
	ProviderID   = "headroom"
	ProviderName = "Headroom (context compression proxy, companion)"
	// Binary is the command name Headroom installs (`pip install "headroom-ai[all]"`).
	Binary = "headroom"
	// InstallHint is Headroom's own documented install command.
	InstallHint = `uv tool install --python 3.13 "headroom-ai[all]"   # or: pip install "headroom-ai[all]"`
	// PrivacyNote: Headroom's anonymous beacon is on by default upstream.
	PrivacyNote = "Headroom sends an anonymous metrics beacon by default; set HEADROOM_BEACON=off or DO_NOT_TRACK=1 to turn it off. Downshift never sends anything."
	// Project is the upstream repository.
	Project = "https://github.com/chopratejas/headroom"
)

// Provider implements contextopt.Provider (and Companion) for Headroom.
type Provider struct {
	// LookPath overrides exec.LookPath in tests.
	LookPath func(file string) (string, error)
	// CommandRunner overrides exec.CommandContext(...).Output() in tests.
	CommandRunner func(ctx context.Context, name string, args ...string) ([]byte, error)
}

// NewProvider creates a Headroom provider with standard OS execution.
func NewProvider() *Provider {
	return &Provider{
		LookPath: exec.LookPath,
		CommandRunner: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			cmd := exec.CommandContext(ctx, name, args...)
			cmd.WaitDelay = time.Second // a child that keeps stdout open cannot outlive the probe
			return cmd.Output()
		},
	}
}

func (p *Provider) ID() string        { return ProviderID }
func (p *Provider) Name() string      { return ProviderName }
func (p *Provider) IsCompanion() bool { return true }

func (p *Provider) Capabilities() []contextopt.Capability {
	return []contextopt.Capability{
		contextopt.CapToolResponseOptimization,
		contextopt.CapContextRetrieval,
		contextopt.CapSessionIntegration,
		contextopt.CapOriginalOutputRecovery,
	}
}

func (p *Provider) HasCapability(cap contextopt.Capability) bool {
	return contextopt.HasCapability(p, cap)
}

// SupportedHarnesses is data from Headroom's compatibility table
// (README, read 2026-10-17). Downshift does not run these commands.
func (p *Provider) SupportedHarnesses() []contextopt.HarnessSupport {
	return []contextopt.HarnessSupport{
		{HarnessID: "claude-code", Supported: true, Method: contextopt.MethodEnvWrapper,
			Notes: "User runs `headroom wrap claude` (undo: `headroom unwrap claude`). The proxy compresses tool results before the API; the Downshift hook still sees the original."},
		{HarnessID: "codex", Supported: true, Method: contextopt.MethodEnvWrapper,
			Notes: "User runs `headroom wrap codex` (undo: `headroom unwrap codex`)."},
		{HarnessID: "cursor", Supported: true, Method: contextopt.MethodConfig,
			Notes: "Manual: `headroom wrap cursor` starts the proxy and prints base URLs to paste into Cursor's settings."},
		{HarnessID: "antigravity", Supported: false, Method: contextopt.MethodConfig,
			Notes: "Not in Headroom's wrap list (2026-10-17). Any OpenAI-compatible client can point at `headroom proxy`; unverified here."},
		{HarnessID: "kirocrew", Supported: false, Method: contextopt.MethodConfig,
			Notes: "Not in Headroom's wrap list (2026-10-17); unverified here."},
	}
}
