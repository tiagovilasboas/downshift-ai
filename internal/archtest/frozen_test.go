// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package archtest

import (
	"path"
	"slices"
	"strings"
	"testing"
)

// Frozen packages (owner decision, 2026-11-20; docs/reference/frozen-packages.md).
// Fixes, security work and removals are allowed; new features and new callers
// from outside the package are not. frozenImporters records today's importers
// of each package, test files excluded. A file in cmd/downshift (one package)
// is named by file; elsewhere by the importer's package directory.
// An entry that stops matching is not an error: the list can only shrink.
var frozenImporters = map[string][]string{
	"loop":    {"cmd/downshift/loop.go", "cmd/downshift/loop_batch.go", "cmd/downshift/loop_report.go"},
	"cascade": {"cmd/downshift/exec.go", "internal/loop"},
	"gateway": {"cmd/downshift/cmd_gateway.go", "cmd/downshift/cmd_gateway_switch.go", "cmd/downshift/cmd_plan.go",
		"cmd/downshift/doctor_gateway.go", "cmd/downshift/doctor_workers.go", "cmd/downshift/doctor.go",
		"cmd/downshift/docs_manifest.go", "cmd/downshift/setup_tui_report.go", "internal/gateway/ninerouter"},
	"missedsavings":  {"cmd/downshift/cmd_discover.go"},
	"operatorexport": {"cmd/downshift/export.go"},
}

// contextoptCmdFiles: the `context` command surface of contextopt is frozen;
// only cmd/downshift files named context* count, and these exist today. Other
// importers of the package (doctor, setup, adapters) stay unrestricted.
var contextoptCmdFiles = []string{"cmd/downshift/context.go", "cmd/downshift/context_compress.go", "cmd/downshift/context_enable.go"}

// frozenNewImporters returns the files of imports (file -> import paths) that
// import the frozen package pkg, are not inside it, and are not in allowed.
// Pure, so the negative test can prove it flags a new importer.
func frozenNewImporters(imports map[string][]string, pkg string, allowed []string) []string {
	var out []string
	for file, imps := range imports {
		dir := path.Dir(file)
		if dir == "internal/"+pkg || !importsPackage(imps, pkg) {
			continue
		}
		if slices.Contains(allowed, file) || slices.Contains(allowed, dir) {
			continue
		}
		out = append(out, file)
	}
	slices.Sort(out)
	return out
}

// importsPackage reports whether imps holds internal/pkg or one of its sub-packages.
func importsPackage(imps []string, pkg string) bool {
	for _, i := range imps {
		if i == module+pkg || strings.HasPrefix(i, module+pkg+"/") {
			return true
		}
	}
	return false
}

func frozenMessage(pkg string, files []string) string {
	return "internal/" + pkg + " is frozen (owner decision, 2026-11-20): new callers are not allowed, found " +
		strings.Join(files, ", ") + "; see docs/reference/frozen-packages.md"
}

func TestFrozenPackagesGainNoImporters(t *testing.T) {
	imports := map[string][]string{}
	for _, f := range allProductionGoFiles(t) {
		imports[f] = importsOf(t, repoRoot+"/"+f)
	}
	for pkg, allowed := range frozenImporters {
		if bad := frozenNewImporters(imports, pkg, allowed); len(bad) > 0 {
			t.Error(frozenMessage(pkg, bad))
		}
	}
	// contextopt: only new cmd/downshift files named context* are frozen.
	cmdContext := map[string][]string{}
	for f, imps := range imports {
		if strings.HasPrefix(f, "cmd/downshift/context") {
			cmdContext[f] = imps
		}
	}
	if bad := frozenNewImporters(cmdContext, "contextopt", contextoptCmdFiles); len(bad) > 0 {
		t.Error(frozenMessage("contextopt", bad))
	}
}

func TestFrozenCheckFlagsNewImporter(t *testing.T) {
	imports := map[string][]string{
		"cmd/downshift/old.go":    {module + "loop"},
		"cmd/downshift/new.go":    {module + "loop"},
		"internal/other/x.go":     {module + "gateway/ninerouter"},
		"internal/loop/runner.go": {module + "loop"},
	}
	got := frozenNewImporters(imports, "loop", []string{"cmd/downshift/old.go"})
	if !slices.Equal(got, []string{"cmd/downshift/new.go"}) {
		t.Fatalf("new loop importers = %v, want [cmd/downshift/new.go]", got)
	}
	got = frozenNewImporters(imports, "gateway", nil)
	if !slices.Equal(got, []string{"internal/other/x.go"}) {
		t.Fatalf("new gateway importers = %v, want internal/other/x.go", got)
	}
	if !strings.Contains(frozenMessage("loop", got), "docs/reference/frozen-packages.md") {
		t.Fatal("message must point to the frozen-packages doc")
	}
}
