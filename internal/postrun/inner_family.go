// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package postrun

import (
	"strings"

	"github.com/tiagovilasboas/downshift-ai/internal/execwrap"
)

const maxPostRunCommandBytes = 4096

// extractPostRunFamily extracts a safe allowlisted family and normalized argv
// when execwrap.SimpleFamily returned FamilyNone (for example `/abs/path/cmd`,
// `VAR=x cmd`, quoted arguments, or `cd dir && cmd` / single-family `;`/`&&`
// compounds on exit 0). Pipes, redirects (other than `2>&1` / `2>/dev/null`),
// command substitutions, backgrounding, and mixed-family compounds return
// FamilyNone so postrun falls back to search-cut or balanced-cut rules.
func extractPostRunFamily(command string, exit int) (execwrap.Family, []string) {
	return extractFamily(command, exit, false)
}

func extractFamily(command string, exit int, allowPipes bool) (execwrap.Family, []string) {
	trimmed := strings.TrimSpace(command)
	if trimmed == "" || len(trimmed) > maxPostRunCommandBytes {
		return execwrap.FamilyNone, nil
	}
	head, stages, piped, ok := splitPipeStages(trimmed)
	if !ok || (piped && !pipeAllowed(trimmed, exit, allowPipes)) {
		return execwrap.FamilyNone, nil
	}
	segs, ok := splitSafeChain(head)
	if !ok || len(segs) == 0 {
		return execwrap.FamilyNone, nil
	}
	if len(segs) > 1 && exit != 0 {
		return execwrap.FamilyNone, nil
	}
	for len(segs) > 0 && isSilentSetup(segs[0]) {
		segs = segs[1:]
	}
	if len(segs) == 0 {
		return execwrap.FamilyNone, nil
	}
	var firstFam execwrap.Family
	var firstArgv []string
	for _, seg := range segs {
		fam, argv, valid := extractSegmentFamily(seg)
		if !valid {
			return execwrap.FamilyNone, nil
		}
		if firstFam == execwrap.FamilyNone {
			firstFam, firstArgv = fam, argv
		} else if fam != firstFam {
			return execwrap.FamilyNone, nil
		}
	}
	if piped && !pipeStagesHarmless(stages, firstFam) {
		return execwrap.FamilyNone, nil
	}
	return firstFam, firstArgv
}

// splitSafeChain splits command on unquoted `;` and `&&` while rejecting
// pipes, redirects (except `2>&1` / `2>/dev/null`), substitutions, subshells,
// background `&`, and newlines.
func splitSafeChain(command string) ([]string, bool) {
	var segs []string
	start := 0
	var quote byte
	for i := 0; i < len(command); i++ {
		c := command[i]
		switch {
		case c == '\\' && quote != '\'':
			if i+1 >= len(command) {
				return nil, false
			}
			i++
		case quote != 0:
			if c == quote {
				quote = 0
			} else if quote == '"' && (c == '$' || c == '`') {
				return nil, false
			}
		case c == '\'' || c == '"':
			quote = c
		case c == ';':
			segs = append(segs, strings.TrimSpace(command[start:i]))
			start = i + 1
		case c == '&':
			if i+1 < len(command) && command[i+1] == '&' {
				segs = append(segs, strings.TrimSpace(command[start:i]))
				start = i + 2
				i++
				continue
			}
			return nil, false
		case c == '2' && (i == 0 || command[i-1] == ' ') && hasRedirToken(command[i:], "2>&1"):
			i += 3
		case c == '2' && (i == 0 || command[i-1] == ' ') && hasRedirToken(command[i:], "2>/dev/null"):
			i += 10
		case c == '|' || c == '<' || c == '>' || c == '(' || c == ')' || c == '`' || c == '$' || c == '\n' || c == '\r':
			return nil, false
		}
	}
	if quote != 0 {
		return nil, false
	}
	return append(segs, strings.TrimSpace(command[start:])), true
}

func isSilentSetup(seg string) bool {
	if seg == "" {
		return true
	}
	words, ok := lexShellWords(seg)
	if !ok || len(words) == 0 {
		return false
	}
	switch words[0] {
	case "cd", "pushd", "export", "unset", "true", "mkdir":
		return true
	}
	return false
}

func extractSegmentFamily(seg string) (execwrap.Family, []string, bool) {
	seg = strings.TrimSpace(seg)
	seg = strings.TrimSuffix(seg, " 2>&1")
	seg = strings.TrimSuffix(seg, " 2>/dev/null")
	seg = strings.TrimSpace(seg)
	words, ok := lexShellWords(seg)
	if !ok || len(words) == 0 {
		return execwrap.FamilyNone, nil, false
	}
	assigns, argv := stripLeadingEnv(words)
	if len(argv) == 0 {
		return execwrap.FamilyNone, nil, false
	}
	exe := baseExeName(argv[0])
	if exe == "downshift" {
		return execwrap.FamilyNone, nil, false
	}
	argv[0] = exe
	if len(assigns) > 0 && (exe == "mysql" || exe == "redis-cli" || exe == "valkey-cli" || exe == "psql") {
		return execwrap.FamilyNone, nil, false
	}
	fam := execwrap.FamilyOf(argv)
	if fam == execwrap.FamilyNone || fam == execwrap.FamilyFileRead || fam == execwrap.FamilyWc || fam == execwrap.FamilyAgentDesktop {
		return execwrap.FamilyNone, nil, false
	}
	return fam, argv, true
}

func lexShellWords(s string) ([]string, bool) {
	var words []string
	var cur strings.Builder
	inWord := false
	var quote byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '\\' && quote != '\'':
			if i+1 >= len(s) {
				return nil, false
			}
			i++
			cur.WriteByte(s[i])
			inWord = true
		case quote != 0:
			if c == quote {
				quote = 0
			} else {
				cur.WriteByte(c)
			}
			inWord = true
		case c == '\'' || c == '"':
			quote = c
			inWord = true
		case c == ' ' || c == '\t':
			if inWord {
				words = append(words, cur.String())
				cur.Reset()
				inWord = false
			}
		default:
			cur.WriteByte(c)
			inWord = true
		}
	}
	if quote != 0 {
		return nil, false
	}
	if inWord {
		words = append(words, cur.String())
	}
	return words, true
}

func stripLeadingEnv(words []string) ([]string, []string) {
	n := 0
	for n < len(words) && isValidEnvAssignment(words[n]) {
		n++
	}
	return words[:n], words[n:]
}

func isValidEnvAssignment(word string) bool {
	name, _, found := strings.Cut(word, "=")
	if !found || name == "" {
		return false
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		letter := (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_'
		if !letter && (i == 0 || c < '0' || c > '9') {
			return false
		}
	}
	return true
}

func hasRedirToken(rest, tok string) bool {
	if !strings.HasPrefix(rest, tok) {
		return false
	}
	if len(rest) == len(tok) {
		return true
	}
	next := rest[len(tok)]
	return next == ' ' || next == ';' || next == '&'
}

func baseExeName(p string) string {
	if i := strings.LastIndexAny(p, "/\\"); i >= 0 {
		return p[i+1:]
	}
	return p
}
