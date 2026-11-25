// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package postrun

import (
	"fmt"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/core"
	"github.com/tiagovilasboas/downshift-ai/internal/execwrap"
	"github.com/tiagovilasboas/downshift-ai/internal/hookport"
)

func TestCompact_NeverRecompactsCLIRetrieve(t *testing.T) {
	caps := core.HarnessCapabilities{
		ReplacesToolOutput: []core.ToolCategory{
			core.CategoryShell, core.CategorySearch, core.CategoryRead, core.CategoryOther,
		},
	}
	over4K := verboseLogLines(100, "cli-4k")   // > 4 KiB
	over16K := verboseLogLines(400, "cli-16k") // > 16 KiB

	commands := []string{
		"downshift retrieve 0123456789ab",
		"downshift retrieve --session=aaaaaaaaaaaa 0123456789ab",
		"/usr/local/bin/downshift retrieve 0123456789ab",
		`C:\Users\dev\bin\downshift.exe retrieve 0123456789ab`,
		"FOO=1 DOWNSHIFT_CCR=0 /opt/homebrew/bin/downshift retrieve --head 500 0123456789ab",
	}

	for _, cmd := range commands {
		for _, body := range []string{over4K, over16K} {
			keepCalls := 0
			opts := Options{
				Keep: func([]byte) (string, bool) {
					keepCalls++
					return "shouldnotstore", true
				},
				Balanced: true,
				Session:  strings.Repeat("ab", 32),
			}
			in := hookport.ToolOutput{
				Category:  core.CategoryShell,
				Command:   cmd,
				Stdout:    body,
				HasStdout: true,
				ExitCode:  0,
			}
			res := Compact(caps, in, opts)
			if !res.Considered {
				t.Fatalf("%q (%d B): Considered = false, want true", cmd, len(body))
			}
			if res.Compacted || res.Replaced {
				t.Fatalf("%q (%d B): Compacted=%v Replaced=%v, want false", cmd, len(body), res.Compacted, res.Replaced)
			}
			if res.Passthrough != execwrap.PassthroughRetrieve {
				t.Fatalf("%q (%d B): Passthrough = %q, want %q", cmd, len(body), res.Passthrough, execwrap.PassthroughRetrieve)
			}
			if res.Output.Stdout != body {
				t.Fatalf("%q (%d B): Stdout was modified (got %d B, want %d B)", cmd, len(body), len(res.Output.Stdout), len(body))
			}
			if res.KeptID != "" || keepCalls != 0 {
				t.Fatalf("%q (%d B): triggered CCR storage (KeptID=%q keepCalls=%d)", cmd, len(body), res.KeptID, keepCalls)
			}
		}
	}
}

func TestCompact_NeverRecompactsMCPRetrieve(t *testing.T) {
	caps := core.HarnessCapabilities{
		ReplacesToolOutput: []core.ToolCategory{
			core.CategoryShell, core.CategorySearch, core.CategoryRead, core.CategoryOther,
		},
	}
	over4KText := verboseLogLines(100, "mcp-4k")
	over16KText := verboseLogLines(400, "mcp-16k")

	var jsonItems strings.Builder
	jsonItems.WriteString(`{"items":[`)
	for i := 0; i < 400; i++ {
		if i > 0 {
			jsonItems.WriteByte(',')
		}
		fmt.Fprintf(&jsonItems, `{"id":%d,"msg":"retrieved payload item with enough text to exceed JSONPruneMinBytes and 16 KiB"}`, i)
	}
	jsonItems.WriteString(`]}`)
	over16KJSON := jsonItems.String()

	toolNames := []string{
		"mcp__downshift__retrieve",
		"mcp__plugin_downshift_downshift__retrieve",
		"retrieve",
	}

	for _, tool := range toolNames {
		for _, body := range []string{over4KText, over16KText, over16KJSON} {
			keepCalls := 0
			opts := Options{
				Keep: func([]byte) (string, bool) {
					keepCalls++
					return "shouldnotstore", true
				},
				Balanced: true,
				Session:  strings.Repeat("cd", 32),
			}
			in := hookport.ToolOutput{
				Category:    core.CategoryOther,
				ToolName:    tool,
				VerboseKind: VerboseKindMCP,
				Content:     body,
				HasContent:  true,
				ExitCode:    -1,
			}
			res := Compact(caps, in, opts)
			if !res.Considered {
				t.Fatalf("%q (%d B): Considered = false, want true", tool, len(body))
			}
			if res.Compacted || res.Replaced {
				t.Fatalf("%q (%d B): Compacted=%v Replaced=%v, want false", tool, len(body), res.Compacted, res.Replaced)
			}
			if res.Passthrough != execwrap.PassthroughRetrieve {
				t.Fatalf("%q (%d B): Passthrough = %q, want %q", tool, len(body), res.Passthrough, execwrap.PassthroughRetrieve)
			}
			if res.Output.Content != body {
				t.Fatalf("%q (%d B): Content was modified (got %d B, want %d B)", tool, len(body), len(res.Output.Content), len(body))
			}
			if res.KeptID != "" || keepCalls != 0 {
				t.Fatalf("%q (%d B): triggered CCR storage (KeptID=%q keepCalls=%d)", tool, len(body), res.KeptID, keepCalls)
			}
		}
	}

	// Non-Downshift MCP tools still compact normally.
	other := Compact(caps, hookport.ToolOutput{
		Category:    core.CategoryOther,
		ToolName:    "mcp__other__retrieve",
		VerboseKind: VerboseKindMCP,
		Content:     over16KText,
		HasContent:  true,
		ExitCode:    -1,
	}, Options{Keep: keepOK, Balanced: true})
	if !other.Compacted || !other.Replaced {
		t.Fatalf("non-downshift MCP tool should still compact, got Compacted=%v Replaced=%v", other.Compacted, other.Replaced)
	}
}

func TestIsRetrieveCommand_IgnoresNonRetrieve(t *testing.T) {
	for _, cmd := range []string{
		"",
		"downshift status",
		"echo downshift retrieve 0123456789ab",
		`git commit -m "fix downshift retrieve"`,
	} {
		if isRetrieveCommand(cmd) {
			t.Errorf("isRetrieveCommand(%q) = true, want false", cmd)
		}
	}
}
