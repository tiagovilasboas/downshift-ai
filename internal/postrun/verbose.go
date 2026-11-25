// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package postrun

import (
	"bytes"

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

// compactVerboseText applies semantic JSON pruning (for mcp__* JSON payloads
// >= JSONPruneMinBytes) followed by safe balanced compaction (BalanceLog)
// when outputs still exceed balanceThreshold(o) and CCR storage succeeds.
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
	if isRetrieveTool(in.ToolName) {
		if in.ExitCode > 0 {
			res.ExitCode = in.ExitCode
		}
		res.Passthrough = execwrap.PassthroughRetrieve
		return settle(res, in)
	}
	if in.ExitCode > 0 {
		res.ExitCode = in.ExitCode
		res.Passthrough = execwrap.PassthroughUnrecognized
		return settle(res, in)
	}
	if o.Keep == nil {
		return settle(res, in)
	}
	if ptr, id, ok := dedupMCPDoc(in, o); ok {
		res.Output.Content = ptr
		res.EmittedBytes = int64(len(ptr))
		res.KeptID = id
		res.Compacted = true
		res.Folded = true
		res.Passthrough = ""
		return settle(res, in)
	}
	cut, applied := compactVerboseBody(in.VerboseKind, in.ToolName, []byte(in.Content), balanceThreshold(o), o.Balanced)
	if !applied {
		return settle(res, in)
	}
	if !bytes.HasSuffix(cut, []byte("\n")) {
		cut = append(append([]byte(nil), cut...), '\n')
	}
	worstMarker := execwrap.MarkerWithID(raw, len(cut), readRetrieveArg(o.Session, "000000000000"))
	if len(cut)+len(worstMarker) >= raw {
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

func compactVerboseBody(kind, toolName string, raw []byte, threshold int, balanced bool) ([]byte, bool) {
	cur := raw
	changed := false
	if kind == VerboseKindMCP {
		if u, ok := compressor.UnwrapJSONResult(cur); ok {
			cur = u
			changed = true
		}
		if p, ok := compressor.PruneMCP(toolName, cur); ok {
			cur = p
			changed = true
		}
	}
	if kind == VerboseKindMCP && len(cur) >= compressor.JSONPruneMinBytes {
		if r := compressor.CompressCommandOutput(cur, compressor.FormatJSONPrune, 0); r.Applied {
			cur = r.Output
			changed = true
		}
	}
	if len(cur) > threshold && balanced {
		if cut, ok := compressor.Balance(compressor.BalanceLog, cur); ok {
			cur = cut
			changed = true
		} else if cut, ok := compressor.BalanceBytesScaled(cur); ok {
			cur = cut
			changed = true
		}
	}
	return cur, changed
}
