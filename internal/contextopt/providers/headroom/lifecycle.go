// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package headroom

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/contextopt"
)

func (p *Provider) HealthCheck(ctx context.Context) error {
	installed, _, _, err := p.Detect(ctx)
	if err != nil {
		return err
	}
	if !installed {
		return errors.New("headroom is not installed")
	}
	return nil
}

// Detect reports whether the headroom binary is on PATH and its version.
func (p *Provider) Detect(ctx context.Context) (bool, string, string, error) {
	binPath, err := p.LookPath(Binary)
	if err != nil || binPath == "" {
		return false, "", "", nil
	}
	if p.CommandRunner == nil {
		return true, "", binPath, nil
	}
	vctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	out, err := p.CommandRunner(vctx, binPath, "--version")
	if err != nil {
		return true, "", binPath, fmt.Errorf("headroom found at %s but `headroom --version` failed: %w", binPath, err)
	}
	return true, parseVersion(string(out)), binPath, nil
}

// parseVersion keeps the last token that starts with a digit ("headroom 1.2.3" → "1.2.3").
func parseVersion(out string) string {
	fields := strings.Fields(out)
	for i := len(fields) - 1; i >= 0; i-- {
		f := strings.TrimPrefix(fields[i], "v")
		if f != "" && f[0] >= '0' && f[0] <= '9' {
			return f
		}
	}
	return strings.TrimSpace(out)
}

// Validate fails when Headroom is not installed, with the install hint.
func (p *Provider) Validate(ctx context.Context) error {
	installed, _, _, err := p.Detect(ctx)
	if err != nil {
		return err
	}
	if !installed {
		return fmt.Errorf("headroom is not on PATH; install it first: %s", InstallHint)
	}
	return nil
}

// Configure records the companion in Downshift's own config. It writes
// nothing to the project or to any harness and runs no headroom command.
func (p *Provider) Configure(ctx context.Context, targetDir string, harnessID string) error {
	cfg, err := contextopt.LoadConfig()
	if err != nil {
		cfg = contextopt.DefaultConfig()
	}
	cfg.Companion = ProviderID
	return contextopt.SaveConfig(cfg)
}

// Disable clears the companion record. It does not unwrap any harness.
func (p *Provider) Disable(ctx context.Context, targetDir string, harnessID string) error {
	cfg, err := contextopt.LoadConfig()
	if err != nil {
		return err
	}
	if cfg.Companion != ProviderID {
		return nil
	}
	cfg.Companion = ""
	return contextopt.SaveConfig(cfg)
}

// GetDiagnostics reports detection, the enabled record, and the per-harness
// wrap command. Issues and recommendations are plain strings for doctor.
func (p *Provider) GetDiagnostics(ctx context.Context, targetDir string) (*contextopt.Diagnostics, error) {
	installed, version, binPath, err := p.Detect(ctx)
	diag := &contextopt.Diagnostics{
		ProviderID:         ProviderID,
		Installed:          installed,
		BinaryPath:         binPath,
		Version:            version,
		SupportedHarnesses: p.SupportedHarnesses(),
	}
	cfg, cfgErr := contextopt.LoadConfig()
	enabled := cfgErr == nil && cfg.Companion == ProviderID
	if err != nil {
		diag.Issues = append(diag.Issues, err.Error())
	}
	switch {
	case !installed && enabled:
		diag.Issues = append(diag.Issues, "companion enabled in Downshift but the headroom binary is not on PATH")
		diag.Recommendations = append(diag.Recommendations, "install it ("+InstallHint+") or run `downshift context disable headroom`")
	case !installed:
		diag.Recommendations = append(diag.Recommendations, "optional companion, not installed: "+InstallHint+" ("+Project+")")
	case installed && !enabled:
		diag.Recommendations = append(diag.Recommendations, "detected but not recorded: run `downshift context enable headroom`, then `headroom wrap <tool>` for your harness")
	default:
		diag.ActiveHarnesses = nil // Downshift cannot see which harness is wrapped.
		diag.Recommendations = append(diag.Recommendations, "recorded as active; wrap each harness yourself (`headroom wrap claude`, `headroom wrap codex`, ...). Downshift cannot tell whether a session runs through the proxy.")
	}
	diag.Recommendations = append(diag.Recommendations, PrivacyNote,
		"Savings while Headroom is active are not routing savings alone; read `headroom savings` separately from `downshift stats`.")
	return diag, nil
}

// GetMetrics stays unavailable: Headroom keeps its own counters
// (`headroom savings`, `headroom dashboard`) and Downshift does not parse them.
func (p *Provider) GetMetrics(ctx context.Context, projectDir string) (*contextopt.Metrics, error) {
	return &contextopt.Metrics{
		ProviderID:  ProviderID,
		CapturedAt:  time.Now().UTC(),
		IsEstimated: false,
		TokenState:  "unavailable",
	}, nil
}
