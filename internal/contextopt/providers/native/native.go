// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package native

import (
	"github.com/tiagovilasboas/downshift-ai/internal/compressor"
	"github.com/tiagovilasboas/downshift-ai/internal/contextopt"
)

const (
	ProviderID   = "native"
	ProviderName = "Downshift Native Context Compressor"
)

// Provider implements contextopt.Provider for the in-process native compressor.
type Provider struct {
	Mode compressor.Mode
}

// NewProvider creates a new Native provider.
func NewProvider(mode compressor.Mode) *Provider {
	if mode == "" {
		mode = compressor.ModeObserve
	}
	return &Provider{Mode: mode}
}

func (p *Provider) ID() string   { return ProviderID }
func (p *Provider) Name() string { return ProviderName }

func (p *Provider) Capabilities() []contextopt.Capability {
	return []contextopt.Capability{
		contextopt.CapCommandOutputOptimization,
		contextopt.CapMetricsSupport,
		contextopt.CapOriginalOutputRecovery,
	}
}

func (p *Provider) HasCapability(cap contextopt.Capability) bool {
	return contextopt.HasCapability(p, cap)
}

func (p *Provider) SupportedHarnesses() []contextopt.HarnessSupport {
	return []contextopt.HarnessSupport{
		{
			HarnessID: "claude-code",
			Supported: true,
			Method:    contextopt.MethodHook,
			Notes:     "PostToolUse records CompressExit in observe mode when the native provider is enabled, using the tool exit code. Claude Code would accept a replacement (updatedToolOutput); this hook does not send one.",
		},
		{
			HarnessID: "cursor",
			Supported: true,
			Method:    contextopt.MethodAgentInstruction,
			Notes:     "The hook does not rewrite tool output. Compression runs only when a caller invokes the compressor. Prompts are not compressed automatically.",
		},
		{
			HarnessID: "codex",
			Supported: true,
			Method:    contextopt.MethodAgentInstruction,
			Notes:     "Codex PostToolUse cannot replace tool output. Compression runs only when a caller invokes the compressor. Prompts are not compressed automatically.",
		},
		{
			HarnessID: "antigravity",
			Supported: true,
			Method:    contextopt.MethodAgentInstruction,
			Notes:     "The hook does not rewrite tool output. Compression runs only when a caller invokes the compressor. Prompts are not compressed automatically.",
		},
		{
			HarnessID: "kirocrew",
			Supported: true,
			Method:    contextopt.MethodAgentInstruction,
			Notes:     "The hook does not rewrite tool output. Spawn policy may omit memory. Prompts are not compressed automatically.",
		},
	}
}
