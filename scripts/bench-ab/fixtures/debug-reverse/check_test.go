// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package debugreverse

import (
	"fmt"
	"testing"
)

func TestReverseASCII(t *testing.T) {
	for i := 0; i < 12; i++ {
		t.Run(fmt.Sprintf("ascii_%02d", i), func(t *testing.T) {
			if got := Reverse("abc"); got != "cba" {
				t.Fatalf("got %q want cba", got)
			}
		})
	}
}

func TestReverseUTF8(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"héllo", "olléh"},
		{"日本語", "語本日"},
		{"café", "éfac"},
	}
	for _, tc := range cases {
		if got := Reverse(tc.in); got != tc.want {
			t.Errorf("Reverse(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
