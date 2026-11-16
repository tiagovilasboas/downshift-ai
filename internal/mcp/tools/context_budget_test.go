// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package tools

import "testing"

// Every tool description is loaded into the model's context in every session
// that registers the server, so each one has a byte cap and the sum has a
// second cap. Raise a cap only with a measured reason; move a rule that belongs
// to no single tool into the server instructions instead.
const (
	maxToolDescriptionBytes = 700
	maxAllDescriptionsBytes = 6500
)

func TestToolDescriptionsStayWithinTheContextBudget(t *testing.T) {
	all := append(Info(fullDeps()), WorkTools(Deps{})...)
	total := 0
	for _, tl := range all {
		n := len(tl.Description)
		total += n
		if n > maxToolDescriptionBytes {
			t.Errorf("%s: description is %d bytes, cap is %d", tl.Name, n, maxToolDescriptionBytes)
		}
	}
	if total > maxAllDescriptionsBytes {
		t.Errorf("descriptions total %d bytes, cap is %d", total, maxAllDescriptionsBytes)
	}
}
