// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package postrun compacts a tool's result after the tool ran, for a harness
// whose hook can replace that result before the model reads it
// (core.HarnessCapabilities.ReplacesToolOutput). It is harness-free: an
// adapter reads its payload into a hookport.ToolOutput and writes the answer
// back; this package only decides what the text becomes.
//
// Shell output goes through the same steps as `downshift compact`
// (execwrap.CompactCaptured): the family's compressor when the command is one
// plain allowlisted program, otherwise only the balanced cut, which needs the
// raw output stored in CCR and a gain of at least a quarter. Output of a
// command led by a search keeps the first rows of each file instead, at any
// size, with the raw output stored (execwrap.CutSearch). Search results
// (matches and file lists) are cut only when larger than the balanced
// threshold, with the raw text stored first. Large file reads (> ReadThreshold)
// cut only whole middle ranges with the raw text stored first, keeping head
// and tail lines verbatim so subsequent edits still match.
package postrun

import (
	"github.com/tiagovilasboas/downshift-ai/internal/core"
	"github.com/tiagovilasboas/downshift-ai/internal/execwrap"
	"github.com/tiagovilasboas/downshift-ai/internal/hookport"
)

// Env turns post-run compaction off when "0". It is on by default wherever
// the harness declares ReplacesToolOutput.
const Env = "DOWNSHIFT_POST_COMPACT"

// Entry is the exec log's entry code for a run compacted after it ran.
const Entry = "post_tool_use"

// FamilySearchTool is the exec log family of a search tool's result (a
// harness's own grep or glob tool, not a shell command).
const FamilySearchTool execwrap.Family = "search-tool"

// PostRunBalanceThreshold is the default size (4 KiB) above which post-run
// shell compaction applies the balanced cut when Options.BalanceThreshold <= 0.
const PostRunBalanceThreshold = 4 << 10

// Options are the ports and switches the runner reads at the edge.
type Options struct {
	// Keep stores raw output for `downshift retrieve` (CCR). Nil means
	// nothing is stored, so nothing is cut with loss.
	Keep func(raw []byte) (id string, ok bool)
	// Balanced allows the balanced cut (DOWNSHIFT_BALANCED).
	Balanced bool
	// BalanceThreshold overrides PostRunBalanceThreshold when positive.
	BalanceThreshold int
	// Session is the hashed session of the call, empty when unknown. With it
	// the marker's hint reads `downshift retrieve --session=<ref> <id>`.
	Session string
	// PreWrapped reports whether the harness's pre-run hook already wrapped
	// this shell command in `downshift compact`, so its output was compacted
	// there. Nil means no pre-run wrap is known.
	PreWrapped func(command string) bool
}

func balanceThreshold(o Options) int {
	if o.BalanceThreshold > 0 {
		return o.BalanceThreshold
	}
	return PostRunBalanceThreshold
}

// Result is what Compact did with one tool result. It carries byte counts and
// codes for the exec log, never the text it logged nothing of.
type Result struct {
	// Considered is true when the output went through the compactor; the
	// runner logs one exec row for it. False: not this package's to handle
	// (wrapped before the run, an opaque result, nothing printed).
	Considered bool
	// Replaced is true when Output differs from the input and is shorter.
	Replaced bool
	Output   hookport.ToolOutput
	Family   execwrap.Family
	// RawBytes and EmittedBytes count the text fields before and after.
	RawBytes, EmittedBytes int64
	Compacted, Folded      bool
	Passthrough, Detail    string
	KeptID                 string
	ExtraKeptIDs           []string
	ExitCode               int
	// Inner and Kind describe a shell command with no family that the
	// policy refuses for a metacharacter: execwrap.CompoundShape. Empty otherwise.
	Inner, Kind string
	// Head and LineBucket describe a command with no family: codes from
	// execwrap.CommandHead and execwrap.LineBucket, never text.
	Head, LineBucket string
}

// Compact returns the compaction of in, a finished tool call on a harness
// with caps. Every failure leaves the result unreplaced: a panic, a category
// the harness cannot replace or this package does not handle.
func Compact(caps core.HarnessCapabilities, in hookport.ToolOutput, o Options) (res Result) {
	defer func() {
		if recover() != nil {
			res = Result{}
		}
	}()
	if in.Opaque || !caps.ReplacesOutputOf(in.Category) {
		return Result{}
	}
	switch in.Category {
	case core.CategoryShell:
		return compactShell(in, o)
	case core.CategorySearch:
		return compactSearch(in, o)
	case core.CategoryRead:
		return compactRead(in, o)
	case core.CategoryOther:
		return compactVerboseText(in, o)
	}
	return Result{}
}

// compactShell compacts stdout as the wrapper would, and stderr only by the
// balanced cut when it alone is over the threshold.
func compactShell(in hookport.ToolOutput, o Options) Result {
	if !in.HasStdout || in.Stdout == "" && in.Stderr == "" {
		return Result{}
	}
	if execwrap.IsWrapped(in.Command) || execwrap.HasMarker(in.Stdout) || (o.PreWrapped != nil && o.PreWrapped(in.Command)) {
		return Result{}
	}
	exit := in.ExitCode
	if exit < 0 {
		exit = 0 // a post-run event for a successful call carries no code
	}
	if exit == 0 {
		if ptr, id, ok := dedupShellRead(in.Command, in.Stdout, o, in.Session); ok {
			res := Result{Considered: true, Family: execwrap.FamilyNone, Output: in, Compacted: true, Folded: true, KeptID: id}
			res.Head = execwrap.CommandHead(in.Command)
			res.RawBytes = int64(len(in.Stdout) + len(in.Stderr))
			res.Output.Stdout = ptr
			res.EmittedBytes = int64(len(ptr) + len(in.Stderr))
			return settle(res, in)
		}
	}
	family, argv := execwrap.SimpleFamily(in.Command)
	res := Result{Considered: true, Family: family, ExitCode: exit, Output: in}
	if family == execwrap.FamilyNone {
		res.Inner, res.Kind = execwrap.CompoundShape(in.Command)
		if fam, extractedArgv := extractPostRunFamily(in.Command, exit); fam != execwrap.FamilyNone {
			family, argv = fam, extractedArgv
			res.Family = fam
		}
	}
	res.RawBytes = int64(len(in.Stdout) + len(in.Stderr))
	if res.Family == execwrap.FamilyNone {
		res.Head = execwrap.CommandHead(in.Command)
		res.LineBucket = execwrap.LineBucket(execwrap.CountLines(in.Stdout) + execwrap.CountLines(in.Stderr))
	}
	// A compound command that runs curl, wget or glab may carry credentials in
	// its output: offer no store, so the output stays raw (never cut unstored).
	if family == execwrap.FamilyNone && execwrap.CompoundRefusesRawStore(in.Command) {
		o.Keep = nil
	}
	out, cut := execwrap.Captured{}, false
	if family == execwrap.FamilyNone && o.Balanced {
		out, cut = execwrap.CutSearch(in.Command, []byte(in.Stdout), exit, o.Keep, o.Session)
	}
	balanced := o.Balanced && exit == 0
	threshold := balanceThreshold(o)
	if !cut {
		out = execwrap.CompactCapturedWithThreshold(argv, family, []byte(in.Stdout), exit, o.Keep, balanced, threshold, o.Session)
	}
	res.Compacted, res.Folded, res.Passthrough, res.Detail, res.KeptID = out.Compacted, out.Folded, out.Passthrough, out.Detail, out.KeptID
	if out.Compacted {
		res.Output.Stdout = string(out.Output)
	}
	if in.HasStderr && len(in.Stderr) > threshold {
		errOut := execwrap.CompactCapturedWithThreshold(nil, execwrap.FamilyNone, []byte(in.Stderr), exit, o.Keep, balanced, threshold, o.Session)
		if errOut.Compacted {
			res.Output.Stderr = string(errOut.Output)
			res.Compacted, res.Folded, res.Passthrough, res.Detail = true, res.Folded || errOut.Folded, "", ""
			recordKeptID(&res, errOut.KeptID)
		}
	}
	res.EmittedBytes = int64(len(res.Output.Stdout) + len(res.Output.Stderr))
	return settle(res, in)
}

func recordKeptID(res *Result, id string) {
	if id == "" {
		return
	}
	if res.KeptID == "" {
		res.KeptID = id
		return
	}
	if res.KeptID == id {
		return
	}
	for _, extra := range res.ExtraKeptIDs {
		if extra == id {
			return
		}
	}
	res.ExtraKeptIDs = append(res.ExtraKeptIDs, id)
}

// settle undoes a result that would not shorten the output: the row then
// says the output stayed raw because the summary was not smaller.
func settle(res Result, in hookport.ToolOutput) Result {
	res.Replaced = res.Compacted && (res.EmittedBytes < res.RawBytes || res.Family == execwrap.FamilyGlab)
	if res.Replaced {
		return res
	}
	if res.Compacted {
		res.Compacted, res.Folded, res.KeptID, res.ExtraKeptIDs, res.Passthrough, res.Detail = false, false, "", nil, execwrap.PassthroughSmallOutput, ""
	}
	res.Output, res.EmittedBytes = in, res.RawBytes
	return res
}
