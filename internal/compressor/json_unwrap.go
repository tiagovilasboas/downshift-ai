// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"bytes"
	"encoding/json"
)

// jsonUnwrapMinGain is the smallest saving (percent of input) worth emitting.
const jsonUnwrapMinGain = 5

// unwrapKeys are the wrapper fields whose string value may hold escaped JSON.
var unwrapKeys = map[string]bool{"result": true, "text": true, "content": true}

type jsonMember struct {
	key string
	val json.RawMessage
}

// UnwrapJSONResult is a lossless pass for MCP bodies. It compacts a pretty
// JSON object/array and, when a wrapper field (the only field, or one of
// result/text/content) holds a string that is itself a JSON object/array,
// replaces that string by the inner JSON as raw compact JSON. Key order and
// every value byte (numbers, escapes) are kept. It reports false when data is
// not JSON, nothing could be unwrapped, or the saving is under 5%.
func UnwrapJSONResult(data []byte) ([]byte, bool) {
	trim := bytes.TrimSpace(data)
	if len(trim) == 0 || (trim[0] != '{' && trim[0] != '[') || !json.Valid(trim) {
		return nil, false
	}
	out := unwrapObject(trim)
	if out == nil {
		var buf bytes.Buffer
		if json.Compact(&buf, trim) != nil {
			return nil, false
		}
		out = buf.Bytes()
	}
	if len(out)*100 > len(data)*(100-jsonUnwrapMinGain) {
		return nil, false
	}
	return out, true
}

// unwrapObject returns the rebuilt compact object when a wrapper string was
// unwrapped, or nil.
func unwrapObject(b []byte) []byte {
	if b[0] != '{' {
		return nil
	}
	members, ok := parseMembers(b)
	if !ok || len(members) == 0 {
		return nil
	}
	changed := false
	for i, m := range members {
		if len(members) != 1 && !unwrapKeys[m.key] {
			continue
		}
		if inner, ok := innerJSON(m.val); ok {
			members[i].val = inner
			changed = true
		}
	}
	if !changed {
		return nil
	}
	var out bytes.Buffer
	out.WriteByte('{')
	for i, m := range members {
		if i > 0 {
			out.WriteByte(',')
		}
		k, err := marshalKey(m.key)
		if err != nil {
			return nil
		}
		out.Write(k)
		out.WriteByte(':')
		var c bytes.Buffer
		if json.Compact(&c, m.val) != nil {
			return nil
		}
		out.Write(c.Bytes())
	}
	out.WriteByte('}')
	return out.Bytes()
}

func parseMembers(b []byte) ([]jsonMember, bool) {
	dec := json.NewDecoder(bytes.NewReader(b))
	if t, err := dec.Token(); err != nil || t != json.Delim('{') {
		return nil, false
	}
	var ms []jsonMember
	for dec.More() {
		kt, err := dec.Token()
		if err != nil {
			return nil, false
		}
		k, _ := kt.(string)
		var v json.RawMessage
		if dec.Decode(&v) != nil {
			return nil, false
		}
		ms = append(ms, jsonMember{key: k, val: v})
	}
	return ms, true
}

// innerJSON returns the raw inner document when v is a JSON string holding a
// JSON object or array.
func innerJSON(v json.RawMessage) (json.RawMessage, bool) {
	if len(v) == 0 || v[0] != '"' {
		return nil, false
	}
	var s string
	if json.Unmarshal(v, &s) != nil {
		return nil, false
	}
	in := bytes.TrimSpace([]byte(s))
	if len(in) == 0 || (in[0] != '{' && in[0] != '[') || !json.Valid(in) {
		return nil, false
	}
	return in, true
}

func marshalKey(k string) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(k); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}
