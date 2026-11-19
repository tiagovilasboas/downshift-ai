// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package opencode_test

import (
	"slices"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/adapters/opencode"
)

func TestSkillRequest_HandleAndUsedSkills(t *testing.T) {
	cases := []struct {
		name       string
		req        opencode.Request
		wantReason string
		wantSkills []string
	}{
		{
			name: "skill tool ponytail",
			req: opencode.Request{
				V: 1, Kind: "skill", Harness: opencode.RequestHarness{ID: "opencode"}, Event: "skill_used",
				Correlation: opencode.RequestRelation{Child: testChild},
				Tool:        "skill", Skill: "ponytail",
			},
			wantReason: "skill_observed",
			wantSkills: []string{"ponytail"},
		},
		{
			name: "read tool SKILL.md",
			req: opencode.Request{
				V: 1, Kind: "skill", Harness: opencode.RequestHarness{ID: "opencode"}, Event: "skill_used",
				Correlation: opencode.RequestRelation{Child: testChild},
				Tool:        "read", Path: "/home/alice/.config/opencode/skills/downshift-audit/SKILL.md",
			},
			wantReason: "skill_observed",
			wantSkills: []string{"downshift-audit"},
		},
		{
			name: "private skill collapses to other",
			req: opencode.Request{
				V: 1, Kind: "skill", Harness: opencode.RequestHarness{ID: "opencode"}, Event: "skill_used",
				Correlation: opencode.RequestRelation{Child: testChild},
				Tool:        "skill", Skill: "internal-corp-skill",
			},
			wantReason: "skill_observed",
			wantSkills: []string{"other"},
		},
		{
			name: "prompt mention",
			req: opencode.Request{
				V: 1, Kind: "skill", Harness: opencode.RequestHarness{ID: "opencode"}, Event: "skill_used",
				Correlation: opencode.RequestRelation{Child: testChild},
				TaskText:    "please use $ponytail here",
			},
			wantReason: "skill_observed",
			wantSkills: []string{"ponytail"},
		},
		{
			name: "missing signal fails open",
			req: opencode.Request{
				V: 1, Kind: "skill", Harness: opencode.RequestHarness{ID: "opencode"}, Event: "skill_used",
				Correlation: opencode.RequestRelation{Child: testChild},
			},
			wantReason: opencode.ReasonFailOpen,
			wantSkills: nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, note, dec := opencode.Handle(tc.req)
			if resp.Action != "allow" || resp.ReasonCode != tc.wantReason || note != "" || dec.Harness != "" {
				t.Fatalf("Handle = (%+v, %q, %+v), want allow/%s and zero decision", resp, note, dec, tc.wantReason)
			}
			if got := tc.req.UsedSkills(); !slices.Equal(got, tc.wantSkills) {
				t.Errorf("UsedSkills() = %v, want %v", got, tc.wantSkills)
			}
		})
	}
}
