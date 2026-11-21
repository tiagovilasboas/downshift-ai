// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package contextopt

import (
	"errors"
	"testing"
)

func TestCCRSamplesEnabled(t *testing.T) {
	f := false
	tr := true
	cases := []struct {
		name        string
		cfg         Config
		err         error
		env, ccrEnv string
		want        bool
	}{
		{"default on", Config{}, nil, "", "", true},
		{"file false", Config{CCRSamples: &f}, nil, "", "", false},
		{"file true", Config{CCRSamples: &tr}, nil, "", "", true},
		{"env off", Config{}, nil, "0", "", false},
		{"env on beats file", Config{CCRSamples: &f}, nil, "1", "", true},
		{"ccr off", Config{}, nil, "", "0", false},
		{"ccr file false keeps samples", Config{CCR: &f}, nil, "", "", true},
		{"bad config stays on", Config{}, errors.New("x"), "", "", true},
	}
	for _, c := range cases {
		if got := CCRSamplesEnabled(c.cfg, c.err, c.env, c.ccrEnv); got != c.want {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}
