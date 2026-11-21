// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// decodePathPrefixes inverts FactorPathPrefixes (format in its doc comment).
func decodePathPrefixes(s string) string {
	lines := strings.Split(strings.TrimSuffix(s, "\n"), "\n")
	var out []string
	for i := 0; i < len(lines); i++ {
		if i+1 < len(lines) && strings.HasPrefix(lines[i+1], pathPrefixIndent) && !strings.HasPrefix(lines[i], pathPrefixIndent) {
			dir := lines[i]
			for i+1 < len(lines) && strings.HasPrefix(lines[i+1], pathPrefixIndent) {
				i++
				out = append(out, dir+strings.TrimPrefix(lines[i], pathPrefixIndent))
			}
			continue
		}
		out = append(out, lines[i])
	}
	res := strings.Join(out, "\n")
	if strings.HasSuffix(s, "\n") {
		res += "\n"
	}
	return res
}

func repeatPaths(dir string, names ...string) string {
	var b strings.Builder
	for _, n := range names {
		b.WriteString(dir + n + "\n")
	}
	return b.String()
}

func TestFactorPathPrefixes(t *testing.T) {
	telemetry := repeatPaths("internal/telemetry/", "a.go", "b.go", "c.go", "d_test.go", "store.go")
	tests := []struct {
		name   string
		in     string
		want   string // "" with ok=true means only check the round trip
		wantOK bool
	}{
		{"git ls-files", telemetry + repeatPaths("internal/core/", "route.go", "tier.go", "x.go", "y.go"), "", true},
		{"find output", repeatPaths("./src/pkg/", "one.ts", "two.ts", "three.ts", "four.ts"), "./src/pkg/\n  one.ts\n  two.ts\n  three.ts\n  four.ts\n", true},
		{"single dir", telemetry, "internal/telemetry/\n  a.go\n  b.go\n  c.go\n  d_test.go\n  store.go\n", true},
		{"mixed dirs keep order", telemetry + "README.md\n" + repeatPaths("docs/guides/", "install.md", "config.md", "ccr.md") + "go.mod\n", "", true},
		{"absolute paths", repeatPaths("/var/log/app/", "a.log", "b.log", "c.log"), "/var/log/app/\n  a.log\n  b.log\n  c.log\n", true},
		{"windows paths", repeatPaths(`internal\telemetry\`, "a.go", "b.go", "c.go", "d.go"), "internal\\telemetry\\\n  a.go\n  b.go\n  c.go\n  d.go\n", true},
		{"mixed separators", "src/win\\a.go\nsrc/win\\b.go\nsrc/win\\c.go\nsrc/win\\d.go\n", "src/win\\\n  a.go\n  b.go\n  c.go\n  d.go\n", true},
		{"paths with spaces", repeatPaths("my docs/sub dir/", "a b.txt", "c d.txt", "e f.txt"), "my docs/sub dir/\n  a b.txt\n  c d.txt\n  e f.txt\n", true},
		{"no trailing newline", strings.TrimSuffix(telemetry, "\n"), "internal/telemetry/\n  a.go\n  b.go\n  c.go\n  d_test.go\n  store.go", true},
		{"stray non-path line below 20%", telemetry + "warning: something odd\n", "", true},
		{"no match: unique dirs", "a/x.go\nb/y.go\nc/z.go\nd/w.go\n", "", false},
		{"no match: run of two", "internal/telemetry/a.go\ninternal/telemetry/b.go\n", "", false},
		{"no match: bare names", "a.go\nb.go\nc.go\nd.go\n", "", false},
		{"no match: prose", "Error: build failed\nsee the log for details\nexit 2\n", "", false},
		{"no match: ls -l rows", "-rw-r--r--  1 u g 120 Nov 20 10:00 a/b.go\n-rw-r--r--  1 u g 120 Nov 20 10:00 a/c.go\n-rw-r--r--  1 u g 120 Nov 20 10:00 a/d.go\n", "", false},
		{"no match: indented line", "  a/b.go\na/c.go\na/d.go\na/e.go\n", "", false},
		{"no match: crlf", "internal/telemetry/a.go\r\ninternal/telemetry/b.go\r\ninternal/telemetry/c.go\r\n", "", false},
		{"no match: empty", "", "", false},
		{"short parent saves nothing", "a/b\na/c\na/d\na/e\n", "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out, ok := FactorPathPrefixes([]byte(tc.in))
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v; out:\n%s", ok, tc.wantOK, out)
			}
			if !ok {
				if string(out) != tc.in {
					t.Fatalf("input changed on false:\n%s", out)
				}
				return
			}
			if tc.want != "" && string(out) != tc.want {
				t.Errorf("output:\n%s\nwant:\n%s", out, tc.want)
			}
			if len(out) >= len(tc.in) {
				t.Errorf("no gain: %d >= %d", len(out), len(tc.in))
			}
			if got := decodePathPrefixes(string(out)); got != tc.in {
				t.Errorf("round trip lost data:\n%s\nwant:\n%s", got, tc.in)
			}
		})
	}
}

func TestFactorPathPrefixesFixtureRatio(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "unclassified", "git_diff_name_only.txt"))
	if err != nil {
		t.Fatal(err)
	}
	out, ok := FactorPathPrefixes(data)
	if !ok {
		t.Fatal("fixture was not factored")
	}
	if got := decodePathPrefixes(string(out)); got != string(data) {
		t.Fatal("fixture round trip lost data")
	}
	t.Logf("git_diff_name_only.txt: %d -> %d bytes, ratio %.2f", len(data), len(out), float64(len(out))/float64(len(data)))
	if float64(len(out)) > 0.85*float64(len(data)) {
		t.Errorf("ratio %.2f is above 0.85", float64(len(out))/float64(len(data)))
	}
}
