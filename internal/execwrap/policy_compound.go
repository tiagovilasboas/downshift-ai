// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import "strings"

// Compound shapes: the few shell forms agents wrap around an allowlisted
// command most often. Each is matched exactly, word by word; anything else
// keeps the plain policy's answer (unsafe_character).
//
//	cd <dir> && <allowlisted>
//	<allowlisted> | tail -N        (also head, -n N, -nN)
//	<allowlisted> | grep <pat>     (also rg/egrep/fgrep, sort/uniq/wc/cut stdin filters)
//	<allowlisted> 2>&1 | tail -N
//	<allowlisted> 2>&1 | grep <pat>
//	cd <dir> && <allowlisted> 2>&1 | tail -N
//	<allowlisted> 2>&1             (a stderr merge, no pipe)
//	cd <dir> && <allowlisted> 2>&1
//
// The 2>&1 merge is kept as written: the wrapper already captures both
// streams of the command, so the rewritten command keeps the same merge on
// the wrapper's own output.
//
// Only the allowlisted command is wrapped; the cd prefix and the pipe suffix
// are kept as written (or single-quoted per word when a grep/rg pattern holds
// quotes), so the exit code is what it was: cd's on failure, the pipe's last
// command otherwise. <dir> and unquoted pipe stages pass the safe-byte gate,
// so they hold no expansion, quote, glob or tilde.
//
// The wrapped command may also hold quoted words and a single 2>/dev/null
// (policy_quoted.go, policy_redirect.go), as in
// `go test -run 'A|B' ./... 2>&1 | tail -20`. Two trailing forms are refused
// there because the shape around them changes their meaning: `|| true`
// (`a && b || true` applies it to the whole list) and 2>/dev/null combined with
// a 2>&1 merge. A shape whose wrapped command or grep stage went through the
// quoted path carries ReasonAllowlistedQuoted, so every guard on the quoted
// path (a deny or ask rule that holds a quote) still applies.

// ReasonAllowlistedCompound is the reason for a wrapped compound shape.
const ReasonAllowlistedCompound = "allowlisted_compound"

const (
	andSep    = " && "
	pipeSep   = " | "
	stderrDup = " 2>&1"
	// maxPipeLines bounds the -N of tail/head: five digits.
	maxPipeLines = 5
)

// decideCompound wraps the allowlisted core of a compound shape, or returns
// the plain policy's unsafe_character answer when the command is no shape.
func decideCompound(trimmed, exe string) Decision {
	rest, prefix, cdSeg := trimmed, "", ""
	if dir, after, ok := cdPrefix(rest); ok {
		cdSeg = "cd " + dir
		prefix = cdSeg + andSep
		rest = after
	}
	rest, suffix, pipeSegs, pipeKind, ok := pipeSuffix(rest)
	if !ok {
		return Decision{Reason: ReasonUnsafeChar}
	}
	if suffix == "" {
		rest, suffix = stderrSuffix(rest)
	}
	if prefix == "" && suffix == "" {
		return Decision{Reason: ReasonUnsafeChar}
	}
	inner := decideSimple(rest, exe)
	if inner.Reason == ReasonUnsafeChar {
		inner = decideQuotedPart(rest, exe, strings.HasPrefix(suffix, stderrDup))
	}
	if !inner.Rewrite {
		if pipeKind.quoted {
			return Decision{Reason: ReasonUnsafeChar}
		}
		return inner
	}
	if pipeKind.filter && (inner.Family == FamilyFileRead || inner.Family == FamilyWc || inner.Family == FamilyAgentDesktop) {
		return Decision{Reason: ReasonUnsafeChar}
	}
	inner.Command = prefix + inner.Command + suffix
	if pipeKind.quoted || inner.Reason == ReasonAllowlistedQuoted {
		inner.Reason = ReasonAllowlistedQuoted
	} else {
		inner.Reason = ReasonAllowlistedCompound
	}
	if cdSeg != "" {
		inner.Segments = append(inner.Segments, cdSeg)
	}
	inner.Segments = append(inner.Segments, pipeSegs...)
	return inner
}

// decideQuotedPart is the quoted policy for the command a shape or a chain
// wraps. It answers unsafe_character for anything it does not wrap, so a
// quoted command that is not allowlisted is never kept as written in a chain,
// and it refuses the trailing forms whose meaning the surrounding shape
// changes: `|| true` always, 2>/dev/null when a 2>&1 merge follows.
func decideQuotedPart(part, exe string, mergesStderr bool) Decision {
	if strings.HasSuffix(part, orTrueTail) || (mergesStderr && strings.HasSuffix(part, devNullTail)) {
		return Decision{Reason: ReasonUnsafeChar}
	}
	if d := decideQuoted(part, exe); d.Rewrite {
		return d
	}
	return Decision{Reason: ReasonUnsafeChar}
}

// cdPrefix splits `cd <dir> && rest`. dir is one safe word that is not an
// option.
func cdPrefix(command string) (dir, rest string, ok bool) {
	head, rest, found := strings.Cut(command, andSep)
	if !found {
		return "", "", false
	}
	words := strings.Fields(head)
	if len(words) != 2 || words[0] != "cd" || strings.HasPrefix(words[1], "-") || !allSafe(words[1]) {
		return "", "", false
	}
	return words[1], strings.TrimLeft(rest, " "), true
}

type pipeKindFlags struct {
	filter bool
	quoted bool
}

// pipeSuffix splits `core [2>&1] | <lineFilter|pipeFilter>`. suffix is what
// follows the wrapped command in the rewrite; segs are the pipe's command
// forms for the permission check. No pipe returns ok=true and empty parts; an
// unsafe or unsupported pipe returns ok=false.
func pipeSuffix(command string) (core, suffix string, segs []string, kind pipeKindFlags, ok bool) {
	i := lastUnquotedPipe(command)
	if i < 0 {
		return command, "", nil, kind, true
	}
	tail := strings.TrimLeft(command[i+len(pipeSep):], " ")
	core = strings.TrimRight(command[:i], " ")
	var pipeOut string
	words := strings.Fields(tail)
	if allSafe(tail) && lineFilter(words) {
		seg := strings.Join(words, " ")
		pipeOut, segs = seg, []string{seg}
	} else {
		var filterOK bool
		pipeOut, segs, kind.quoted, filterOK = parsePipeFilter(tail)
		if !filterOK {
			return command, "", nil, pipeKindFlags{}, false
		}
		kind.filter = true
	}
	strippedCore, redirKind, redirOK := extractStderrRedirect(core)
	if !redirOK {
		return command, "", nil, pipeKindFlags{}, false
	}
	redirect := ""
	if redirKind == redirMerge {
		redirect = stderrDup
		core = strippedCore
	}
	return core, redirect + pipeSep + pipeOut, segs, kind, true
}

func lastUnquotedPipe(command string) int {
	last := -1
	var quote byte
	for i := 0; i < len(command); i++ {
		c := command[i]
		if quote != 0 {
			if c == quote {
				quote = 0
			}
			continue
		}
		if c == '\'' || c == '"' {
			quote = c
			continue
		}
		if strings.HasPrefix(command[i:], pipeSep) {
			last = i
			i += len(pipeSep) - 1
		}
	}
	return last
}

// stderrSuffix splits a single `2>&1` off a command that has no pipe. Every
// other redirect (`>file`, `1>&2`, or multiple redirects) keeps the command
// unsafe; `2>/dev/null` is left for decideQuoted to turn into --discard-stderr.
func stderrSuffix(command string) (core, suffix string) {
	c, kind, ok := extractStderrRedirect(command)
	if !ok || kind != redirMerge {
		return command, ""
	}
	return c, stderrDup
}

// lineFilter reports whether words are `tail|head` (default 10 lines) or
// `tail|head -N`, `-n N`, or `-nN`, with N a short positive count.
func lineFilter(words []string) bool {
	if len(words) < 1 || len(words) > 3 || (words[0] != "tail" && words[0] != "head") {
		return false
	}
	if len(words) == 1 {
		return true
	}
	switch {
	case len(words) == 3:
		return words[1] == "-n" && count(words[2])
	case strings.HasPrefix(words[1], "-n"):
		return count(words[1][2:])
	case strings.HasPrefix(words[1], "-"):
		return count(words[1][1:])
	}
	return false
}

func count(s string) bool {
	if s == "" || len(s) > maxPipeLines {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

func allSafe(s string) bool {
	for i := 0; i < len(s); i++ {
		if !safeByte(s[i]) {
			return false
		}
	}
	return true
}
