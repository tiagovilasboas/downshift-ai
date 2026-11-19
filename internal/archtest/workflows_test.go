// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package archtest

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var usesPattern = regexp.MustCompile(`(?m)^\s*(?:-\s*)?uses:\s*([^\s#]+)`)
var shaRefPattern = regexp.MustCompile(`@[0-9a-f]{40}$`)

func TestWorkflowActionsArePinned(t *testing.T) {
	dir := filepath.Join(repoRoot, ".github", "workflows")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() || (!strings.HasSuffix(e.Name(), ".yml") && !strings.HasSuffix(e.Name(), ".yaml")) {
			continue
		}
		path := filepath.Join(dir, e.Name())
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range usesPattern.FindAllStringSubmatch(string(b), -1) {
			ref := m[1]
			if strings.HasPrefix(ref, "./") || shaRefPattern.MatchString(ref) {
				continue
			}
			t.Errorf("%s: action reference %q must be pinned to a 40-char commit SHA", filepath.ToSlash(filepath.Join(".github/workflows", e.Name())), ref)
		}
	}
}

func TestCodeQLGoBuildMode(t *testing.T) {
	path := filepath.Join(repoRoot, ".github", "workflows", "codeql.yml")
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(b), "\n")
	for i, line := range lines {
		if strings.TrimSpace(line) == "- language: go" {
			if i+1 >= len(lines) || strings.Contains(lines[i+1], "build-mode: none") {
				t.Errorf("codeql.yml: Go language entry must not use build-mode: none (use autobuild or manual)")
			}
		}
	}
}
