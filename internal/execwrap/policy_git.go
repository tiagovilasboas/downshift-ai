// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import "strings"

// gitSubcommands names the family of each git subcommand that has one. Every
// other subcommand is FamilyGitOther.
var gitSubcommands = map[string]Family{
	"status": FamilyGitStatus,
	"log":    FamilyGitLog,
	"diff":   FamilyGitDiff,
	"show":   FamilyGitShow,
	"branch": FamilyGitBranch,
	"fetch":  FamilyGitFetch,
	"pull":   FamilyGitPull,
	"push":   FamilyGitPush,
	"commit": FamilyGitCommit,
	"add":    FamilyGitAdd,
	"stash":  FamilyGitStash,
	"remote": FamilyGitRemote,
	"tag":    FamilyGitTag,
}

// gitFamilyOf names the family of a git command, given its words after
// "git". Leading `-C <dir>` directory changes (with a non-empty, non-flag
// safe <dir>) are allowed and repeatable; any other leading global option
// (-c, --config, --git-dir, --work-tree, --exec-path, --upload-pack, ...) is
// never wrapped so a config injection or repository override cannot hide
// behind the wrapper.
func gitFamilyOf(args []string) Family {
	for len(args) >= 2 && args[0] == "-C" {
		dir := args[1]
		if dir == "" || strings.HasPrefix(dir, "-") || !allSafe(dir) {
			return FamilyNone
		}
		args = args[2:]
	}
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return FamilyNone
	}
	if args[0] == "grep" {
		return gitGrepFamily(args[1:])
	}
	if args[0] == "config" {
		return gitConfigFamily(args[1:])
	}
	if args[0] == "ls-files" {
		return gitLsFilesFamily(args[1:])
	}
	if family, ok := gitSubcommands[args[0]]; ok {
		return family
	}
	return FamilyGitOther
}
