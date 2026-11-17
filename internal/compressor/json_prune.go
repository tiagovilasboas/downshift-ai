// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

const (
	// JSONPruneMinBytes is the shortest JSON payload eligible for pruning.
	JSONPruneMinBytes = 512
	jsonPruneMaxArray = 20
	jsonPruneMaxURL   = 80
)

// jsonNoisyKeys are GitHub API / REST metadata keys stripped during pruning.
var jsonNoisyKeys = map[string]bool{
	"node_id":     true,
	"gravatar_id": true,
	"_links":      true,
}

// isJSONPrunable reports whether input is a valid JSON object or array above
// JSONPruneMinBytes that becomes strictly smaller after pruning.
func isJSONPrunable(input []byte) bool {
	trimmed := bytes.TrimSpace(input)
	if len(trimmed) < JSONPruneMinBytes {
		return false
	}
	if trimmed[0] != '{' && trimmed[0] != '[' {
		return false
	}
	// agent-desktop snapshots are handled by FormatAgentDesktop.
	if bytes.Contains(trimmed, []byte(`"snapshot_id":`)) {
		return false
	}
	_, ok, _ := compressJSONPrune(input)
	return ok
}

// compressJSONPrune recursively strips noisy metadata keys (*_url, node_id,
// gravatar_id, _links, and long url strings) and truncates arrays longer than
// jsonPruneMaxArray with a "... (N more items)" summary element.
func compressJSONPrune(input []byte) ([]byte, bool, string) {
	if len(bytes.TrimSpace(input)) < JSONPruneMinBytes {
		return input, false, "no_compression_gain"
	}
	val, err := decodeSingleJSON(input)
	if err != nil {
		return input, false, "json_invalid_preserved"
	}
	pruned, changed := pruneJSONValue(val)
	if !changed {
		return input, false, "no_compression_gain"
	}
	out, err := marshalPrunedJSON(pruned, len(input))
	if err != nil || len(out) >= len(input) {
		return input, false, "no_compression_gain"
	}
	return out, true, "JSON metadata pruned"
}

// decodeSingleJSON parses exactly one JSON value, preserving numeric literals.
func decodeSingleJSON(input []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(input))
	dec.UseNumber()
	var val any
	if err := dec.Decode(&val); err != nil {
		return nil, err
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, fmt.Errorf("trailing data after JSON document")
	}
	return val, nil
}

// pruneJSONValue recursively strips noisy keys and truncates long arrays.
func pruneJSONValue(v any) (any, bool) {
	switch x := v.(type) {
	case map[string]any:
		return pruneJSONObject(x)
	case []any:
		return pruneJSONArray(x)
	default:
		return v, false
	}
}

func pruneJSONObject(obj map[string]any) (map[string]any, bool) {
	out := make(map[string]any, len(obj))
	changed := false
	for k, v := range obj {
		if isNoisyJSONKey(k, v) {
			changed = true
			continue
		}
		pv, childChanged := pruneJSONValue(v)
		if childChanged {
			changed = true
		}
		out[k] = pv
	}
	return out, changed
}

func pruneJSONArray(arr []any) ([]any, bool) {
	alreadyTruncated := len(arr) == jsonPruneMaxArray+1 && isArrayTruncateMarker(arr[jsonPruneMaxArray])
	limit := len(arr)
	truncated := 0
	if !alreadyTruncated && limit > jsonPruneMaxArray {
		truncated = limit - jsonPruneMaxArray
		limit = jsonPruneMaxArray
	}
	out := make([]any, 0, limit+1)
	changed := truncated > 0
	for i := 0; i < limit; i++ {
		pv, childChanged := pruneJSONValue(arr[i])
		if childChanged {
			changed = true
		}
		out = append(out, pv)
	}
	if truncated > 0 {
		out = append(out, fmt.Sprintf("... (%d more items)", truncated))
	}
	return out, changed
}

func isArrayTruncateMarker(v any) bool {
	s, ok := v.(string)
	return ok && strings.HasPrefix(s, "... (") && strings.HasSuffix(s, " more items)")
}

func isNoisyJSONKey(key string, val any) bool {
	if jsonNoisyKeys[key] || strings.HasSuffix(key, "_url") {
		return true
	}
	if key == "url" {
		if s, ok := val.(string); ok && len(s) > jsonPruneMaxURL {
			return true
		}
	}
	return false
}

func marshalPrunedJSON(v any, origLen int) ([]byte, error) {
	indented, err := encodeJSON(v, true)
	if err == nil && len(indented) < origLen {
		return indented, nil
	}
	return encodeJSON(v, false)
}

func encodeJSON(v any, indent bool) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if indent {
		enc.SetIndent("", "  ")
	}
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
