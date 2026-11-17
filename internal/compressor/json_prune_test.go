// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestJSONPruneStripsMetadataAndPreservesCore(t *testing.T) {
	payload := map[string]any{
		"id":                18234567890,
		"node_id":           "MDEwOlJlcG9zaXRvcnkxMjM0NTY=",
		"name":              "downshift-ai",
		"full_name":         "tiagovilasboas/downshift-ai",
		"state":             "open",
		"title":             "feat: prune json",
		"url":               "https://api.github.com/repos/tiagovilasboas/downshift-ai",
		"html_url":          "https://github.com/tiagovilasboas/downshift-ai",
		"followers_url":     "https://api.github.com/users/tiagovilasboas/followers",
		"avatar_url":        "https://avatars.githubusercontent.com/u/1?v=4",
		"gravatar_id":       "",
		"issues_url":        "https://api.github.com/repos/tiagovilasboas/downshift-ai/issues{/number}",
		"pulls_url":         "https://api.github.com/repos/tiagovilasboas/downshift-ai/pulls{/number}",
		"notifications_url": "https://api.github.com/repos/tiagovilasboas/downshift-ai/notifications{?since,all,participating}",
		"owner": map[string]any{
			"login":      "tiagovilasboas",
			"id":         42,
			"node_id":    "MDQ6VXNlcjQy",
			"avatar_url": "https://avatars.githubusercontent.com/u/42?v=4",
			"html_url":   "https://github.com/tiagovilasboas",
		},
	}
	raw, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	raw = append(raw, '\n')

	res := CompressExit(raw, ModeSafe, 0)
	if !res.Applied || res.Format != FormatJSONPrune {
		t.Fatalf("expected json_prune applied, got applied=%v format=%s reason=%q", res.Applied, res.Format, res.Reason)
	}
	if res.ReducedBytes >= res.OriginalBytes {
		t.Fatalf("expected smaller output: %d >= %d", res.ReducedBytes, res.OriginalBytes)
	}
	out := string(res.Output)
	for _, stripped := range []string{"node_id", "html_url", "followers_url", "avatar_url", "gravatar_id", "issues_url", "pulls_url"} {
		if strings.Contains(out, stripped) {
			t.Errorf("expected %q to be stripped from output:\n%s", stripped, out)
		}
	}
	for _, kept := range []string{"18234567890", "downshift-ai", "tiagovilasboas", `"state": "open"`, `"title": "feat: prune json"`, `"url": "https://api.github.com/repos/tiagovilasboas/downshift-ai"`} {
		if !strings.Contains(out, kept) {
			t.Errorf("expected %q to be preserved in output:\n%s", kept, out)
		}
	}
}

func TestJSONPruneTruncatesLongArrays(t *testing.T) {
	items := make([]map[string]any, 35)
	for i := range items {
		items[i] = map[string]any{
			"id":       i + 1,
			"node_id":  fmt.Sprintf("NODE_%d", i+1),
			"html_url": fmt.Sprintf("https://github.com/x/y/issues/%d", i+1),
			"title":    fmt.Sprintf("Issue %d", i+1),
		}
	}
	raw, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	raw = append(raw, '\n')

	res := CompressExit(raw, ModeSafe, 0)
	if !res.Applied || res.Format != FormatJSONPrune {
		t.Fatalf("expected json_prune applied, got applied=%v format=%s reason=%q", res.Applied, res.Format, res.Reason)
	}
	var parsed []any
	if err := json.Unmarshal(res.Output, &parsed); err != nil {
		t.Fatalf("pruned output is not valid JSON: %v\n%s", err, res.Output)
	}
	if len(parsed) != jsonPruneMaxArray+1 {
		t.Fatalf("expected %d elements, got %d", jsonPruneMaxArray+1, len(parsed))
	}
	if last, ok := parsed[len(parsed)-1].(string); !ok || last != "... (15 more items)" {
		t.Fatalf("expected summary element, got %#v", parsed[len(parsed)-1])
	}
}

func TestJSONPruneEdgeCases(t *testing.T) {
	small := []byte(`{"id":1,"node_id":"abc","html_url":"https://example.com"}`)
	if res := CompressExit(small, ModeSafe, 0); res.Applied || string(res.Output) != string(small) {
		t.Errorf("small JSON should pass through: applied=%v", res.Applied)
	}

	malformed := []byte(`{"id": 1, "node_id": "` + strings.Repeat("x", 600))
	if res := CompressExit(malformed, ModeSafe, 0); res.Applied || string(res.Output) != string(malformed) {
		t.Errorf("malformed JSON should fall back cleanly: applied=%v", res.Applied)
	}

	validBig := []byte(`{"id":1,"node_id":"` + strings.Repeat("a", 600) + `","name":"ok"}`)
	if res := CompressExit(validBig, ModeSafe, 1); res.Applied || string(res.Output) != string(validBig) {
		t.Errorf("non-zero exit must preserve raw output: applied=%v", res.Applied)
	}
}

func TestHTTPFetchJSONPruned(t *testing.T) {
	body := `{"id":99,"node_id":"XYZ","avatar_url":"https://example.com/a.png","followers_url":"https://example.com/f","login":"alice","bio":"` + strings.Repeat("dev ", 150) + `"}`
	res := CompressCommandOutputHint([]byte(body), FormatHTTPFetch, 0, CommandHint{})
	if !res.Applied {
		t.Fatalf("expected bare curl JSON to be pruned, reason=%q", res.Reason)
	}
	if strings.Contains(string(res.Output), "node_id") || strings.Contains(string(res.Output), "avatar_url") {
		t.Errorf("expected metadata stripped:\n%s", res.Output)
	}

	withHeaders := "HTTP/2 200\r\ncontent-type: application/json\r\nauthorization: Bearer secret123\r\n\r\n" + body
	res = CompressCommandOutputHint([]byte(withHeaders), FormatHTTPFetch, 0, CommandHint{HTTPHeaders: true})
	if !res.Applied {
		t.Fatalf("expected HTTP JSON with headers to be pruned, reason=%q", res.Reason)
	}
	if strings.Contains(string(res.Output), "secret123") || strings.Contains(string(res.Output), "node_id") {
		t.Errorf("expected secret redacted and metadata stripped:\n%s", res.Output)
	}
}
