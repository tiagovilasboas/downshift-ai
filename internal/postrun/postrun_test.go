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

var allCaps = core.HarnessCapabilities{ReplacesToolOutput: []core.ToolCategory{core.CategoryShell, core.CategorySearch, core.CategoryRead}}

func keepOK(raw []byte) (string, bool) { return "0123456789ab", true }

func bigLog() string {
	var b strings.Builder
	for i := 0; i < 2000; i++ {
		fmt.Fprintf(&b, "step %d of the job finished\n", i)
	}
	return b.String()
}

func TestCompact_Shell(t *testing.T) {
	log := bigLog()
	shell := func(cmd, stdout string) hookport.ToolOutput {
		return hookport.ToolOutput{Category: core.CategoryShell, Command: cmd, Stdout: stdout, HasStdout: true, HasStderr: true, ExitCode: -1}
	}
	opts := Options{Keep: keepOK, Balanced: true}

	res := Compact(allCaps, shell("./job.sh | tee job.log", log), opts)
	if !res.Considered || !res.Replaced || res.Family != "" || res.KeptID == "" || res.EmittedBytes >= res.RawBytes {
		t.Fatalf("piped log: %+v", res)
	}
	if res.Output.Stderr != "" || !res.Output.HasStdout {
		t.Error("the shape flags or stderr changed")
	}

	if res := Compact(allCaps, shell("'/b/downshift' compact -- go test", log), opts); res.Considered {
		t.Error("a wrapped command was compacted again")
	}
	pre := Options{Keep: keepOK, Balanced: true, PreWrapped: func(string) bool { return true }}
	if res := Compact(allCaps, shell("go test ./...", log), pre); res.Considered {
		t.Error("a command the pre-run hook wrapped was compacted again")
	}
	if res := Compact(allCaps, shell("x | y", log), Options{Balanced: true}); res.Replaced || !res.Considered || res.Passthrough == "" {
		t.Errorf("no CCR: %+v, want a raw, logged run", res)
	}
	if res := Compact(allCaps, shell("x | y", log), Options{Keep: keepOK}); res.Replaced {
		t.Error("balanced off still cut")
	}
	op := shell("x | y", log)
	op.Opaque = true
	if res := Compact(allCaps, op, opts); res.Considered {
		t.Error("an opaque result was considered")
	}
	if res := Compact(core.HarnessCapabilities{}, shell("x | y", log), opts); res.Considered {
		t.Error("a harness without the capability was considered")
	}
}

// The marker names the session when the call has a valid hashed one, and is
// the bare `retrieve <id>` otherwise; KeptID stays the bare id.
func TestCompact_HintCarriesSession(t *testing.T) {
	in := hookport.ToolOutput{Category: core.CategoryShell, Command: "./job.sh | tee job.log", Stdout: bigLog(), HasStdout: true, ExitCode: -1}
	for _, tc := range []struct{ session, want string }{
		{"", "downshift retrieve 0123456789ab]"},
		{"not-a-hash", "downshift retrieve 0123456789ab]"},
		{strings.Repeat("ab", 32), "downshift retrieve --session=abababababab 0123456789ab]"},
	} {
		res := Compact(allCaps, in, Options{Keep: keepOK, Balanced: true, Session: tc.session})
		if !res.Replaced || res.KeptID != "0123456789ab" || !strings.Contains(res.Output.Stdout, tc.want) {
			t.Errorf("session %q: replaced=%v kept=%q, want hint %q", tc.session, res.Replaced, res.KeptID, tc.want)
		}
	}
}

func TestCompact_ReadThreshold(t *testing.T) {
	small := hookport.ToolOutput{Category: core.CategoryRead, Content: bigLog(), HasContent: true}
	if res := Compact(allCaps, small, Options{Keep: keepOK, Balanced: true}); !res.Considered || res.Replaced || res.Output.Content != small.Content {
		t.Errorf("Read under 64 KiB was modified: considered=%v replaced=%v", res.Considered, res.Replaced)
	}

	var b strings.Builder
	for i := 1; i <= 3000; i++ {
		fmt.Fprintf(&b, "line %04d: exact content for edit matching\n", i)
	}
	large := hookport.ToolOutput{Category: core.CategoryRead, Content: b.String(), HasContent: true}
	res := Compact(allCaps, large, Options{Keep: keepOK, Balanced: true})
	if !res.Considered || !res.Replaced || !res.Compacted || !res.Folded || res.Family != FamilyReadTool || res.KeptID != "0123456789ab" {
		t.Fatalf("large Read not compacted as expected: %+v", res)
	}
	if res.EmittedBytes >= res.RawBytes {
		t.Errorf("large Read did not shrink: raw=%d emitted=%d", res.RawBytes, res.EmittedBytes)
	}
}

func TestCompact_SearchUnderThresholdKept(t *testing.T) {
	in := hookport.ToolOutput{Category: core.CategorySearch, Paths: []string{"a.go", "b.go"}, HasPaths: true}
	res := Compact(allCaps, in, Options{Keep: keepOK, Balanced: true})
	if res.Replaced || !res.Considered || res.RawBytes != 10 {
		t.Errorf("small search: %+v", res)
	}
}

func TestCompact_StoreFailureLeavesRaw(t *testing.T) {
	fail := func([]byte) (string, bool) { return "", false }
	in := hookport.ToolOutput{Category: core.CategorySearch, Content: strings.Repeat("a.go:1:x err\n", 3000), HasContent: true}
	if res := Compact(allCaps, in, Options{Keep: fail, Balanced: true}); res.Replaced {
		t.Error("cut without stored raw text")
	}
	panicky := func([]byte) (string, bool) { panic("store") }
	if res := Compact(allCaps, in, Options{Keep: panicky, Balanced: true}); res.Replaced {
		t.Error("a panic in the store did not fail open")
	}
}

// A failed command's failure detail and exit code survive the post-run
// compaction: the cut may drop the passing lines, never the failure.
func TestCompact_ShellFailedRunKeepsItsDetailAndCode(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 300; i++ {
		fmt.Fprintf(&b, "ok  \tgithub.com/x/y/pkg%d\t0.01s\n", i)
	}
	b.WriteString("--- FAIL: TestSomething (0.00s)\n    thing_test.go:12: the failure detail\nFAIL\nFAIL\tgithub.com/x/y/bad\t0.01s\n")
	for _, tc := range []struct{ name, stderr string }{{"no stderr", ""}, {"stderr text", "# a note on stderr\n"}} {
		t.Run(tc.name, func(t *testing.T) {
			in := hookport.ToolOutput{Category: core.CategoryShell, Command: "go test ./...", Stdout: b.String(), Stderr: tc.stderr, HasStdout: true, HasStderr: true, ExitCode: 1}
			res := Compact(allCaps, in, Options{Keep: keepOK, Balanced: true})
			if res.ExitCode != 1 {
				t.Errorf("exit code = %d, want 1", res.ExitCode)
			}
			if !strings.Contains(res.Output.Stdout, "the failure detail") {
				t.Errorf("the failure detail is gone from %q", res.Output.Stdout)
			}
			if res.Output.Stderr != tc.stderr {
				t.Errorf("stderr = %q, want it as received %q", res.Output.Stderr, tc.stderr)
			}
			if res.Replaced && res.EmittedBytes >= res.RawBytes {
				t.Errorf("replaced with %d bytes for %d raw", res.EmittedBytes, res.RawBytes)
			}
		})
	}
}

// A small output is left exactly as it was: no marker, no rewrite, and the
// logged row says why.
func TestCompact_ShellSmallOutputStaysRaw(t *testing.T) {
	for _, stdout := range []string{"ok\n", "On branch main\nnothing to commit, working tree clean\n"} {
		in := hookport.ToolOutput{Category: core.CategoryShell, Command: "git status", Stdout: stdout, HasStdout: true, HasStderr: true, ExitCode: -1}
		res := Compact(allCaps, in, Options{Keep: keepOK, Balanced: true})
		if res.Output.Stdout != stdout && res.EmittedBytes >= res.RawBytes {
			t.Errorf("%q: output grew to %q", stdout, res.Output.Stdout)
		}
		if strings.Contains(res.Output.Stdout, "downshift compact") && !res.Replaced {
			t.Errorf("%q: a marker on an output that was not replaced: %q", stdout, res.Output.Stdout)
		}
	}
}

func TestCompact_CompoundCommandNamesItsShape(t *testing.T) {
	shell := func(cmd string) hookport.ToolOutput {
		return hookport.ToolOutput{Category: core.CategoryShell, Command: cmd, Stdout: "a\nb\n", HasStdout: true, ExitCode: 0}
	}
	res := Compact(allCaps, shell("go test ./... | tail -5"), Options{Keep: keepOK, Balanced: true})
	if !res.Considered || res.Inner != "go-test" || res.Kind != "pipe_other" {
		t.Errorf("compound: inner %q kind %q, want go-test pipe_other", res.Inner, res.Kind)
	}
	res = Compact(allCaps, shell("git status"), Options{Keep: keepOK, Balanced: true})
	if res.Inner != "" || res.Kind != "" {
		t.Errorf("simple command: inner %q kind %q, want none", res.Inner, res.Kind)
	}
}

func TestCompact_SearchLedCut(t *testing.T) {
	var b strings.Builder
	for i := 1; i <= 60; i++ {
		fmt.Fprintf(&b, "internal/execwrap/compound_search.go:%d:\tif keep == nil {\n", i)
	}
	stdout := b.String()
	in := hookport.ToolOutput{Category: core.CategoryShell, Command: "grep -rn keep internal | sort", Stdout: stdout, HasStdout: true, ExitCode: -1}
	res := Compact(allCaps, in, Options{Keep: keepOK, Balanced: true})
	if !res.Replaced || !res.Folded || res.KeptID != "0123456789ab" || !strings.Contains(res.Output.Stdout, "  … 35 more in internal/execwrap/compound_search.go\n") {
		t.Fatalf("not cut: replaced %v folded %v id %q\n%s", res.Replaced, res.Folded, res.KeptID, res.Output.Stdout)
	}
	if res := Compact(allCaps, in, Options{Keep: keepOK, Balanced: false}); res.Replaced {
		t.Errorf("cut with the balanced cut off:\n%s", res.Output.Stdout)
	}
	in.Command = "grep -rn keep internal | head -80"
	// Inside the agent's head the lossless grouping may run, but no row goes.
	res = Compact(allCaps, in, Options{Keep: keepOK, Balanced: true})
	if res.Replaced {
		if strings.Contains(res.Output.Stdout, " more in ") {
			t.Errorf("cut within the head count the agent asked for:\n%s", res.Output.Stdout)
		}
		for i := 1; i <= 60; i++ {
			if !strings.Contains(res.Output.Stdout, fmt.Sprintf("%d:\tif keep == nil {", i)) {
				t.Fatalf("row %d lost inside the head limit:\n%s", i, res.Output.Stdout)
			}
		}
	}
	in.Command = "grep -rn keep internal && sed -n 1,60p rows.txt"
	var rows strings.Builder
	for i := 1; i <= 40; i++ {
		fmt.Fprintf(&rows, "plain file text line %d\n", i) // distinct: identical rows would be folded losslessly
	}
	in.Stdout = stdout + rows.String()
	// A chained output is unclassified: just over the threshold it takes the
	// proportional cut (CCR keeps the original), like any other unclassified text.
	if res := Compact(allCaps, in, Options{Keep: keepOK, Balanced: true}); !res.Replaced || res.KeptID != "0123456789ab" {
		t.Errorf("chained unclassified output not cut:\n%s", res.Output.Stdout)
	}
	in.Stdout = stdout
	in.Command = "cat rows.txt | head -80"
	if res := Compact(allCaps, in, Options{Keep: keepOK, Balanced: true}); res.Replaced {
		t.Errorf("file text cut:\n%s", res.Output.Stdout)
	}
}

func TestCompact_PostRun4KBBand(t *testing.T) {
	if execwrap.BalanceThreshold != 16<<10 {
		t.Fatalf("execwrap.BalanceThreshold = %d, want 16 KiB", execwrap.BalanceThreshold)
	}
	if PostRunBalanceThreshold != 4<<10 {
		t.Fatalf("PostRunBalanceThreshold = %d, want 4 KiB", PostRunBalanceThreshold)
	}

	// ~5.5 KB log with >200 lines so balanced cut saves >25%.
	var b strings.Builder
	for i := 0; i < 220; i++ {
		fmt.Fprintf(&b, "step %03d ok: build check\n", i)
	}
	fiveKB := b.String()
	if len(fiveKB) <= 4<<10 || len(fiveKB) >= 16<<10 {
		t.Fatalf("fiveKB len = %d, want in (4 KiB, 16 KiB)", len(fiveKB))
	}

	opts := Options{Keep: keepOK, Balanced: true}
	in := hookport.ToolOutput{Category: core.CategoryShell, Command: "custom-tool --run", Stdout: fiveKB, HasStdout: true, ExitCode: 0}
	res := Compact(allCaps, in, opts)
	if !res.Considered || !res.Replaced || !res.Compacted || !res.Folded || res.KeptID != "0123456789ab" {
		t.Fatalf("5 KB exit 0 with CCR not compacted: %+v", res)
	}
	if res.EmittedBytes*4 > res.RawBytes*3 {
		t.Errorf("5 KB compaction did not achieve 25%% gain: raw=%d emitted=%d", res.RawBytes, res.EmittedBytes)
	}

	// Nonzero exit stays whole.
	failIn := in
	failIn.ExitCode = 1
	if failRes := Compact(allCaps, failIn, opts); failRes.Replaced || failRes.Output.Stdout != fiveKB {
		t.Errorf("5 KB nonzero exit was cut: replaced=%v", failRes.Replaced)
	}

	// Just over the threshold (80 lines * ~55 bytes = ~4.4 KB) the proportional byte cut keeps ~60% and still gains 25%.
	var smallGain strings.Builder
	for i := 0; i < 80; i++ {
		fmt.Fprintf(&smallGain, "row %03d: %s\n", i, strings.Repeat("x", 45))
	}
	sg := smallGain.String()
	if len(sg) <= 4<<10 {
		t.Fatalf("smallGain len = %d, want > 4 KiB", len(sg))
	}
	sgIn := hookport.ToolOutput{Category: core.CategoryShell, Command: "custom-tool --run", Stdout: sg, HasStdout: true, ExitCode: 0}
	sgRes := Compact(allCaps, sgIn, opts)
	if !sgRes.Replaced || sgRes.EmittedBytes*4 > sgRes.RawBytes*3 || sgRes.KeptID != "0123456789ab" {
		t.Errorf("4.4 KB near-threshold output not cut by 25%%: %+v", sgRes)
	}

	// Explicit Options.BalanceThreshold overrides PostRunBalanceThreshold.
	highThresh := Options{Keep: keepOK, Balanced: true, BalanceThreshold: 16 << 10}
	if highRes := Compact(allCaps, in, highThresh); highRes.Replaced || highRes.Output.Stdout != fiveKB {
		t.Errorf("explicit 16 KiB threshold still cut 5 KB output: replaced=%v", highRes.Replaced)
	}
}

func TestCompact_InnerAndLeadingFamily(t *testing.T) {
	var goTestOut strings.Builder
	for i := 0; i < 40; i++ {
		fmt.Fprintf(&goTestOut, "=== RUN   TestCase%d\n--- PASS: TestCase%d (0.00s)\n", i, i)
	}
	goTestOut.WriteString("PASS\nok  \texample.com/pkg\t0.123s\n")

	var gitStatusOut strings.Builder
	gitStatusOut.WriteString("On branch main\nChanges not staged for commit:\n")
	for i := 0; i < 30; i++ {
		fmt.Fprintf(&gitStatusOut, "\tmodified:   internal/pkg/file_%02d.go\n", i)
	}

	opts := Options{Keep: keepOK, Balanced: true}
	for _, tc := range []struct {
		name       string
		cmd        string
		stdout     string
		exit       int
		wantFamily execwrap.Family
		wantCut    bool
	}{
		{"cd compound go test exit 0", "cd repo && go test ./...", goTestOut.String(), 0, execwrap.FamilyGoTest, true},
		{"cd compound go test exit 1 stays unextracted", "cd repo && go test ./...", goTestOut.String(), 1, execwrap.FamilyNone, false},
		{"abs_path git status", "/opt/homebrew/bin/git status", gitStatusOut.String(), 0, execwrap.FamilyGitStatus, true},
		{"quoted go test", `CGO_ENABLED=0 go test -run "TestCase" ./...`, goTestOut.String(), 0, execwrap.FamilyGoTest, true},
		{"tee pipe stays unextracted", "go test ./... | tee out.log", goTestOut.String(), 0, execwrap.FamilyNone, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := hookport.ToolOutput{Category: core.CategoryShell, Command: tc.cmd, Stdout: tc.stdout, HasStdout: true, ExitCode: tc.exit}
			res := Compact(allCaps, in, opts)
			if res.Family != tc.wantFamily {
				t.Errorf("Family = %q, want %q", res.Family, tc.wantFamily)
			}
			if res.Replaced != tc.wantCut {
				t.Errorf("Replaced = %v, want %v (passthrough=%q detail=%q)", res.Replaced, tc.wantCut, res.Passthrough, res.Detail)
			}
		})
	}
}

func TestCompact_ShellDualKeptIDs(t *testing.T) {
	log := bigLog()
	in := hookport.ToolOutput{
		Category:  core.CategoryShell,
		Command:   "./job.sh | tee job.log",
		Stdout:    log,
		Stderr:    log,
		HasStdout: true,
		HasStderr: true,
		ExitCode:  0,
	}

	seq := 0
	ids := []string{"111111111111", "222222222222"}
	res := Compact(allCaps, in, Options{
		Keep: func([]byte) (string, bool) {
			id := ids[seq]
			seq++
			return id, true
		},
		Balanced: true,
	})
	if !res.Replaced || res.KeptID != "111111111111" || len(res.ExtraKeptIDs) != 1 || res.ExtraKeptIDs[0] != "222222222222" {
		t.Fatalf("dual kept IDs: kept=%q extra=%v replaced=%v", res.KeptID, res.ExtraKeptIDs, res.Replaced)
	}

	// Duplicate IDs across stdout and stderr are not repeated in ExtraKeptIDs.
	dup := Compact(allCaps, in, Options{Keep: keepOK, Balanced: true})
	if dup.KeptID != "0123456789ab" || len(dup.ExtraKeptIDs) != 0 {
		t.Fatalf("duplicate kept IDs: kept=%q extra=%v", dup.KeptID, dup.ExtraKeptIDs)
	}
}

func TestCompact_GlabAndSmallDiffPassthrough(t *testing.T) {
	t.Run("glab with secrets redacts without marker", func(t *testing.T) {
		raw := "title:\tWebhook delivery fails behind the corporate proxy\n" +
			"url:\thttps://gitlab.example.com/platform/webhooks/-/issues/311\n--\n" +
			"Deliveries time out when HTTPS_PROXY points at http://svc-proxy:pr0xyS3cretValue@proxy.corp.example.com:3128\n" +
			"and the request is sent with `PRIVATE-TOKEN: example-deploy-secret`.\n"
		res := Compact(allCaps, hookport.ToolOutput{
			Category:  core.CategoryShell,
			Command:   "glab issue view 311",
			Stdout:    raw,
			HasStdout: true,
		}, Options{Keep: keepOK})
		if !res.Replaced || !res.Compacted {
			t.Fatalf("expected glab secrets to be replaced: %+v", res)
		}
		if strings.Contains(res.Output.Stdout, "pr0xyS3cretValue") || strings.Contains(res.Output.Stdout, "example-deploy-secret") {
			t.Fatalf("glab secret leaked: %q", res.Output.Stdout)
		}
		if strings.Contains(res.Output.Stdout, "[downshift compact:") {
			t.Fatalf("unexpected marker on glab output: %q", res.Output.Stdout)
		}
		if len(res.Output.Stdout) >= len(raw) {
			t.Fatalf("redacted glab grew: %d >= %d", len(res.Output.Stdout), len(raw))
		}
	})

	t.Run("glab without secrets stays raw", func(t *testing.T) {
		raw := "Showing 2 pipelines\n(success) #1\n"
		res := Compact(allCaps, hookport.ToolOutput{
			Category:  core.CategoryShell,
			Command:   "glab ci list",
			Stdout:    raw,
			HasStdout: true,
		}, Options{Keep: keepOK})
		if res.Replaced || res.Compacted || res.Output.Stdout != raw {
			t.Fatalf("plain glab changed: %+v", res)
		}
	})

	t.Run("small git diff passes through without marker", func(t *testing.T) {
		raw := "diff --git a/src/main.go b/src/main.go\nindex 73d83e6..63294b2 100644\n--- a/src/main.go\n+++ b/src/main.go\n@@ -1,5 +1,5 @@\n package main\n \n func main() {\n-\tprintln(\"hello, world\")\n+\tprintln(\"hello, team\")\n }\n"
		res := Compact(allCaps, hookport.ToolOutput{
			Category:  core.CategoryShell,
			Command:   "git diff",
			Stdout:    raw,
			HasStdout: true,
		}, Options{Keep: keepOK})
		if res.Replaced || res.Compacted || res.Output.Stdout != raw || res.Passthrough != execwrap.PassthroughSmallOutput {
			t.Fatalf("small git diff not passed through raw: %+v", res)
		}
	})
}
