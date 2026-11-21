// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/compressor"
	"github.com/tiagovilasboas/downshift-ai/internal/execwrap"
)

// postRunThreshold mirrors postrun.PostRunBalanceThreshold (4 KiB); postrun
// is not imported to keep this baseline free of the hook layer.
const postRunThreshold = 4 << 10

// unclassifiedBaseline is the pipeline of an unclassified command's output
// after the run: execwrap.CompactCapturedWithThreshold with FamilyNone, CCR
// storing (so the balanced cut may apply) and the post-run threshold. It is
// the baseline that the lossless pass (C4) and the byte-budget cut (C5) must
// beat. It returns the text the agent would read.
func unclassifiedBaseline(data []byte, exit int) []byte {
	keep := func([]byte) (string, bool) { return "baseline0001", true }
	res := execwrap.CompactCapturedWithThreshold(nil, execwrap.FamilyNone, data, exit, keep, true, postRunThreshold, "")
	return res.Output
}

// unclassifiedWant is the recorded baseline: the bytes the agent reads today
// per fixture. A later change that compacts better lowers a number here, so
// its PR shows the delta; a worse result must not pass. Update with
// DOWNSHIFT_UPDATE_BASELINE=1 and read the logged table.
var unclassifiedWant = map[string]struct{ raw, out int }{
	"ansi_build_progress.txt":    {4319, 123},
	"base64_blob.txt":            {5674, 3552},
	"curl_jq_json.txt":           {9057, 5700},
	"docker_ps_table.txt":        {6103, 3516},
	"error_traceback_middle.txt": {5244, 3857},
	"find_xargs_listing.txt":     {6480, 3639},
	"grep_pipe_matches.txt":      {11376, 3637},
	"grep_pipe_small.txt":        {3803, 2864},
	"git_show_piped.txt":         {14932, 8077},
	"python_script_log.txt":      {24498, 5858},
	"short_output.txt":           {134, 134},
}

var reLineSep = regexp.MustCompile(`[\r\n]+`)

func readUnclassified(t testing.TB, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "unclassified", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestUnclassifiedBaseline(t *testing.T) {
	update := os.Getenv("DOWNSHIFT_UPDATE_BASELINE") == "1"
	for name, want := range unclassifiedWant {
		t.Run(name, func(t *testing.T) {
			data := readUnclassified(t, name)
			out := unclassifiedBaseline(data, 0)
			ratio := float64(len(out)) / float64(len(data))
			t.Logf("%-28s raw=%6d out=%6d ratio=%.2f lines=%d", name, len(data), len(out), ratio, strings.Count(string(data), "\n"))
			if update {
				return
			}
			if len(data) != want.raw {
				t.Fatalf("fixture size = %d, want %d (fixture changed: re-record the baseline)", len(data), want.raw)
			}
			if len(out) > want.out {
				t.Errorf("output grew past the baseline: %d > %d bytes", len(out), want.out)
			}
			if len(out) < want.out {
				t.Errorf("output beats the baseline (%d < %d bytes): record the new number", len(out), want.out)
			}
		})
	}
}

// TestUnclassifiedKeepsDangerLines is the lossless invariant: every line the
// balanced cut keeps for its words (error, fail, warn, panic, ...) is still in
// what the agent reads.
func TestUnclassifiedKeepsDangerLines(t *testing.T) {
	for name := range unclassifiedWant {
		t.Run(name, func(t *testing.T) {
			data := readUnclassified(t, name)
			out := string(unclassifiedBaseline(data, 0))
			for _, l := range reLineSep.Split(string(data), -1) {
				if compressor.IsKeepLine(l) && !strings.Contains(out, l) {
					t.Errorf("danger line lost: %q", l)
				}
			}
		})
	}
}

// TestDiffShapedOnFixture reports what DiffShaped alone would read for the
// piped git show fixture, and checks it keeps every +/- line.
func TestDiffShapedOnFixture(t *testing.T) {
	data := readUnclassified(t, "git_show_piped.txt")
	out, ok := compressor.DiffShaped(data)
	if !ok {
		t.Fatal("DiffShaped did not compress the fixture")
	}
	t.Logf("git_show_piped.txt raw=%d diffshaped=%d ratio=%.2f", len(data), len(out), float64(len(out))/float64(len(data)))
	kept := map[string]int{}
	for _, l := range strings.Split(string(out), "\n") {
		kept[l]++
	}
	for _, l := range strings.Split(string(data), "\n") {
		if (strings.HasPrefix(l, "+") || strings.HasPrefix(l, "-")) && kept[l] == 0 {
			t.Fatalf("change line lost: %q", l)
		}
	}
}

func BenchmarkUnclassifiedBaseline(b *testing.B) {
	for name := range unclassifiedWant {
		data := readUnclassified(b, name)
		b.Run(name, func(b *testing.B) {
			b.SetBytes(int64(len(data)))
			for i := 0; i < b.N; i++ {
				unclassifiedBaseline(data, 0)
			}
		})
	}
}
