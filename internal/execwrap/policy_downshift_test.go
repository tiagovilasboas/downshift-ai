// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/compressor"
)

func TestDownshiftReadOnlyAllowlist(t *testing.T) {
	const exe = "/usr/local/bin/downshift"

	allowed := []string{
		"downshift stats",
		"downshift stats --days 7",
		"downshift discover",
		"downshift report",
		"downshift doctor",
		"downshift doctor --json",
		"downshift version",
		"downshift help",
		"downshift session",
		"downshift models",
		"downshift models available --json",
	}
	for _, cmd := range allowed {
		d := Decide(cmd, exe)
		if !d.Rewrite || d.Family != FamilyDownshift || d.Reason != ReasonAllowlisted {
			t.Errorf("Decide(%q) = %+v, want Rewrite=true Family=%s Reason=%s", cmd, d, FamilyDownshift, ReasonAllowlisted)
		}
	}

	// Quoted variant uses policy_quoted.go.
	d := Decide(`downshift doctor --format "json"`, exe)
	if !d.Rewrite || d.Family != FamilyDownshift {
		t.Errorf("quoted Decide = %+v, want Rewrite=true Family=%s", d, FamilyDownshift)
	}

	if !formatAccepted(FamilyDownshift, compressor.FormatGeneric) {
		t.Error("expected FamilyDownshift to accept FormatGeneric")
	}
}

func TestDownshiftRefusesRecursiveAndMutating(t *testing.T) {
	const exe = "/usr/local/bin/downshift"

	cases := []struct {
		cmd    string
		reason string
	}{
		{"downshift compact -- go test ./...", ReasonAlreadyWrapped},
		{"downshift exec -- go test ./...", ReasonAlreadyWrapped},
		{"downshift compactor -- go test ./...", ReasonAlreadyWrapped},
		{"downshift setup", ReasonNotAllowlisted},
		{"downshift setup --yes", ReasonNotAllowlisted},
		{"downshift enable", ReasonNotAllowlisted},
		{"downshift uninstall", ReasonNotAllowlisted},
		{"downshift retrieve abc123", ReasonNotAllowlisted},
		{"downshift", ReasonNotAllowlisted},
	}
	for _, tc := range cases {
		d := Decide(tc.cmd, exe)
		if d.Rewrite || d.Reason != tc.reason {
			t.Errorf("Decide(%q) = %+v, want Rewrite=false Reason=%s", tc.cmd, d, tc.reason)
		}
	}
}
