// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core

func (r *sessionResolver) EffortFor(harness, id string, effort Effort) string {
	if _, ok := r.efforts[id]; ok && harness == r.harness {
		return effort.String()
	}
	if r.fallback != nil {
		return r.fallback.EffortFor(harness, id, effort)
	}
	return effort.String()
}

func (r *sessionResolver) AllowedEfforts(harness, id string) []string {
	if harness == r.harness {
		if efforts, ok := r.efforts[id]; ok {
			return append([]string(nil), efforts...)
		}
	}
	if provider, ok := r.fallback.(EffortLister); ok {
		return provider.AllowedEfforts(harness, id)
	}
	return nil
}

func (r *sessionResolver) IsExplicitOnly(harness, id string) bool {
	return r.fallback != nil && r.fallback.IsExplicitOnly(harness, id)
}

// IsShadow forwards the allowlist shadow check; the catalog owns the rows.
func (r *sessionResolver) IsShadow(harness, id string) bool {
	if s, ok := r.fallback.(interface{ IsShadow(string, string) bool }); ok {
		return s.IsShadow(harness, id)
	}
	return false
}

// ShadowGateways forwards the optional allowlist extension. The allowlist is
// a user fact, not a session fact, so it comes from the fallback unchanged.
func (r *sessionResolver) ShadowGateways() []Model {
	if g, ok := r.fallback.(GatewayShadowResolver); ok {
		return g.ShadowGateways()
	}
	return nil
}
