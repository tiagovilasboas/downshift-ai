// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package accounting

import (
	"fmt"

	"github.com/tiagovilasboas/downshift-ai/internal/telemetry"
)

// WorkingMemory summarizes how much noisy scrollback the Compactor kept out
// of the agent's context window within a Scope.
type WorkingMemory struct {
	TokensCompactedEstimate int64  `json:"tokens_compacted_estimate"`
	BytesCompactedTotal     int64  `json:"bytes_compacted_total"`
	CompactedCommands       int64  `json:"compacted_commands"`
	Scope                   string `json:"scope,omitempty"`
	Line                    string `json:"line,omitempty"`
}

// BuildWorkingMemory derives WorkingMemory from already-computed accounting
// Numbers so no surface repeats compaction arithmetic.
func BuildWorkingMemory(n Numbers) WorkingMemory {
	return WorkingMemory{
		TokensCompactedEstimate: n.Compactor.Tokens,
		BytesCompactedTotal:     n.Compactor.SavedBytes,
		CompactedCommands:       n.Compactor.Compacted,
		Scope:                   n.Scope.Label(),
		Line:                    WorkingMemoryLine(n.Compactor.Tokens, n.Compactor.SavedBytes, n.Scope.Label()),
	}
}

// WorkingMemoryLine formats the one-line working-memory protection indicator,
// returning "" when no bytes have been kept out of context. The scope label
// is part of the line so a reader never takes a multi-session total for the
// session in front of them.
func WorkingMemoryLine(tokens, savedBytes int64, scope string) string {
	if tokens > 0 {
		return fmt.Sprintf("Context preserved (%s): ~%s tokens of noisy scrollback kept out of working memory", scope, telemetry.ShortTokens(tokens))
	}
	if savedBytes > 0 {
		return fmt.Sprintf("Context preserved (%s): %d bytes of noisy scrollback kept out of working memory", scope, savedBytes)
	}
	return ""
}
