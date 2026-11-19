// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import "strings"

// ════════════════════════════════════════════════════════════════════════════
// DOMAIN: git ls-files — plain path list (read-only)
// ════════════════════════════════════════════════════════════════════════════

// FamilyGitLsFiles is `git ls-files` in plain path-list form, compacted as a
// path tree (compressor.FormatFind). Forms that alter row format (-s, -u, -t,
// -v, -z, --format) stay FamilyGitOther and pass through raw.
const FamilyGitLsFiles Family = "git-ls-files"

const gitLsFilesShort = "cdmoik"

var gitLsFilesLong = []string{
	"cached", "deleted", "modified", "others", "ignored", "killed",
	"exclude-standard", "full-name", "deduplicate", "directory", "no-empty-directory",
}

var gitLsFilesValueLong = []string{"exclude", "exclude-from", "exclude-per-directory"}

// gitLsFilesFamily names `git ls-files` (args are the words after
// "git ls-files") as FamilyGitLsFiles when its output is a plain path list,
// and FamilyGitOther otherwise.
func gitLsFilesFamily(args []string) Family {
	if gitLsFilesForm(args) {
		return FamilyGitLsFiles
	}
	return FamilyGitOther
}

func gitLsFilesForm(args []string) bool {
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--":
			return true
		case strings.HasPrefix(a, "--"):
			flag := a[2:]
			if name, _, hasEq := strings.Cut(flag, "="); hasEq {
				if !inList(gitLsFilesValueLong, name) {
					return false
				}
				continue
			}
			if inList(gitLsFilesValueLong, flag) {
				if i+1 >= len(args) {
					return false
				}
				i++
				continue
			}
			if !inList(gitLsFilesLong, flag) {
				return false
			}
		case strings.HasPrefix(a, "-") && a != "-":
			cluster := a[1:]
			for j := 0; j < len(cluster); j++ {
				c := cluster[j]
				switch {
				case strings.IndexByte(gitLsFilesShort, c) >= 0:
				case (c == 'x' || c == 'X') && j == len(cluster)-1 && i+1 < len(args):
					i++
				default:
					return false
				}
			}
		case a == "-":
			return false
		}
	}
	return true
}
