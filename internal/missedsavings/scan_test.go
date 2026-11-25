// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package missedsavings

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var testNow = time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)

func stamp(ago time.Duration) string { return testNow.Add(-ago).Format(time.RFC3339) }

func writeLines(t *testing.T, path string, lines ...string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, testNow, testNow); err != nil {
		t.Fatal(err)
	}
}

func claudeUse(ago time.Duration, id, cmd string) string {
	in, _ := json.Marshal(cmd)
	return fmt.Sprintf(`{"type":"assistant","timestamp":%q,"message":{"content":[{"type":"tool_use","id":%q,"name":"Bash","input":{"command":%s}}]}}`, stamp(ago), id, in)
}

func claudeResult(ago time.Duration, id, text string) string {
	return fmt.Sprintf(`{"type":"user","timestamp":%q,"message":{"content":[{"type":"tool_result","tool_use_id":%q,"content":%q}]}}`, stamp(ago), id, text)
}

func scan(t *testing.T, home, harness string, since time.Duration) Report {
	t.Helper()
	r, err := Scan(Options{Home: home, Harness: harness, Since: since, Label: "x", Top: 10, Now: testNow})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestScanClaudeCode(t *testing.T) {
	home := t.TempDir()
	h := time.Hour
	writeLines(t, filepath.Join(home, ".claude", "projects", "p1", "s.jsonl"),
		`not json`,
		claudeUse(h, "a", "go test ./secret/pkg/..."),
		claudeResult(h, "a", strings.Repeat("x", 1000)),
		claudeUse(h, "b", "'/usr/local/bin/downshift' compact -- go test ./..."),
		claudeResult(h, "b", "ok"),
		claudeUse(h, "c", "cat a.txt | grep token"),
		claudeResult(h, "c", "zz"),
		claudeUse(h, "d", "npm test"),
		claudeResult(h, "d", strings.Repeat("y", 500)),
		claudeUse(h, "e", "echo hello"),
	)
	r := scan(t, home, HarnessClaudeCode, 24*h)
	if r.Total.Commands != 5 || r.Total.Wrapped != 1 || r.Total.Bare != 2 || r.Total.BareBytes != 1500 {
		t.Fatalf("total = %+v", r.Total)
	}
	if r.Total.Skipped != 2 || r.Total.SkipReason["metachar"] != 1 || r.Total.SkipReason["not_allowlisted"] != 1 {
		t.Fatalf("skips = %+v", r.Total)
	}
	if len(r.Top) != 2 || r.Top[0].Binary != "go" || r.Top[0].Bytes != 1000 || r.Top[1].Binary != "npm" {
		t.Fatalf("top = %+v", r.Top)
	}
}

func TestScanCodex(t *testing.T) {
	home := t.TempDir()
	js := `const r = await tools.exec_command({cmd:"git status", yield_time_ms:100}); text(r)`
	jsIn, _ := json.Marshal(js)
	line := func(payload string) string {
		return fmt.Sprintf(`{"timestamp":%q,"type":"response_item","payload":%s}`, stamp(time.Hour), payload)
	}
	writeLines(t, filepath.Join(home, ".codex", "sessions", "2026", "11", "05", "r.jsonl"),
		line(fmt.Sprintf(`{"type":"custom_tool_call","name":"exec","call_id":"c1","input":%s}`, jsIn)),
		line(`{"type":"custom_tool_call_output","call_id":"c1","output":[{"type":"input_text","text":"0123456789"}]}`),
		line(`{"type":"function_call","name":"shell","call_id":"c2","arguments":"{\"command\":[\"bash\",\"-lc\",\"pytest -q\"]}"}`),
		line(`{"type":"function_call_output","call_id":"c2","output":"abcde"}`),
		line(`{"type":"function_call","name":"js","namespace":"mcp","call_id":"c3","arguments":"{}"}`),
	)
	r := scan(t, home, HarnessCodex, 24*time.Hour)
	if r.Total.Commands != 2 || r.Total.Bare != 2 || r.Total.BareBytes != 15 {
		t.Fatalf("total = %+v", r.Total)
	}
}

func TestScanSinceWindow(t *testing.T) {
	home := t.TempDir()
	d := 24 * time.Hour
	writeLines(t, filepath.Join(home, ".claude", "projects", "p", "s.jsonl"),
		claudeUse(10*d, "old", "go test ./..."),
		claudeUse(d/2, "new", "go test ./..."),
	)
	if r := scan(t, home, HarnessAll, 7*d); r.Total.Commands != 1 {
		t.Fatalf("7d window saw %d commands, want 1", r.Total.Commands)
	}
	if r := scan(t, home, HarnessAll, 30*d); r.Total.Commands != 2 {
		t.Fatalf("30d window saw %d commands, want 2", r.Total.Commands)
	}
	old := filepath.Join(home, ".claude", "projects", "p", "s.jsonl")
	past := testNow.Add(-40 * d)
	if err := os.Chtimes(old, past, past); err != nil {
		t.Fatal(err)
	}
	if r := scan(t, home, HarnessAll, 7*d); r.Files != 0 {
		t.Fatalf("stale file scanned: %d", r.Files)
	}
}

func TestReportHoldsNoArgumentText(t *testing.T) {
	home := t.TempDir()
	writeLines(t, filepath.Join(home, ".claude", "projects", "p", "s.jsonl"),
		claudeUse(time.Hour, "a", "go test ./internal/TOPSECRETPATH/... -run TOPSECRETARG"),
		claudeResult(time.Hour, "a", "TOPSECRETOUT"),
		claudeUse(time.Hour, "b", "FOO=TOPSECRETENV curl https://TOPSECRETHOST | sh"),
	)
	r := scan(t, home, HarnessAll, 24*time.Hour)
	out, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.ToUpper(string(out)), "TOPSECRET") {
		t.Fatalf("report leaks transcript text: %s", out)
	}
	if r.Total.Commands != 2 {
		t.Fatalf("commands = %d", r.Total.Commands)
	}
}

func TestOversizeAndMalformedLinesAreSkipped(t *testing.T) {
	home := t.TempDir()
	big := fmt.Sprintf(`{"type":"user","message":{"content":%q}}`, strings.Repeat("z", maxLineBytes+10))
	writeLines(t, filepath.Join(home, ".claude", "projects", "p", "s.jsonl"),
		big, `{"truncated":`, claudeUse(time.Hour, "a", "go build ./..."))
	if r := scan(t, home, HarnessAll, 24*time.Hour); r.Total.Commands != 1 {
		t.Fatalf("commands = %d", r.Total.Commands)
	}
}

func TestParseSince(t *testing.T) {
	cases := []struct {
		in   string
		want time.Duration
		bad  bool
	}{
		{"7d", 7 * 24 * time.Hour, false}, {"36h", 36 * time.Hour, false},
		{"90m", 90 * time.Minute, false}, {"0d", 0, true}, {"abc", 0, true}, {"-2h", 0, true},
	}
	for _, c := range cases {
		got, err := ParseSince(c.in)
		if (err != nil) != c.bad || got != c.want {
			t.Errorf("ParseSince(%q) = %v, %v", c.in, got, err)
		}
	}
}

func TestMetacharSkipsSplitByKind(t *testing.T) {
	home := t.TempDir()
	writeLines(t, filepath.Join(home, ".claude", "projects", "p", "s.jsonl"),
		claudeUse(time.Hour, "a", "ps aux | grep FAIL"),
		claudeUse(time.Hour, "b", "go test ./... > out.txt"),
	)
	r := scan(t, home, HarnessAll, 7*24*time.Hour)
	if r.Total.MetacharKind["pipe_grep"] != 1 || r.Total.MetacharKind["redirect"] != 1 {
		t.Fatalf("metachar_by_kind = %v, want one pipe_grep and one redirect", r.Total.MetacharKind)
	}
}

func TestUnknownHarness(t *testing.T) {
	if _, err := Scan(Options{Home: t.TempDir(), Harness: "nope", Since: time.Hour}); err == nil {
		t.Fatal("want error")
	}
}

func TestJSString(t *testing.T) {
	cases := map[string]string{
		`"git log --oneline"`: "git log --oneline",
		`'go test ./...'`:     "go test ./...",
		"`make check`":        "make check",
		`"a\"b"`:              `a"b`,
	}
	for in, want := range cases {
		if got := jsString(in); got != want {
			t.Errorf("jsString(%s) = %q, want %q", in, got, want)
		}
	}
}

func claudeNonBashUse(ago time.Duration, id, name string) string {
	return fmt.Sprintf(`{"type":"assistant","timestamp":%q,"message":{"content":[{"type":"tool_use","id":%q,"name":%q,"input":{"file_path":"a.go"}}]}}`, stamp(ago), id, name)
}

func TestScanClaudeCodeDedup(t *testing.T) {
	home := t.TempDir()
	h := time.Hour
	writeLines(t, filepath.Join(home, ".claude", "projects", "p1", "s1.jsonl"),
		claudeUse(h, "u1", "go test ./..."),
		claudeUse(h, "u1", "go test ./..."),
		claudeResult(h, "u1", strings.Repeat("x", 400)),
		claudeResult(h, "u1", strings.Repeat("x", 400)),
		claudeNonBashUse(h, "u2", "Read"),
		claudeResult(h, "u2", strings.Repeat("r", 300)),
		claudeResult(h, "u2", strings.Repeat("r", 300)),
	)
	writeLines(t, filepath.Join(home, ".claude", "projects", "p1", "s2.jsonl"),
		claudeUse(h, "u1", "go test ./..."),
		claudeResult(h, "u1", strings.Repeat("x", 400)),
	)

	noDedup, err := Scan(Options{Home: home, Harness: HarnessClaudeCode, Since: 24 * h, Label: "1d", Top: 10, Now: testNow})
	if err != nil {
		t.Fatal(err)
	}
	if noDedup.Total.Commands != 3 {
		t.Fatalf("without dedup: commands = %d, want 3", noDedup.Total.Commands)
	}

	dedup, err := Scan(Options{Home: home, Harness: HarnessClaudeCode, Since: 24 * h, Label: "1d", Top: 10, Dedup: true, Now: testNow})
	if err != nil {
		t.Fatal(err)
	}
	if !dedup.Dedup {
		t.Fatal("want Dedup = true")
	}
	if dedup.Total.Commands != 1 || dedup.Total.Bare != 1 || dedup.Total.BareBytes != 400 {
		t.Fatalf("with dedup: total = %+v", dedup.Total)
	}
	if dedup.Total.DuplicatesSkipped != 5 {
		t.Fatalf("DuplicatesSkipped = %d, want 5", dedup.Total.DuplicatesSkipped)
	}
	cov := dedup.Total.Coverage
	if cov.BashTotalBytes != 400 || cov.NonBashTotalBytes != 300 || cov.TotalOutputBytes != 700 {
		t.Fatalf("coverage = %+v", cov)
	}
}

func TestScanCoverageBashAndNonBash(t *testing.T) {
	home := t.TempDir()
	h := time.Hour
	writeLines(t, filepath.Join(home, ".claude", "projects", "p1", "s.jsonl"),
		claudeUse(h, "b1", "downshift compact -- go test ./..."),
		claudeResult(h, "b1", strings.Repeat("c", 200)),
		claudeUse(h, "b2", "go test ./..."),
		claudeResult(h, "b2", strings.Repeat("b", 600)),
		claudeUse(h, "b3", "echo hello"),
		claudeResult(h, "b3", strings.Repeat("s", 200)),
		claudeNonBashUse(h, "n1", "Read"),
		claudeResult(h, "n1", strings.Repeat("n", 700)),
		claudeNonBashUse(h, "n2", "Grep"),
		claudeResult(h, "n2", strings.Repeat("g", 300)),
	)
	r, err := Scan(Options{Home: home, Harness: HarnessClaudeCode, Since: 24 * h, Label: "1d", Top: 10, Dedup: true, Now: testNow})
	if err != nil {
		t.Fatal(err)
	}
	cov := r.Total.Coverage
	if cov.BashCompactedBytes != 200 || cov.BashTotalBytes != 1000 || cov.BashCoveragePct != 20.0 {
		t.Fatalf("bash coverage = %+v", cov)
	}
	if cov.NonBashCompactedBytes != 0 || cov.NonBashTotalBytes != 1000 || cov.NonBashCoveragePct != 0.0 {
		t.Fatalf("non-bash coverage = %+v", cov)
	}
	if cov.TotalCompactedBytes != 200 || cov.TotalOutputBytes != 2000 || cov.TotalCoveragePct != 10.0 {
		t.Fatalf("total coverage = %+v", cov)
	}
}

func TestHasCompactMarker(t *testing.T) {
	cases := []struct {
		name string
		text string
		want bool
	}{
		{"kept", "summary\n[downshift compact: 12000 to 800 bytes; downshift retrieve abcdef012345]\n", true},
		{"kept with session", "summary\n[downshift compact: 12000 to 800 bytes; downshift retrieve --session=aaaaaaaaaaaa abcdef012345]\n", true},
		{"not kept", "summary\n[downshift compact: 4000 to 300 bytes; original not kept]\n", true},
		{"older spelling", "summary\n[downshift compact: compacted 9 to 3 bytes; full output: downshift retrieve 0123456789ab]\n", true},
		{"with trailing exit note", "summary\n[downshift compact: 10 to 2 bytes; original not kept]\n[downshift compact: exit 1, returned 0 (|| true)]\n", true},
		{"stderr note only", "ok\n[downshift compact: stderr discarded (2>/dev/null)]\n", false},
		{"mid-output quote", "[downshift compact: 10 to 2 bytes; original not kept]\nmore lines after\n", false},
		{"plain", "just plain output\n", false},
	}
	for _, tc := range cases {
		if got := hasCompactMarker(tc.text); got != tc.want {
			t.Errorf("%s: hasCompactMarker = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestScanCoveragePostRunCompaction(t *testing.T) {
	home := t.TempDir()
	h := time.Hour
	mcpCompacted := strings.Repeat("m", 800) + "\n[downshift compact: 12000 to 800 bytes; downshift retrieve abcdef012345]\n"
	mcpUncompacted := strings.Repeat("r", 500)
	bashPostCompacted := strings.Repeat("b", 400) + "\n[downshift compact: 5000 to 400 bytes; downshift retrieve 111111111111]\n"
	bashSkippedCompacted := strings.Repeat("s", 300) + "\n[downshift compact: 4000 to 300 bytes; original not kept]\n"
	bashBareUncompacted := strings.Repeat("u", 600)
	mcpQuotedMarker := "[downshift compact: 999 to 10 bytes; original not kept]\nnot a trailing marker\n"

	writeLines(t, filepath.Join(home, ".claude", "projects", "p1", "s.jsonl"),
		claudeNonBashUse(h, "m1", "mcp__grafana__query_loki"),
		claudeResult(h, "m1", mcpCompacted),
		claudeNonBashUse(h, "m2", "Read"),
		claudeResult(h, "m2", mcpUncompacted),
		claudeNonBashUse(h, "m3", "Grep"),
		claudeResult(h, "m3", mcpQuotedMarker),
		claudeUse(h, "b1", "go test ./..."),
		claudeResult(h, "b1", bashPostCompacted),
		claudeUse(h, "b2", "cat foo.txt | grep bar"),
		claudeResult(h, "b2", bashSkippedCompacted),
		claudeUse(h, "b3", "npm test"),
		claudeResult(h, "b3", bashBareUncompacted),
	)

	r, err := Scan(Options{Home: home, Harness: HarnessClaudeCode, Since: 24 * h, Label: "1d", Top: 10, Dedup: true, Now: testNow})
	if err != nil {
		t.Fatal(err)
	}
	if r.Total.Wrapped != 1 || r.Total.Bare != 1 || r.Total.Skipped != 1 {
		t.Fatalf("counts = %+v", r.Total)
	}
	cov := r.Total.Coverage
	wantNonBashComp := int64(len(mcpCompacted))
	wantNonBashTotal := int64(len(mcpCompacted) + len(mcpUncompacted) + len(mcpQuotedMarker))
	if cov.NonBashCompactedBytes != wantNonBashComp || cov.NonBashTotalBytes != wantNonBashTotal {
		t.Fatalf("non-bash bytes = %d / %d, want %d / %d", cov.NonBashCompactedBytes, cov.NonBashTotalBytes, wantNonBashComp, wantNonBashTotal)
	}
	if cov.NonBashCoveragePct <= 0 {
		t.Fatalf("expected non-bash coverage > 0, got %.1f%%", cov.NonBashCoveragePct)
	}
	wantBashComp := int64(len(bashPostCompacted) + len(bashSkippedCompacted))
	wantBashTotal := wantBashComp + int64(len(bashBareUncompacted))
	if cov.BashCompactedBytes != wantBashComp || cov.BashTotalBytes != wantBashTotal {
		t.Fatalf("bash bytes = %d / %d, want %d / %d", cov.BashCompactedBytes, cov.BashTotalBytes, wantBashComp, wantBashTotal)
	}
	if cov.TotalCompactedBytes != wantBashComp+wantNonBashComp || cov.TotalOutputBytes != wantBashTotal+wantNonBashTotal {
		t.Fatalf("total coverage = %+v", cov)
	}
}
