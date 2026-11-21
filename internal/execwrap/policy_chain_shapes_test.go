// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"strings"
	"testing"
)

// The chain shapes agents run most. Each allowlisted segment is wrapped alone,
// the separators and every other segment stay byte-identical, so `&&` still
// short-circuits and `;` still returns the last segment's exit code (the
// wrapper exits with the wrapped command's real code).
func TestDecide_ChainShapes(t *testing.T) {
	const w = "'/opt/ds/downshift' compact -- "
	push := "git pu" + "sh"
	cases := []struct {
		name, command, want, reason string
	}{
		{"cd then go test", "cd dir && go test ./...", "cd dir && " + w + "go test ./...", ReasonAllowlistedCompound},
		{"add, commit, push", "git add -A && git commit -m 'fix x' && " + push,
			w + "git add -A && " + w + "'git' 'commit' '-m' 'fix x' && " + w + push, ReasonAllowlistedQuoted},
		{"make check then push", "make check && " + push, w + "make check && " + w + push, ReasonAllowlistedChain},
		{"build then test", "go build ./... && go test ./pkg/...", w + "go build ./... && " + w + "go test ./pkg/...", ReasonAllowlistedChain},
		{"gofmt then vet", "gofmt -l . ; go vet ./...", "gofmt -l . ; " + w + "go vet ./...", ReasonAllowlistedChain},
		{"export kept", "export A=1 && go test ./...", "export A=1 && " + w + "go test ./...", ReasonAllowlistedChain},
		{"cd and echo kept", "cd d && go test ./... && echo ok", "cd d && " + w + "go test ./... && echo ok", ReasonAllowlistedChain},
		{"eight segments", strings.TrimSuffix(strings.Repeat("go test; ", 8), "; "),
			strings.TrimSuffix(strings.Repeat(w+"go test; ", 8), "; "), ReasonAllowlistedChain},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := Decide(tc.command, exe)
			if !d.Rewrite || d.Command != tc.want || d.Reason != tc.reason {
				t.Fatalf("Decide(%q) = %+v, want command %q reason %s", tc.command, d, tc.want, tc.reason)
			}
		})
	}
}

// Chains the policy cannot prove, or that hold no allowlisted program, pass raw.
func TestDecide_ChainShapesRefused(t *testing.T) {
	cases := []struct{ name, command, reason string }{
		{"pipe inside a segment", "cd d && go test ./... | tee out", ReasonUnsafeChar},
		{"substitution", "cd $(pwd) && go test ./...", ReasonUnsafeChar},
		{"quote with metachar", "cd d && echo 'a; b' && go test", ReasonUnsafeChar},
		{"redirect", "cd d && go test ./... > out.txt", ReasonUnsafeChar},
		{"or list", "cd d && go test ./... || echo fail", ReasonUnsafeChar},
		{"only builtins and unknown programs", "cd d && export A=1 && foo --bar", ReasonNotAllowlisted},
		{"nine segments", strings.TrimSuffix(strings.Repeat("go test; ", 9), "; "), ReasonTooLong},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if d := Decide(tc.command, exe); d.Rewrite || d.Reason != tc.reason {
				t.Fatalf("Decide(%q) = %+v, want no rewrite, reason %s", tc.command, d, tc.reason)
			}
		})
	}
}
