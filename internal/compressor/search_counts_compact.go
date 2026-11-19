// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"strconv"
	"strings"
)

const countPerDirCap = 3

func compactSearchCounts(lines []string, rawBytes, maxBytes int) ([]byte, bool, bool) {
	type countEntry struct {
		file, line string
		count      int
	}
	var nonZero []countEntry
	zeroFiles := 0
	for _, l := range lines {
		m := reGrepCount.FindStringSubmatch(l)
		if m == nil {
			return nil, false, false
		}
		n, err := strconv.Atoi(m[2])
		if err != nil {
			return nil, false, false
		}
		if n == 0 {
			zeroFiles++
			continue
		}
		nonZero = append(nonZero, countEntry{file: m[1], line: l, count: n})
	}
	type dirBucket struct {
		dir        string
		files, sum int
		shown      []string
	}
	byDir := map[string]*dirBucket{}
	var order []string
	for _, e := range nonZero {
		dir, base := splitDirBase(e.file)
		b := byDir[dir]
		if b == nil {
			b = &dirBucket{dir: dir}
			byDir[dir] = b
			order = append(order, dir)
		}
		b.files++
		b.sum += e.count
		if len(b.shown) < countPerDirCap || len(nonZero) <= 20 {
			b.shown = append(b.shown, fmt.Sprintf("%s: %d", base, e.count))
		}
	}
	var out strings.Builder
	omittedFiles, omittedMatches := 0, 0
	for _, d := range order {
		b := byDir[d]
		if maxBytes > 0 && out.Len() > maxBytes {
			omittedFiles += b.files
			omittedMatches += b.sum
			continue
		}
		if b.files == 1 && d != "." {
			fmt.Fprintf(&out, "%s%s\n", d, b.shown[0])
			continue
		}
		if d != "." {
			fmt.Fprintf(&out, "%s (%d matches in %d files):\n", d, b.sum, b.files)
		}
		for _, s := range b.shown {
			if d == "." {
				out.WriteString(s + "\n")
			} else {
				out.WriteString("  " + s + "\n")
			}
		}
		if rem := b.files - len(b.shown); rem > 0 {
			fmt.Fprintf(&out, "  … %d more files\n", rem)
		}
	}
	if zeroFiles > 0 {
		fmt.Fprintf(&out, "… %d files with 0 matches folded\n", zeroFiles)
	}
	if omittedFiles > 0 {
		fmt.Fprintf(&out, "… %d more matches in %d files (downshift retrieve)\n", omittedMatches, omittedFiles)
	}
	res := []byte(out.String())
	if len(res) >= rawBytes {
		return nil, false, true
	}
	return res, true, true
}
