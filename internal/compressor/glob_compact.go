// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"strings"
)

const (
	globSingleDirCap  = 15
	globFewDirsCap    = 5
	globManyDirsCap   = 3
	globFlatMaxFiles  = 30
	globSummaryBuffer = 96
)

type pathDirGroup struct {
	dir   string
	paths []string
	bases []string
}

// CompactFilePaths groups file paths by parent directory, keeping representative
// samples per directory and counts of remaining files and directories. It
// returns false if the compacted output is not strictly smaller than raw.
func CompactFilePaths(paths []string, maxBytes int) ([]string, bool) {
	if len(paths) == 0 {
		return nil, false
	}
	rawBytes := 0
	for _, p := range paths {
		rawBytes += len(p) + 1
	}
	groups := groupPathsByDir(paths)
	perDir := globManyDirsCap
	switch {
	case len(groups) == 1:
		perDir = globSingleDirCap
	case len(groups) <= 10:
		perDir = globFewDirsCap
	}
	maxShown := 0
	if len(groups) > globFlatMaxFiles && len(groups) == len(paths) {
		maxShown = globFlatMaxFiles
	}
	budget := rawBytes * 3 / 4
	if maxBytes > 0 && maxBytes < budget {
		budget = maxBytes
	}

	var out []string
	used, shownFiles, omittedFiles, omittedDirs := 0, 0, 0, 0
	for _, g := range groups {
		if omittedDirs > 0 || (maxShown > 0 && shownFiles >= maxShown) {
			omittedDirs++
			omittedFiles += len(g.bases)
			continue
		}
		lines, cutInGroup := renderPathGroup(g, perDir, len(groups) > 1)
		need := sliceBytes(lines)
		if len(out) > 0 && budget > 0 && used+need+globSummaryBuffer > budget {
			omittedDirs++
			omittedFiles += len(g.bases)
			continue
		}
		out = append(out, lines...)
		used += need
		shownFiles += len(g.bases) - cutInGroup
		omittedFiles += cutInGroup
	}
	if omittedFiles > 0 {
		var summary string
		if omittedDirs > 0 {
			summary = fmt.Sprintf("… %d more files (downshift retrieve) across %d more directories", omittedFiles, omittedDirs)
		} else {
			summary = fmt.Sprintf("… %d more files (downshift retrieve)", omittedFiles)
		}
		out = append(out, summary)
		used += len(summary) + 1
	}
	if used >= rawBytes {
		return nil, false
	}
	return out, true
}

func groupPathsByDir(paths []string) []pathDirGroup {
	byDir := map[string]int{}
	var groups []pathDirGroup
	for _, p := range paths {
		dir, base := splitDirBase(p)
		idx, ok := byDir[dir]
		if !ok {
			idx = len(groups)
			byDir[dir] = idx
			groups = append(groups, pathDirGroup{dir: dir})
		}
		groups[idx].paths = append(groups[idx].paths, p)
		groups[idx].bases = append(groups[idx].bases, base)
	}
	return groups
}

func splitDirBase(p string) (dir, base string) {
	i := strings.LastIndexAny(p, "/\\")
	if i < 0 {
		return ".", p
	}
	if i == 0 {
		return p[:1], p[1:]
	}
	return p[:i+1], p[i+1:]
}

func renderPathGroup(g pathDirGroup, perDir int, multiDir bool) ([]string, int) {
	show := min(len(g.bases), perDir)
	cut := len(g.bases) - show
	if len(g.bases) == 1 || g.dir == "." {
		out := append([]string(nil), g.paths[:show]...)
		return out, cut
	}
	out := make([]string, 0, show+2)
	out = append(out, fmt.Sprintf("%s (%d files):", g.dir, len(g.bases)))
	for _, b := range g.bases[:show] {
		out = append(out, "  "+b)
	}
	if cut > 0 && multiDir {
		out = append(out, fmt.Sprintf("  … %d more", cut))
	}
	return out, cut
}

func sliceBytes(lines []string) int {
	n := 0
	for _, l := range lines {
		n += len(l) + 1
	}
	return n
}
