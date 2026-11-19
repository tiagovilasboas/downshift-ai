// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"strings"
	"testing"
)

func TestCompactSearchMatches(t *testing.T) {
	t.Run("groups multi-match files losslessly when >=25% saved", func(t *testing.T) {
		var b strings.Builder
		for f := 0; f < 4; f++ {
			for m := 1; m <= 10; m++ {
				fmt.Fprintf(&b, "internal/very/long/package/path/service_handler_%d.go:%d:\t\treturn fmt.Errorf(\"err %d\")\n", f, m*5, m)
			}
		}
		raw := []byte(b.String())
		out, ok := CompactSearchMatches(raw, 12<<10)
		if !ok {
			t.Fatalf("expected lossless grouped compaction to apply")
		}
		if len(out)*4 > len(raw)*3 {
			t.Errorf("expected >=25%% reduction: raw=%d out=%d", len(raw), len(out))
		}
		text := string(out)
		if !strings.Contains(text, "internal/very/long/package/path/service_handler_0.go (10):\n") {
			t.Errorf("missing file header with count:\n%s", text)
		}
		if strings.Contains(text, "more matches") {
			t.Errorf("did not expect dropped matches when lossless saved >=25%%:\n%s", text)
		}
	})

	t.Run("groups single-match files by directory", func(t *testing.T) {
		var b strings.Builder
		for d := 0; d < 5; d++ {
			for f := 0; f < 8; f++ {
				fmt.Fprintf(&b, "internal/adapters/module_%02d/subpackage/file_%02d.go:%d:val := compute(%d)\n", d, f, f+1, f)
			}
		}
		raw := []byte(b.String())
		out, ok := CompactSearchMatches(raw, 12<<10)
		if !ok {
			t.Fatalf("expected directory-grouped compaction to apply")
		}
		if len(out) >= len(raw) {
			t.Errorf("output grew: %d >= %d", len(out), len(raw))
		}
		text := string(out)
		if !strings.Contains(text, "internal/adapters/module_00/subpackage/ (8 matches in 8 files):\n") {
			t.Errorf("missing directory header:\n%s", text)
		}
	})

	t.Run("applies noisy caps when lossless grouping cannot save 25%", func(t *testing.T) {
		var b strings.Builder
		for i := 0; i < 60; i++ {
			fmt.Fprintf(&b, "d%02d/f.go:%d:match payload number %d with extra context text\n", i, i+1, i)
		}
		raw := []byte(b.String())
		out, ok := CompactSearchMatches(raw, 12<<10)
		if !ok {
			t.Fatalf("expected noisy cap compaction to apply")
		}
		if len(out) >= len(raw) {
			t.Errorf("output grew: %d >= %d", len(out), len(raw))
		}
		if !strings.Contains(string(out), "… 30 more matches in 30 files") {
			t.Errorf("missing remaining match summary:\n%s", out)
		}
	})

	t.Run("caps large grep output at 20 per file and 250 total", func(t *testing.T) {
		var b strings.Builder
		for f := 0; f < 30; f++ {
			for m := 1; m <= 25; m++ {
				fmt.Fprintf(&b, "pkg/f%02d.go:%d:line %d\n", f, m, m)
			}
		}
		raw := []byte(b.String())
		out, ok := CompactSearchMatches(raw, 12<<10)
		if !ok || len(out) >= len(raw) {
			t.Fatalf("expected compaction: ok=%v len=%d/%d", ok, len(out), len(raw))
		}
		if !strings.Contains(string(out), "… 500 more matches in 30 files") {
			t.Errorf("unexpected summary:\n%s", out)
		}
	})

	t.Run("compacts grep count mode", func(t *testing.T) {
		var b strings.Builder
		for d := 0; d < 10; d++ {
			for f := 0; f < 6; f++ {
				cnt := f + 1
				if f == 0 {
					cnt = 0
				}
				fmt.Fprintf(&b, "internal/pkg_%02d/file_%02d.go:%d\n", d, f, cnt)
			}
		}
		raw := []byte(b.String())
		out, ok := CompactSearchMatches(raw, 12<<10)
		if !ok || len(out) >= len(raw) {
			t.Fatalf("expected count compaction: ok=%v len=%d/%d", ok, len(out), len(raw))
		}
		text := string(out)
		if !strings.Contains(text, "10 files with 0 matches folded") {
			t.Errorf("missing zero-count fold:\n%s", text)
		}
		if !strings.Contains(text, "internal/pkg_00/ (20 matches in 5 files):") {
			t.Errorf("missing directory bucket:\n%s", text)
		}
	})

	t.Run("never lengthens tiny or invalid input", func(t *testing.T) {
		for _, in := range [][]byte{
			nil,
			[]byte("a.go:1:x\n"),
			[]byte("bad\x00byte\n"),
			[]byte{0xff, 0xfe},
		} {
			out, ok := CompactSearchMatches(in, 12<<10)
			if ok || string(out) != string(in) {
				t.Errorf("expected passthrough for %q, got ok=%v out=%q", in, ok, out)
			}
		}
	})
}

func TestCompactFilePaths(t *testing.T) {
	t.Run("single directory keeps top samples and count", func(t *testing.T) {
		paths := make([]string, 50)
		for i := range paths {
			paths[i] = fmt.Sprintf("internal/compressor/handler_%02d.go", i)
		}
		out, ok := CompactFilePaths(paths, 6<<10)
		if !ok {
			t.Fatal("expected single-dir glob compaction")
		}
		joined := strings.Join(out, "\n")
		if !strings.Contains(joined, "internal/compressor/ (50 files):") {
			t.Errorf("missing dir header:\n%s", joined)
		}
		if !strings.Contains(joined, "… 35 more files (downshift retrieve)") {
			t.Errorf("missing remaining count:\n%s", joined)
		}
	})

	t.Run("wide multi-dir glob bounds to budget with dir count", func(t *testing.T) {
		paths := make([]string, 300)
		for i := range paths {
			paths[i] = fmt.Sprintf("internal/module_%03d/sub/component_%03d.go", i%60, i)
		}
		out, ok := CompactFilePaths(paths, 2<<10)
		if !ok {
			t.Fatal("expected wide glob compaction")
		}
		joined := strings.Join(out, "\n")
		if len(joined) > 2<<10 {
			t.Errorf("exceeded budget: %d > %d", len(joined), 2<<10)
		}
		if !strings.Contains(joined, "more files (downshift retrieve)") || !strings.Contains(joined, "more directories") {
			t.Errorf("missing file/dir remaining counts:\n%s", joined)
		}
	})

	t.Run("never lengthens short paths", func(t *testing.T) {
		paths := []string{"a.go", "b.go", "c.go"}
		if out, ok := CompactFilePaths(paths, 6<<10); ok || out != nil {
			t.Errorf("expected false for short path list, got %v", out)
		}
		if out, ok := CompactFilePaths(nil, 6<<10); ok || out != nil {
			t.Errorf("expected false for empty slice, got %v", out)
		}
	})
}

func TestCompressCommandOutput_JSONPruneWithoutTrailingNewline(t *testing.T) {
	raw := fmt.Sprintf(`{"id":123,"node_id":"NODE_XYZ","avatar_url":"https://example.com/a.png","html_url":"https://example.com/1","followers_url":"https://example.com/f","following_url":"https://example.com/g","subscriptions_url":"https://example.com/s","title":"keep me","body":%q}`, strings.Repeat("hello ", 80))
	if strings.HasSuffix(raw, "\n") {
		t.Fatal("fixture must not end with newline")
	}
	res := CompressCommandOutput([]byte(raw), FormatJSONPrune, 0)
	if !res.Applied || res.Format != FormatJSONPrune {
		t.Fatalf("expected FormatJSONPrune applied without trailing newline, got applied=%v reason=%q", res.Applied, res.Reason)
	}
	if res.ReducedBytes >= res.OriginalBytes {
		t.Errorf("expected shrink: %d >= %d", res.ReducedBytes, res.OriginalBytes)
	}
	if strings.Contains(string(res.Output), "node_id") || !strings.Contains(string(res.Output), "keep me") {
		t.Errorf("unexpected pruned output:\n%s", res.Output)
	}
}
