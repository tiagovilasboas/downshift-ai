// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tiagovilasboas/downshift-ai/internal/setup"
)

// pluginHooksRel is where a plugin keeps its hooks, relative to its install path.
const pluginHooksRel = "hooks/hooks.json"

// pluginHookCmd is one command found in an installed hooks.json.
type pluginHookCmd struct {
	Event, Matcher, Command string
}

// pluginHooksDiff is how the installed hooks differ from the ones this binary ships.
type pluginHooksDiff struct {
	Missing, Retired, Extra []string
}

func (d pluginHooksDiff) empty() bool {
	return len(d.Missing)+len(d.Retired)+len(d.Extra) == 0
}

// diffInstalledPluginHooks reads <installPath>/hooks/hooks.json and compares it
// with the hooks of t.
func diffInstalledPluginHooks(installPath string, t setup.Target) (pluginHooksDiff, error) {
	file := filepath.Join(installPath, filepath.FromSlash(pluginHooksRel))
	raw, err := os.ReadFile(file)
	if err != nil {
		return pluginHooksDiff{}, fmt.Errorf("cannot read %s", file)
	}
	got, err := parsePluginHooks(raw)
	if err != nil {
		return pluginHooksDiff{}, fmt.Errorf("%s is not a hooks file: %v", file, err)
	}
	return comparePluginHooks(got, t), nil
}

// parsePluginHooks flattens the nested hooks.json shape (event, matcher group,
// commands) into a sorted list, so a report does not depend on map order.
func parsePluginHooks(raw []byte) ([]pluginHookCmd, error) {
	var doc struct {
		Hooks map[string][]struct {
			Matcher string `json:"matcher"`
			Hooks   []struct {
				Command string `json:"command"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	var out []pluginHookCmd
	for event, groups := range doc.Hooks {
		for _, g := range groups {
			for _, h := range g.Hooks {
				out = append(out, pluginHookCmd{Event: event, Matcher: g.Matcher, Command: h.Command})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Event != b.Event {
			return a.Event < b.Event
		}
		if a.Matcher != b.Matcher {
			return a.Matcher < b.Matcher
		}
		return a.Command < b.Command
	})
	return out, nil
}

// comparePluginHooks sorts the installed commands against the hooks this binary
// ships: a shipped hook with no installed command is missing; an installed
// command that matches a retired hook is retired; any other unmatched command
// is extra.
func comparePluginHooks(got []pluginHookCmd, t setup.Target) pluginHooksDiff {
	var d pluginHooksDiff
	for _, want := range t.Hooks {
		found := false
		for _, g := range got {
			if isInstalledHook(g, []setup.Hook{want}, true) {
				found = true
				break
			}
		}
		if !found {
			d.Missing = append(d.Missing, hookLabel(want.Event, want.Matcher, "downshift "+want.Args))
		}
	}
	for _, g := range got {
		if isInstalledHook(g, t.Hooks, true) {
			continue
		}
		label := hookLabel(g.Event, g.Matcher, g.Command)
		if isInstalledHook(g, t.Retired, false) {
			d.Retired = append(d.Retired, label)
		} else {
			d.Extra = append(d.Extra, label)
		}
	}
	return d
}

// isInstalledHook reports whether installed command g is one of hooks: same
// event and the same Downshift subcommand, and the same matcher when
// withMatcher is set. A retired hook is matched without the matcher: the one an
// old plugin gave it is not part of what makes it retired.
func isInstalledHook(g pluginHookCmd, hooks []setup.Hook, withMatcher bool) bool {
	for _, h := range hooks {
		if g.Event == h.Event && (!withMatcher || g.Matcher == h.Matcher) && setup.IsDownshiftCommand(g.Command, h.Args) {
			return true
		}
	}
	return false
}

func hookLabel(event, matcher, command string) string {
	if matcher == "" {
		return event + " " + command
	}
	return event + "(" + matcher + ") " + command
}

func (d pluginHooksDiff) String() string {
	var parts []string
	for _, p := range []struct {
		name  string
		items []string
	}{{"missing", d.Missing}, {"retired", d.Retired}, {"extra", d.Extra}} {
		if len(p.items) > 0 {
			parts = append(parts, p.name+": "+strings.Join(p.items, ", "))
		}
	}
	return strings.Join(parts, "; ")
}
