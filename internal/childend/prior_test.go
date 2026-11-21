// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package childend

import (
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/hookport"
	"github.com/tiagovilasboas/downshift-ai/internal/telemetry"
)

// A call that can produce no record must not read the event log at all.
func TestNeedsPrior(t *testing.T) {
	use := hookport.TokenUsage{InputTokens: 5}
	cases := []struct {
		name  string
		ce    hookport.ChildEnd
		usage telemetry.TokenUsage
		sess  string
		call  string
		agent string
		want  bool
	}{
		{"plain tool call, nothing reported", hookport.ChildEnd{Harness: "claude-code", Source: hookport.SourcePayload}, telemetry.TokenUsage{}, "s", "c", "", false},
		{"unknown status only", hookport.ChildEnd{Harness: "claude-code", Status: telemetry.ChildUnknown}, telemetry.TokenUsage{}, "s", "c", "", false},
		{"resolved model without a session or call id", hookport.ChildEnd{Harness: "claude-code", ResolvedModel: "m"}, telemetry.TokenUsage{}, "", "", "", false},
		{"resolved model with a call id", hookport.ChildEnd{Harness: "claude-code", ResolvedModel: "m"}, telemetry.TokenUsage{}, "", "c", "", true},
		{"child status with a session", hookport.ChildEnd{Harness: "claude-code", Status: telemetry.ChildCompleted}, telemetry.TokenUsage{}, "s", "", "", true},
		{"payload usage", hookport.ChildEnd{Harness: "claude-code", Usage: &use}, telemetry.TokenUsage{InputTokens: 5}, "", "", "", true},
		{"transcript without usage", hookport.ChildEnd{Source: hookport.SourceTranscript}, telemetry.TokenUsage{}, "s", "", "a", false},
		{"transcript usage without agent id", hookport.ChildEnd{Source: hookport.SourceTranscript}, telemetry.TokenUsage{InputTokens: 5}, "s", "", "", false},
		{"transcript usage with agent id", hookport.ChildEnd{Source: hookport.SourceTranscript}, telemetry.TokenUsage{InputTokens: 5}, "s", "", "a", true},
		{"transcript status is not observed", hookport.ChildEnd{Source: hookport.SourceTranscript, Status: telemetry.ChildCompleted}, telemetry.TokenUsage{}, "s", "", "a", false},
		{"transcript failed end without usage", hookport.ChildEnd{Source: hookport.SourceTranscript, Status: telemetry.ChildError}, telemetry.TokenUsage{}, "s", "", "a", true},
		{"transcript failed end without agent id", hookport.ChildEnd{Source: hookport.SourceTranscript, Status: telemetry.ChildError}, telemetry.TokenUsage{}, "s", "", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := needsPrior(tc.ce, tc.usage, tc.sess, tc.call, tc.agent); got != tc.want {
				t.Fatalf("needsPrior = %v, want %v", got, tc.want)
			}
		})
	}
}
