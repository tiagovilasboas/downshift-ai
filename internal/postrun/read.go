// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package postrun

import (
	"fmt"
	"strings"

	"github.com/tiagovilasboas/downshift-ai/internal/execwrap"
	"github.com/tiagovilasboas/downshift-ai/internal/hookport"
)

// ReadThreshold is the minimum Read result size (64 KiB) before middle-range
// compaction triggers. Smaller reads stay whole.
const ReadThreshold = 64 << 10

// readHeadBudget and readTailBudget bound the verbatim head and tail slices
// kept from a large Read result (~16 KiB total).
const (
	readHeadBudget = 8 << 10
	readTailBudget = 8 << 10
)

// FamilyReadTool is the exec log family of a file read tool's result.
const FamilyReadTool execwrap.Family = "read-tool"

type lineSpan struct {
	start int
	end   int
}

// compactRead cuts only whole middle line ranges from a large Read result
// (> ReadThreshold), keeping head and tail lines byte-for-byte verbatim so
// subsequent Edit calls match exact text. Raw content is stored in CCR first;
// any failure leaves the result uncompacted.
func compactRead(in hookport.ToolOutput, o Options) Result {
	if !in.HasContent || in.Content == "" {
		return Result{}
	}
	raw := len(in.Content)
	res := Result{
		Considered:   true,
		Family:       FamilyReadTool,
		Output:       in,
		RawBytes:     int64(raw),
		EmittedBytes: int64(raw),
		Passthrough:  execwrap.PassthroughSmallOutput,
	}
	if raw <= ReadThreshold || !o.Balanced || o.Keep == nil {
		return settle(res, in)
	}
	spans := splitLineSpans(in.Content)
	headCount, tailCount := pickHeadTail(spans, readHeadBudget, readTailBudget)
	if headCount == 0 || tailCount == 0 || headCount+tailCount >= len(spans) {
		return settle(res, in)
	}
	id, ok := o.Keep([]byte(in.Content))
	if !ok || id == "" {
		return settle(res, in)
	}
	arg := readRetrieveArg(o.Session, id)
	cut := buildReadCut(in.Content, spans, headCount, tailCount, arg)
	res.Output.Content = cut
	res.EmittedBytes = int64(len(cut))
	res.KeptID = id
	res.Compacted = true
	res.Folded = true
	res.Passthrough = ""
	return settle(res, in)
}

// splitLineSpans returns [start, end) byte offsets for each line in s,
// preserving exact line endings.
func splitLineSpans(s string) []lineSpan {
	var spans []lineSpan
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			spans = append(spans, lineSpan{start: start, end: i + 1})
			start = i + 1
		}
	}
	if start < len(s) {
		spans = append(spans, lineSpan{start: start, end: len(s)})
	}
	return spans
}

// pickHeadTail selects how many whole lines from the start and end fit within
// headBudget and tailBudget bytes, leaving at least one line in the middle.
func pickHeadTail(spans []lineSpan, headBudget, tailBudget int) (headCount, tailCount int) {
	if len(spans) < 3 {
		return 0, 0
	}
	used := 0
	for i := 0; i < len(spans)-2; i++ {
		lineLen := spans[i].end - spans[i].start
		if i > 0 && used+lineLen > headBudget {
			break
		}
		used += lineLen
		headCount++
	}
	used = 0
	for i := len(spans) - 1; i > headCount; i-- {
		lineLen := spans[i].end - spans[i].start
		if tailCount > 0 && used+lineLen > tailBudget {
			break
		}
		used += lineLen
		tailCount++
	}
	return headCount, tailCount
}

// buildReadCut assembles verbatim head + range marker + verbatim tail + CCR marker.
func buildReadCut(raw string, spans []lineSpan, headCount, tailCount int, retrieveArg string) string {
	head := raw[:spans[headCount-1].end]
	tailIdx := len(spans) - tailCount
	tail := raw[spans[tailIdx].start:]

	firstOmitted := headCount + 1
	lastOmitted := tailIdx
	omitted := lastOmitted - firstOmitted + 1

	var b strings.Builder
	b.Grow(len(head) + len(tail) + 256)
	b.WriteString(head)
	if !strings.HasSuffix(head, "\n") {
		b.WriteByte('\n')
	}
	fmt.Fprintf(&b, "… [lines %d-%d omitted (%d lines); Read(offset=%d, limit=%d) or downshift retrieve %s] …\n",
		firstOmitted, lastOmitted, omitted, firstOmitted, omitted, retrieveArg)
	b.WriteString(tail)
	if !strings.HasSuffix(tail, "\n") {
		b.WriteByte('\n')
	}
	bodyLen := b.Len()
	b.WriteString(execwrap.MarkerWithID(len(raw), bodyLen, retrieveArg))
	return b.String()
}

// readRetrieveArg formats the retrieve argument with an optional --session ref.
func readRetrieveArg(session, id string) string {
	if ref := execwrap.SessionRef(session); ref != "" {
		return "--session=" + ref + " " + id
	}
	return id
}
