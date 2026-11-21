// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"encoding/json"
	"strings"
	"testing"
)

func escapedResult(t *testing.T, key, inner string) string {
	t.Helper()
	s, err := json.Marshal(inner)
	if err != nil {
		t.Fatal(err)
	}
	return `{"` + key + `":` + string(s) + `}`
}

func TestUnwrapJSONResult(t *testing.T) {
	pretty := "{\n  \"id\": 12345678901234567890123,\n  \"price\": 1.50,\n  \"name\": \"caf\\u00e9 <b>\",\n  \"z\": 1,\n  \"a\": [\n    1,\n    2\n  ]\n}"
	// The unicode escape must survive byte for byte (not become a literal é).
	prettyCompact := `{"id":12345678901234567890123,"price":1.50,"name":"caf` + "\\" + `u00e9 <b>","z":1,"a":[1,2]}`
	tests := []struct {
		name string
		in   string
		want string
		ok   bool
	}{
		{"escaped pretty JSON", escapedResult(t, "result", pretty), `{"result":` + prettyCompact + `}`, true},
		{"text key keeps name", escapedResult(t, "text", pretty), `{"text":` + prettyCompact + `}`, true},
		{"sole custom key", escapedResult(t, "payload", pretty), `{"payload":` + prettyCompact + `}`, true},
		{"inner array", escapedResult(t, "result", "[\n  1,\n  2,\n  3,\n  4,\n  5,\n  6\n]"), `{"result":[1,2,3,4,5,6]}`, true},
		{"sibling fields kept in order", `{"b":"x","result":` + mustQuote(t, pretty) + `,"a":1}`, `{"b":"x","result":` + prettyCompact + `,"a":1}`, true},
		{"plain pretty JSON", pretty, prettyCompact, true},
		{"already compact", prettyCompact, "", false},
		{"non-JSON text", strings.Repeat("hello world\n", 20), "", false},
		{"inner string not JSON", escapedResult(t, "result", strings.Repeat("plain words   ", 20)), "", false},
		{"inner invalid JSON", escapedResult(t, "result", "{\n  \"a\": \n"), "", false},
		{"non wrapper key with siblings untouched", `{"x":` + mustQuote(t, pretty) + `,"y":1}`, "", false},
		{"empty", "", "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := UnwrapJSONResult([]byte(tc.in))
			if ok != tc.ok {
				t.Fatalf("ok=%v want %v (out=%q)", ok, tc.ok, got)
			}
			if ok && string(got) != tc.want {
				t.Fatalf("got  %s\nwant %s", got, tc.want)
			}
			if ok && !json.Valid(got) {
				t.Fatalf("output not valid JSON: %s", got)
			}
		})
	}
}

func mustQuote(t *testing.T, s string) string {
	t.Helper()
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
