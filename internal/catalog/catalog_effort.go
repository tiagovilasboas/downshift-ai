// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package catalog

import "github.com/tiagovilasboas/downshift-ai/internal/core"

// EffortValue translates a core.Effort to the harness-native string for a
// given model entry. Falls back to effort.String() if no map entry exists.
func EffortValue(e Entry, effort core.Effort) string {
	if e.EffortMap != nil {
		if v, ok := e.EffortMap[effort.String()]; ok {
			return v
		}
	}
	return effort.String()
}

// EffortFor implements core.Resolver. It translates a core.Effort level to the
// harness-native string for the given model ID using the entry's effort_map.
// Falls back to effort.String() ("low"/"medium"/"high") when no entry or map
// is found — safe to call for any harness/model combination.
func (c *Catalog) EffortFor(harness, modelID string, effort core.Effort) string {
	if entry, ok := c.EntryFor(harness, modelID); ok {
		return EffortValue(entry, effort)
	}
	return effort.String()
}

// AllowedEfforts implements core.EffortLister. It returns the effort levels the
// catalog declares for the exact model id (id, alias or native name). A family
// prefix match never counts: a list is only trusted for the model it was
// verified on. Nil means unknown.
func (c *Catalog) AllowedEfforts(harness, modelID string) []string {
	if c == nil || modelID == "" {
		return nil
	}
	if slug, ok := gatewaySlug(modelID); ok {
		if slug == "" {
			return nil
		}
		for _, e := range c.entries {
			if isGatewayProvider(e.Provider) && e.Harness == harness && isGatewayName(e, slug) {
				return e.Efforts
			}
		}
		return nil
	}
	for _, id := range uniqueStrings(modelID, normaliseModelID(modelID)) {
		for _, e := range c.entries {
			if e.Harness != harness || len(e.Efforts) == 0 {
				continue
			}
			if e.ID == id || e.NativeName == id {
				return e.Efforts
			}
			for _, a := range e.Aliases {
				if a == id {
					return e.Efforts
				}
			}
		}
	}
	return nil
}

// EffortUnknownModels lists "harness/id" for every entry that declares no
// efforts list, so doctor can show where effort values are not constrained.
func (c *Catalog) EffortUnknownModels() []string {
	var out []string
	for _, e := range c.entries {
		if e.ID != "" && len(e.Efforts) == 0 {
			out = append(out, e.Harness+"/"+e.ID)
		}
	}
	return out
}
