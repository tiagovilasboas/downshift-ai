// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tiagovilasboas/downshift-ai/internal/setup"
)

// The installed-plugin hook check for `doctor`. A plugin is pinned to the
// commit it was installed from, so the hooks it carries can be older than this
// binary: a hook this binary expects may be missing, and a hook that was
// removed (the retired `dci` hook) may still run. The "hooks wired" row counts
// the hooks this binary ships, so only the installed hooks/hooks.json can show
// the difference.
//
// The registry path comes from the setup target (ConfigDir, Plugin, Hooks,
// Retired): nothing here names a harness. The check only reads files.

// installedPlugin is one entry of a plugin id in installed_plugins.json.
type installedPlugin struct {
	Scope        string `json:"scope"`
	InstallPath  string `json:"installPath"`
	Version      string `json:"version"`
	GitCommitSha string `json:"gitCommitSha"`
}

// checkInstalledPluginHooks compares the hooks of the installed plugin with the
// hooks this binary ships and reports one "plugin hooks" row per user install.
// enabled says the settings file enables the plugin: then a plugin that is not
// installed is reported. Without the plugin nothing is reported, the hooks live
// in the settings file and the "hooks wired" row covers them.
func checkInstalledPluginHooks(add func(id, status, detail, fix string), home, harness string, enabled bool) {
	_, binCommit := currentBuild()
	checkInstalledPluginHooksAt(add, home, harness, enabled, binCommit)
}

func checkInstalledPluginHooksAt(add func(id, status, detail, fix string), home, harness string, enabled bool, binCommit string) {
	t, ok := pluginTarget(harness)
	if !ok {
		return
	}
	reg := filepath.Join(home, filepath.FromSlash(t.ConfigDir), filepath.FromSlash(setup.PluginRegistryRel))
	installs, err := readPluginInstalls(reg, t.Plugin)
	installFix := "/plugin install " + strings.SplitN(t.Plugin, "@", 2)[0] + " --marketplace " + setup.PluginMarketplaceSource
	switch {
	case !enabled && (err != nil || len(installs) == 0):
		return
	case err != nil:
		add("plugin hooks", doctorWarn, t.Plugin+" is enabled but "+err.Error(), installFix)
		return
	case len(installs) == 0:
		add("plugin hooks", doctorWarn, fmt.Sprintf("%s is enabled but %s lists no user install of it", t.Plugin, reg), installFix)
		return
	}
	updateFix := "claude plugin update " + t.Plugin + ", then open a new session"
	for _, in := range installs {
		commit := in.commit()
		diff, err := diffInstalledPluginHooks(in.InstallPath, t)
		switch {
		case err != nil:
			add("plugin hooks", doctorWarn, fmt.Sprintf("installed plugin at %s: %v", commit, err), updateFix)
		case diff.empty():
			if shortBin, lags := pluginCommitLags(in.GitCommitSha, binCommit); lags {
				add("plugin hooks", doctorWarn,
					fmt.Sprintf("installed plugin at %s carries the %d hooks, but lags behind binary commit %s (bundled mod/skills may be older)",
						shortCommit(in.GitCommitSha), len(t.Hooks), shortBin), updateFix)
				continue
			}
			add("plugin hooks", doctorOK,
				fmt.Sprintf("installed plugin at %s carries the %d hooks this binary ships", commit, len(t.Hooks)), "")
		default:
			add("plugin hooks", doctorWarn, fmt.Sprintf("installed plugin at %s differs from this binary: %s", commit, diff), updateFix)
		}
	}
}

// pluginTarget returns the setup target of harness when it can load a plugin.
func pluginTarget(harness string) (setup.Target, bool) {
	for _, t := range setup.Targets {
		if t.Name == harness && t.Plugin != "" && t.ConfigDir != "" {
			return t, true
		}
	}
	return setup.Target{}, false
}

// readPluginInstalls returns the user-scope installs of plugin from the
// registry file. A missing or invalid registry is an error; a registry without
// the plugin is an empty list. A relative installPath is skipped: it would
// resolve against the working directory, not the plugin cache.
func readPluginInstalls(reg, plugin string) ([]installedPlugin, error) {
	raw, err := os.ReadFile(reg)
	if err != nil {
		return nil, fmt.Errorf("cannot read %s", reg)
	}
	var doc struct {
		Plugins map[string][]installedPlugin `json:"plugins"`
	}
	if json.Unmarshal(raw, &doc) != nil {
		return nil, fmt.Errorf("%s is not valid JSON", reg)
	}
	var out []installedPlugin
	for _, in := range doc.Plugins[plugin] {
		if (in.Scope == "" || in.Scope == "user") && filepath.IsAbs(in.InstallPath) {
			out = append(out, in)
		}
	}
	return out, nil
}

// commit names the installed commit for a message.
func (in installedPlugin) commit() string {
	switch {
	case in.GitCommitSha != "":
		return "commit " + shortCommit(in.GitCommitSha)
	case in.Version != "":
		return "version " + in.Version + " (no commit recorded)"
	}
	return "an unknown commit"
}

// pluginCommitLags reports whether installedSha and binCommit are both real
// hex commits and neither is a prefix of the other, returning the shortened
// binary commit for the warning detail.
func pluginCommitLags(installedSha, binCommit string) (string, bool) {
	inst, ok1 := normHexCommit(installedSha)
	bin, ok2 := normHexCommit(binCommit)
	if !ok1 || !ok2 {
		return "", false
	}
	if strings.HasPrefix(inst, bin) || strings.HasPrefix(bin, inst) {
		return "", false
	}
	return shortCommit(bin), true
}

func normHexCommit(s string) (string, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	if len(s) < 7 {
		return "", false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return "", false
		}
	}
	return s, true
}

func shortCommit(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 12 {
		return s[:12]
	}
	return s
}
