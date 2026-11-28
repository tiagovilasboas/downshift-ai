// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import "strings"

// ReadOnly reports whether command is one read-only command a harness may
// approve on Downshift's behalf: the rewrite answer of some harnesses (Codex)
// also approves the command, so the user is never asked. It is true only for
// a single command with no shell metacharacter (no list, pipe, background,
// substitution, redirection, quote or newline), no leading NAME=value
// assignment, and an argv in the read-only set below. Every runner, build,
// test and install family executes project code and is never read-only;
// neither is a subcommand or flag that writes local or remote state.
func ReadOnly(command string) bool {
	trimmed := strings.Trim(command, " ")
	if trimmed == "" || len(trimmed) > maxCommandBytes || !allSafe(trimmed) {
		return false
	}
	assigns, argv := splitEnvAssignments(strings.Fields(trimmed))
	if len(assigns) > 0 || len(argv) == 0 {
		return false
	}
	norm, _ := normalizeBinaryPath(argv)
	if _, isExec := containerExecWords(norm); isExec {
		return false
	}
	check, ok := readOnlyFamilies[FamilyOf(norm)]
	return ok && check(norm)
}

// readOnlyFamilies maps each family that has a read-only form to the check
// its argv must pass. A family absent from the map is never read-only.
var readOnlyFamilies = map[Family]func(argv []string) bool{
	FamilyGitStatus: gitReadArgs,
	FamilyGitLog:    gitReadArgs,
	FamilyGitDiff:   gitReadArgs,
	FamilyGitShow:   gitReadArgs,
	FamilyGitBranch: func(argv []string) bool {
		_, a, ok := gitSubcommand(argv)
		return ok && onlyWords(a, gitBranchListFlags)
	},
	FamilyGitFetch: gitFetchReadArgs,
	FamilyGitRemote: func(argv []string) bool {
		_, a, ok := gitSubcommand(argv)
		return ok && (onlyWords(a, gitRemoteListFlags) || (len(a) == 2 && a[0] == "get-url" && !strings.HasPrefix(a[1], "-")))
	},
	FamilyGitStash: func(argv []string) bool {
		_, a, ok := gitSubcommand(argv)
		return ok && len(a) >= 1 && (a[0] == "list" || a[0] == "show") && !namesFlag(a[1:], gitWriteFlags...)
	},
	FamilyGitTag:     gitTagReadArgs,
	FamilyGitConfig:  gitConfigReadArgs,
	FamilyGitGrep:    gitGrepReadArgs,
	FamilyGitLsFiles: gitReadArgs,
	FamilyGitOther: func(argv []string) bool {
		sub, a, ok := gitSubcommand(argv)
		return ok && ((sub == "grep" && gitGrepReadArgs(argv)) || (gitReadOther[sub] && !namesFlag(a, gitWriteFlags...)))
	},
	FamilyGhPr:      ghReadArgs,
	FamilyGhIssue:   ghReadArgs,
	FamilyGhRun:     ghReadArgs,
	FamilyGhRepo:    ghReadArgs,
	FamilyGhSearch:  func(argv []string) bool { return len(argv) >= 2 && !namesFlag(argv[2:], ghBrowserFlags...) },
	FamilyGlab:      func([]string) bool { return true },
	FamilyGoVersion: func([]string) bool { return true },
	// FamilyGoDoc also holds `go tool`, which builds and runs programs: only
	// `go doc` is read-only, and never with -http (it runs pkgsite).
	FamilyGoDoc:        func(argv []string) bool { return len(argv) >= 2 && argv[1] == "doc" && !namesFlag(argv[2:], "http") },
	FamilyGoEnv:        func(argv []string) bool { return len(argv) >= 2 && !namesFlag(argv[2:], "w", "u") },
	FamilyGoList:       func(argv []string) bool { return len(argv) >= 2 && !namesFlag(argv[2:], goListWriteFlags...) },
	FamilyMise:         func(argv []string) bool { return len(argv) >= 2 && versionListSubcommands[argv[1]] },
	FamilyAsdf:         func(argv []string) bool { return len(argv) >= 2 && versionListSubcommands[argv[1]] },
	FamilyDirenv:       func(argv []string) bool { return len(argv) >= 2 && argv[1] == "status" },
	FamilyDockerPS:     func([]string) bool { return true },
	FamilyDockerImages: func([]string) bool { return true },
	FamilyDockerLogs:   func(argv []string) bool { return !keepsRunning(FamilyDockerLogs, argv) },
	FamilyLs:           func([]string) bool { return true },
	FamilyFind: func(argv []string) bool {
		return findPlainForm(argv[1:]) && !namesFlag(argv[1:], findUnsafePrimaries...)
	},
	FamilyGrep:         func([]string) bool { return true },
	FamilyRg:           func([]string) bool { return true },
	FamilyRgFiles:      func([]string) bool { return true },
	FamilyFileRead:     func(argv []string) bool { return readFamily(argv) == FamilyFileRead },
	FamilyWc:           func(argv []string) bool { return wcListArgs(argv[1:]) },
	FamilyAgentDesktop: func([]string) bool { return true },
}

var (
	// gitReadOther names read-only git subcommands without their own family.
	gitReadOther = map[string]bool{
		"rev-parse": true, "ls-files": true, "describe": true, "rev-list": true,
		"merge-base": true, "ls-tree": true, "show-ref": true, "for-each-ref": true,
		"diff-tree": true, "diff-index": true, "diff-files": true, "count-objects": true,
	}
	// gitWriteFlags write a file or run a configured program from a read subcommand.
	gitWriteFlags = []string{"output", "ext-diff", "textconv"}
	// gitFetchUnsafeFlags run a helper binary, force/prune refs or update HEAD.
	gitFetchUnsafeFlags = []string{
		"c", "config", "upload-pack", "exec", "receive-pack",
		"f", "force", "p", "prune", "P", "prune-tags", "u", "update-head-ok", "refmap",
	}
	// findUnsafePrimaries execute commands or write files (defense in depth).
	findUnsafePrimaries = []string{"exec", "execdir", "ok", "okdir", "delete", "fprint", "fprint0", "fprintf", "fls"}
	// gitBranchListFlags and gitRemoteListFlags only list (no positional names).
	gitBranchListFlags = map[string]bool{
		"-a": true, "--all": true, "-r": true, "--remotes": true,
		"-v": true, "-vv": true, "--verbose": true, "--list": true, "--show-current": true,
		"-av": true, "-va": true, "-avv": true, "-rv": true, "-vr": true,
		"--merged": true, "--no-merged": true, "--no-color": true,
	}
	gitRemoteListFlags = map[string]bool{"-v": true, "--verbose": true}
	// gitTagFilterFlags are list-mode filter options taking an optional commit/object.
	gitTagFilterFlags = map[string]bool{
		"--contains": true, "--no-contains": true, "--merged": true, "--no-merged": true, "--points-at": true,
	}
	// gitConfigSafeFlags are read-only modifiers (mutating/edit/file flags excluded).
	gitConfigSafeFlags = map[string]bool{
		"--local": true, "--global": true, "--system": true, "--worktree": true,
		"--show-origin": true, "--show-scope": true, "--name-only": true,
		"--includes": true, "--no-includes": true, "--null": true, "-z": true,
		"--bool": true, "--int": true, "--bool-or-int": true, "--path": true, "--expiry-date": true,
	}
	// ghReadSubcommands are the gh subcommands, per group, that only read.
	ghReadSubcommands = map[string]map[string]bool{
		"pr":       {"view": true, "list": true, "status": true, "diff": true, "checks": true},
		"issue":    {"view": true, "list": true, "status": true},
		"run":      {"view": true, "list": true},
		"workflow": {"view": true, "list": true},
		"repo":     {"view": true, "list": true},
		"release":  {"view": true, "list": true},
		"gist":     {"view": true, "list": true},
	}
	// ghBrowserFlags open a browser instead of printing (gh pr view --web).
	ghBrowserFlags = []string{"web", "w"}
	// goListWriteFlags make go list run a program or edit go.mod (-mod=mod).
	goListWriteFlags = []string{"toolexec", "exec", "export", "compiled", "mod"}
	// versionListSubcommands are the mise/rtx/asdf subcommands that only list.
	versionListSubcommands = map[string]bool{"list": true, "ls": true, "current": true}
)

// gitSubcommand strips leading `-C <dir>` pairs after "git" (matching
// gitFamilyOf) and returns the subcommand and its arguments.
func gitSubcommand(argv []string) (string, []string, bool) {
	if len(argv) < 2 || argv[0] != "git" {
		return "", nil, false
	}
	rest := argv[1:]
	for len(rest) >= 2 && rest[0] == "-C" {
		dir := rest[1]
		if dir == "" || strings.HasPrefix(dir, "-") || !allSafe(dir) {
			return "", nil, false
		}
		rest = rest[2:]
	}
	if len(rest) == 0 || strings.HasPrefix(rest[0], "-") {
		return "", nil, false
	}
	return rest[0], rest[1:], true
}

func gitReadArgs(argv []string) bool {
	_, args, ok := gitSubcommand(argv)
	return ok && !namesFlag(args, gitWriteFlags...)
}

func gitFetchReadArgs(argv []string) bool {
	_, args, ok := gitSubcommand(argv)
	if !ok || namesFlag(args, gitWriteFlags...) || namesFlag(args, gitFetchUnsafeFlags...) {
		return false
	}
	for _, a := range args {
		if strings.HasPrefix(a, "-") && !strings.HasPrefix(a, "--") {
			if strings.ContainsAny(a[1:], "cfpPu") {
				return false
			}
			continue
		}
		if strings.HasPrefix(a, "ext::") || strings.HasPrefix(a, "fd::") {
			return false
		}
		if !strings.HasPrefix(a, "-") && strings.Contains(a, ":") &&
			!strings.Contains(a, "://") && !strings.Contains(a, "@") {
			return false
		}
	}
	return true
}

func gitTagReadArgs(argv []string) bool {
	_, args, ok := gitSubcommand(argv)
	if !ok {
		return false
	}
	if len(args) == 0 {
		return true
	}
	explicitList, hasListFlag := false, false
	for i := 0; i < len(args); i++ {
		a := args[i]
		name, _, hasEq := strings.Cut(a, "=")
		switch {
		case a == "-l" || a == "--list":
			explicitList, hasListFlag = true, true
		case a == "-n" || (strings.HasPrefix(a, "-n") && strings.TrimLeft(a[2:], "0123456789") == ""):
			hasListFlag = true
		case a == "--sort" && i+1 < len(args) && !strings.HasPrefix(args[i+1], "--"):
			hasListFlag = true
			i++
		case strings.HasPrefix(a, "--sort="):
			hasListFlag = true
		case gitTagFilterFlags[name]:
			hasListFlag = true
			if !hasEq && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				i++
			}
		case strings.HasPrefix(a, "-") || !explicitList:
			return false
		}
	}
	return hasListFlag
}

func gitConfigReadArgs(argv []string) bool {
	sub, args, ok := gitSubcommand(argv)
	if !ok || sub != "config" {
		return false
	}
	hasRead := false
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case gitConfigReadFlags[a]:
			hasRead = true
		case gitConfigSafeFlags[a]:
		case (a == "--type" || a == "--default") && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-"):
			i++
		case strings.HasPrefix(a, "--type=") || strings.HasPrefix(a, "--default="):
		case strings.HasPrefix(a, "-"):
			return false
		}
	}
	return hasRead
}

func gitGrepReadArgs(argv []string) bool {
	sub, args, ok := gitSubcommand(argv)
	if !ok || sub != "grep" || namesFlag(args, gitWriteFlags...) || namesFlag(args, "O", "open-files-in-pager") {
		return false
	}
	for _, a := range args {
		if a == "--" {
			break
		}
		if strings.HasPrefix(a, "-") && !strings.HasPrefix(a, "--") && strings.ContainsRune(a[1:], 'O') {
			return false
		}
	}
	return true
}

func ghReadArgs(argv []string) bool {
	return len(argv) >= 3 && ghReadSubcommands[argv[1]][argv[2]] && !namesFlag(argv[3:], ghBrowserFlags...)
}

// onlyWords reports whether every word is one of the allowed flags.
func onlyWords(words []string, allowed map[string]bool) bool {
	for _, w := range words {
		if !allowed[w] {
			return false
		}
	}
	return true
}

// namesFlag reports whether a word is one of the named flags, written with one
// or two dashes, alone or followed by "=value".
func namesFlag(words []string, names ...string) bool {
	for _, w := range words {
		if !strings.HasPrefix(w, "-") {
			continue
		}
		bare := strings.TrimLeft(w, "-")
		for _, n := range names {
			if bare == n || strings.HasPrefix(bare, n+"=") {
				return true
			}
		}
	}
	return false
}
