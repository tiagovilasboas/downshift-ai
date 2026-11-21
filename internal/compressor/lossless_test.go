// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import "testing"

func TestLosslessText(t *testing.T) {
	cases := []struct {
		name, in, want string
		ok             bool
	}{
		{"ansi stripped", "\x1b[32mok\x1b[0m done\n\x1b]0;title\x07tail\n", "ok done\ntail\n", true},
		{"cr progress keeps final", "dl 10%\rdl 50%\rdl 100%\ndone\n", "dl 100%\ndone\n", true},
		{"crlf normalised", "a\r\nb\r\n", "a\nb\n", true},
		{"cr danger segment kept", "step\rerror: disk full\rstep 2 ok\nend\n", "error: disk full\nstep 2 ok\nend\n", true},
		{"identical run folded", "tick tock\ntick tock\ntick tock\ntick tock\nend\n", "tick tock\n(repeated 4 times)\nend\n", true},
		{"identical danger lines kept", "error: boom\nerror: boom\nerror: boom\n", "error: boom\nerror: boom\nerror: boom\n", false},
		{"blank run not folded", "a\n\n\n\nb\n", "a\n\n\n\nb\n", false},
		{"nothing to do", "plain\ntext\n", "plain\ntext\n", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := LosslessText([]byte(c.in))
			if ok != c.ok || string(got) != c.want {
				t.Errorf("LosslessText(%q) = %q, %v; want %q, %v", c.in, got, ok, c.want, c.ok)
			}
		})
	}
}
