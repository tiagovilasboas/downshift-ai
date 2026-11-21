// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package postrun

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/core"
	"github.com/tiagovilasboas/downshift-ai/internal/hookport"
)

// TestCompact_MCPEscapedResultUnwrapped shows the unwrap pass cuts a
// {"result":"<escaped pretty JSON>"} body further than the old path, and that
// CCR still stores the original bytes.
func TestCompact_MCPEscapedResultUnwrapped(t *testing.T) {
	caps := core.HarnessCapabilities{ReplacesToolOutput: []core.ToolCategory{core.CategoryOther}}
	rows := make([]map[string]any, 12)
	for i := range rows {
		rows[i] = map[string]any{"id": i, "name": fmt.Sprintf("row %d", i), "total": 1234567890123 + i, "ok": true}
	}
	inner, err := json.MarshalIndent(rows, "", "    ")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(map[string]string{"result": string(inner)})
	if err != nil {
		t.Fatal(err)
	}

	// Old behaviour: the same body without the unwrap pass is the raw size.
	var stored []byte
	keep := func(b []byte) (string, bool) { stored = append([]byte(nil), b...); return "ccrunwrap0001", true }
	res := Compact(caps, hookport.ToolOutput{
		Category: core.CategoryOther, VerboseKind: VerboseKindMCP,
		Content: string(raw), HasContent: true, ExitCode: -1,
	}, Options{Keep: keep, Balanced: true, Session: strings.Repeat("ab", 32)})
	if !res.Compacted || !res.Replaced {
		t.Fatalf("expected compaction: %+v", res)
	}
	if string(stored) != string(raw) {
		t.Errorf("CCR must store original bytes")
	}
	body := res.Output.Content
	if strings.Contains(body, `\n`) || strings.Contains(body, `\"`) {
		t.Errorf("body still escaped:\n%s", body)
	}
	if !strings.Contains(body, `"total":1234567890123`) {
		t.Errorf("values lost:\n%s", body)
	}
	if res.EmittedBytes*100 > res.RawBytes*60 {
		t.Errorf("expected >40%% cut, raw=%d emitted=%d", res.RawBytes, res.EmittedBytes)
	}
	t.Logf("raw=%d emitted=%d", res.RawBytes, res.EmittedBytes)
}
