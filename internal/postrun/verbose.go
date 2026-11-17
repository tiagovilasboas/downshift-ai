// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package postrun

import (
	"github.com/tiagovilasboas/downshift-ai/internal/compressor"
	"github.com/tiagovilasboas/downshift-ai/internal/execwrap"
	"github.com/tiagovilasboas/downshift-ai/internal/hookport"
)

const (
	// VerboseKindFetch and VerboseKindMCP identify verbose CategoryOther tools.
	VerboseKindFetch = "fetch"
	VerboseKindMCP   = "mcp"
	// FamilyFetchTool is the exec log family of a web fetch tool's result.
	FamilyFetchTool execwrap.Family = "fetch-tool"
	// FamilyMCPTool is the exec log family of an MCP tool's result.
	FamilyMCPTool execwrap.Family = "mcp-tool"
)

func verboseFamily(kind string) (execwrap.Family, bool) {
	switch kind {
	case VerboseKindFetch:
		return FamilyFetchTool, true
	case VerboseKindMCP:
		return FamilyMCPTool, true
	default:
		return "", false
	}
}

// compactVerboseText applies safe balanced compaction (BalanceLog) to verbose
// non-shell tool outputs such as WebFetch and mcp__* when they exceed
// balanceThreshold(o) and CCR storage succeeds.
func compactVerboseText(in hookport.ToolOutput, o Options) Result {
	family, ok := verboseFamily(in.VerboseKind)
	if !ok || !in.HasContent || in.Content == "" {
		return Result{}
	}
	raw := len(in.Content)
	res := Result{
		Considered:   true,
		Family:       family,
		Output:       in,
		RawBytes:     int64(raw),
		EmittedBytes: int64(raw),
		Passthrough:  execwrap.PassthroughSmallOutput,
	}
	if in.ExitCode > 0 {
		res.ExitCode = in.ExitCode
		res.Passthrough = execwrap.PassthroughUnrecognized
		return settle(res, in)
	}
	threshold := balanceThreshold(o)
	if raw <= threshold || !o.Balanced || o.Keep == nil {
		return settle(res, in)
	}
	cut, applied := compressor.Balance(compressor.BalanceLog, []byte(in.Content))
	if !applied {
		return settle(res, in)
	}
	id, ok := o.Keep([]byte(in.Content))
	if !ok || id == "" {
		return settle(res, in)
	}
	arg := readRetrieveArg(o.Session, id)
	res.Output.Content = string(cut) + execwrap.MarkerWithID(raw, len(cut), arg)
	res.EmittedBytes = int64(len(res.Output.Content))
	res.KeptID = id
	res.Compacted = true
	res.Folded = true
	res.Passthrough = ""
	return settle(res, in)
}
