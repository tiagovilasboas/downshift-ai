// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

// expectedFormatLists mirrors every list in internal/execwrap/expected_format.go
// (execwrap imports compressor, so the lists are copied rather than imported).
var expectedFormatLists = [][]Format{
	{FormatGitDiff, FormatLockfileDiff},
	{FormatVitest, FormatJest},
	{FormatProgress, FormatNpmAudit, FormatNpmInstall},
	{FormatGhRunList, FormatGhRunLog},
	{FormatGhPr, FormatGhChecks},
	{FormatGrep, FormatGrepRows, FormatGrepCounts, FormatFind},
	{FormatGoTest}, {FormatGitStatus}, {FormatGitLog}, {FormatGitShow},
	{FormatPytest}, {FormatVitest}, {FormatPlaywright}, {FormatCargoTest},
	{FormatTSC}, {FormatJest}, {FormatCypress}, {FormatNextBuild},
	{FormatPHPUnit}, {FormatFlutterTest}, {FormatGradle}, {FormatMaven},
	{FormatEslint}, {FormatRuff}, {FormatMypy}, {FormatGolangciLint},
	{FormatGoBuild}, {FormatCargoBuild}, {FormatDotnetBuild}, {FormatPipInstall},
	{FormatBazel}, {FormatTurbo}, {FormatNx}, {FormatMarkdown}, {FormatRspec},
	{FormatRubocop}, {FormatDotnetTest}, {FormatSwiftTest}, {FormatPHPStan}, {FormatDockerBuild},
	{FormatDockerPs}, {FormatDockerImages}, {FormatKubectlPods}, {FormatGitStash},
	{FormatGoTestRuns}, {FormatGitRemote}, {FormatFileRead}, {FormatGlab},
	{FormatAgentDesktop}, {FormatHTTPFetch}, {FormatCTest}, {FormatGeneric},
}

// corpusInputs reads every fixture under testdata: inputs (.txt, .in) and
// goldens (.golden), which are realistic command output too.
func corpusInputs(t *testing.T) map[string][]byte {
	t.Helper()
	out := map[string][]byte{}
	err := filepath.WalkDir("testdata", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		switch filepath.Ext(path) {
		case ".txt", ".in", ".golden":
		default:
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		out[filepath.ToSlash(path)] = b
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) < 100 {
		t.Fatalf("corpus too small: %d fixtures", len(out))
	}
	return out
}

func formatIn(f Format, list []Format) bool {
	for _, w := range list {
		if w == f {
			return true
		}
	}
	return false
}

// TestDetectFormatExpected_CorpusParity holds the invariant the expected fast
// path must keep: when the full scan picks a format in the expected list, the
// fast path picks the same format and error flag; when the full scan picks
// anything else, the fast path never picks a format in the list (the caller
// would compact output the full scan keeps raw).
func TestDetectFormatExpected_CorpusParity(t *testing.T) {
	corpus := corpusInputs(t)
	type detected struct {
		format Format
		isErr  bool
	}
	full := make(map[string]detected, len(corpus))
	for name, in := range corpus {
		f, e := detectFormat(in)
		full[name] = detected{f, e}
	}
	for _, list := range expectedFormatLists {
		t.Run(fmt.Sprint(list), func(t *testing.T) {
			t.Parallel()
			for name, in := range corpus {
				want := full[name]
				gotF, gotErr := detectFormatExpected(in, list)
				if formatIn(want.format, list) {
					if gotF != want.format || gotErr != want.isErr {
						t.Errorf("%s: got (%s,%v), full scan (%s,%v)", name, gotF, gotErr, want.format, want.isErr)
					}
					continue
				}
				if formatIn(gotF, list) {
					t.Errorf("%s: expected path claims %s, full scan %s", name, gotF, want.format)
				}
			}
		})
	}
}

// TestCompressExitExpected_CorpusParity runs the public entry points over the
// corpus: whenever CompressExit's format is in the expected list, the expected
// path returns the identical result. Detection is covered list by list above;
// here each fixture runs the multi-format lists and the list it belongs to,
// which keeps the whole-pipeline pass affordable.
func TestCompressExitExpected_CorpusParity(t *testing.T) {
	corpus := corpusInputs(t)
	for name, in := range corpus {
		want := CompressExit(in, ModeSafe, 0)
		for _, list := range expectedFormatLists {
			if len(list) == 1 && list[0] != want.Format {
				continue
			}
			got := CompressExitExpected(in, ModeSafe, 0, list...)
			if formatIn(want.Format, list) {
				if got.Format != want.Format || got.Applied != want.Applied || got.Reason != want.Reason || string(got.Output) != string(want.Output) {
					t.Errorf("%s expected=%v: got %s/%v/%q, want %s/%v/%q", name, list, got.Format, got.Applied, got.Reason, want.Format, want.Applied, want.Reason)
				}
				continue
			}
			if got.Format != want.Format && formatIn(got.Format, list) {
				t.Errorf("%s expected=%v: expected path claims %s, full path %s", name, list, got.Format, want.Format)
			}
		}
	}
}
