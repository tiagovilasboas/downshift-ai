// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import "strings"

// A compound command (a pipe, a list, a redirect) has no family: its output is
// not one program's alone, so no compressor reads it. To know which programs
// sit inside the compound commands that reach the post-run hook, and so where
// widening the wrapper would pay, the exec log records InnerFamily and the
// command's MetacharKind on those rows. Both are fixed codes; the text is read
// in memory only.

// InnerFamily returns the family of the first allowlisted program among the
// segments of command, split at unquoted `|`, `&`, `;`, `(`, `)` and newlines,
// with a segment read up to its first redirect. It returns FamilyNone when no
// segment names one. It is a measurement aid, never a wrap decision: a segment
// it names is not proven safe to wrap.
func InnerFamily(command string) Family {
	for _, seg := range compoundSegments(command) {
		if f := segmentFamily(seg); f != FamilyNone {
			return f
		}
	}
	return FamilyNone
}

// CompoundRefusesRawStore reports whether any segment of command runs a family
// whose raw output must never reach the raw-output store (curl, wget, glab).
// Unlike InnerFamily it looks at every segment, so `git log | curl ...` is seen.
func CompoundRefusesRawStore(command string) bool {
	for _, seg := range compoundSegments(command) {
		if RawStoreRefused(segmentFamily(seg)) {
			return true
		}
	}
	return false
}

// segmentFamily is the family of the program one segment names.
func segmentFamily(seg string) Family {
	if i := strings.IndexAny(seg, "<>"); i >= 0 {
		seg = seg[:i]
	}
	seg = stripFuncHeader(seg)
	words := strings.Fields(seg)
	for i, w := range words {
		words[i] = strings.Trim(w, `'"`)
	}
	for len(words) > 0 {
		w := words[0]
		if w == "for" {
			return FamilyNone
		}
		if w == "{" || w == "}" || w == "!" || shellControlKeywords[w] {
			words = words[1:]
			continue
		}
		break
	}
	_, argv := splitEnvAssignments(words)
	if len(argv) == 0 || argv[0] == "downshift" {
		return FamilyNone
	}
	return FamilyOf(argv)
}

// compoundSegments splits command at the separators InnerFamily names, outside
// single and double quotes. A `$(` opens a segment too, so the program of a
// substitution is seen. Unbalanced quotes end the scan with what was split.
func compoundSegments(command string) []string {
	var segs []string
	start := 0
	var quote byte
	for i := 0; i < len(command); i++ {
		c := command[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case c == '\'' || c == '"':
			quote = c
		case strings.IndexByte("|&;()\n`", c) >= 0:
			segs = append(segs, command[start:i])
			start = i + 1
		case c == '$' && i+1 < len(command) && command[i+1] == '(':
			segs = append(segs, command[start:i])
			start = i + 2
			i++
		}
	}
	return append(segs, command[start:])
}

// CompoundShape returns InnerFamily and MetacharKind of a command the policy
// refuses for a metacharacter, and "", "" for any other command (a simple
// command that is merely not allowlisted has no shape to report).
func CompoundShape(command string) (inner, kind string) {
	if !hasUnsafeByte(command) {
		return "", ""
	}
	return string(InnerFamily(command)), MetacharKind(command)
}
