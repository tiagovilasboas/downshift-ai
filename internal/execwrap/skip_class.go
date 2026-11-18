// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"encoding/json"
	"strings"
)

// Metachar classes: which shell metacharacter kept a command outside the
// safe-byte gate. The class names the first such byte; the log never holds the
// command text.
const (
	MetaSemicolon  = "semicolon"
	MetaAnd        = "and"
	MetaOr         = "or"
	MetaPipe       = "pipe"
	MetaBackground = "background"
	MetaSubst      = "subst"
	MetaBacktick   = "backtick"
	MetaRedirect   = "redirect"
	MetaQuote      = "quote"
	MetaNewline    = "newline"
	MetaGlob       = "glob"
	MetaBackslash  = "backslash"
	MetaParen      = "paren"
	MetaOther      = "other"
)

// MetacharClasses lists every class, for tests and the docs.
var MetacharClasses = []string{
	MetaSemicolon, MetaAnd, MetaOr, MetaPipe, MetaBackground, MetaSubst,
	MetaBacktick, MetaRedirect, MetaQuote, MetaNewline, MetaGlob,
	MetaBackslash, MetaParen, MetaOther,
}

// MetacharClass returns the class of the first byte of command the policy
// still refuses, or "" when there is none.
func MetacharClass(command string) string {
	i := firstUnsafe(command)
	if i < 0 {
		return ""
	}
	next := byte(0)
	if i+1 < len(command) {
		next = command[i+1]
	}
	switch c := command[i]; c {
	case ';':
		return MetaSemicolon
	case '&':
		switch next {
		case '&':
			return MetaAnd
		case '>':
			return MetaRedirect
		}
		return MetaBackground
	case '|':
		if next == '|' {
			return MetaOr
		}
		return MetaPipe
	case '$':
		return MetaSubst
	case '`':
		return MetaBacktick
	case '<', '>':
		return MetaRedirect
	case '\'', '"':
		return MetaQuote
	case '\n', '\r':
		return MetaNewline
	case '*', '?', '[', ']', '{', '}', '~':
		return MetaGlob
	case '\\':
		return MetaBackslash
	case '(', ')':
		return MetaParen
	}
	return MetaOther
}

// firstUnsafe is the index of the first byte the policy still refuses, or -1.
// A closed quoted word and an exact trailing 2>/dev/null or || true are
// accepted (policy_quoted.go), so they are not what a skip is classified by.
func firstUnsafe(command string) int { return firstRefused(command) }

// SkipDetail returns the metacharacter class (for SkipMetachar only) and the
// command family of a skip. For SkipMetachar, the family comes from the words
// before the first metacharacter; for any other reason, class is "" and the
// family comes from safe argv parsing (including basename resolution for
// absPathAllowlisted commands). Only fixed codes are returned, never command
// text.
func SkipDetail(reason, command string) (class, family string) {
	if reason == "" {
		return "", ""
	}
	if reason != SkipMetachar {
		return "", string(commandFamily(command))
	}
	i := firstUnsafe(command)
	if i < 0 {
		return "", ""
	}
	_, argv := splitEnvAssignments(parseWords(command[:i]))
	return MetacharClass(command), string(FamilyOf(argv))
}

func commandFamily(command string) Family {
	words := safeCommandWords(command)
	if len(words) == 0 {
		return FamilyNone
	}
	_, argv := splitEnvAssignments(words)
	if fam := FamilyOf(argv); fam != FamilyNone {
		return fam
	}
	return absPathFamily(argv)
}

func safeCommandWords(command string) []string {
	trimmed := strings.TrimSpace(command)
	if trimmed == "" {
		return nil
	}
	core, _ := cutShellTail(trimmed)
	if lexed, bad := lexQuoted(core); bad < 0 {
		return wordTexts(lexed)
	}
	i := firstUnsafe(trimmed)
	if i <= 0 {
		return nil
	}
	return parseWords(trimmed[:i])
}

func parseWords(prefix string) []string {
	if lexed, bad := lexQuoted(prefix); bad < 0 {
		return wordTexts(lexed)
	}
	return strings.Fields(prefix)
}

func wordTexts(lexed []shellWord) []string {
	out := make([]string, len(lexed))
	for i, w := range lexed {
		out[i] = w.text
	}
	return out
}

// SummarizeSkipClasses counts the metachar skips of the log by class. Rows
// written before the class existed count under no class. A missing file is
// empty.
func SummarizeSkipClasses(p string) (map[string]int64, error) {
	var out map[string]int64
	err := scanLog(p, func(line []byte) {
		var ev SkipEvent
		if json.Unmarshal(line, &ev) != nil || ev.Metachar == "" {
			return
		}
		if out == nil {
			out = map[string]int64{}
		}
		out[ev.Metachar]++
	})
	return out, err
}

// SummarizeSkipKinds counts the metachar skips of the log by MetacharKind.
// Rows written before the kind existed count under none. A missing file is
// empty.
func SummarizeSkipKinds(p string) (map[string]int64, error) {
	var out map[string]int64
	err := scanLog(p, func(line []byte) {
		var ev SkipEvent
		if json.Unmarshal(line, &ev) != nil || ev.Kind == "" {
			return
		}
		if out == nil {
			out = map[string]int64{}
		}
		out[ev.Kind]++
	})
	return out, err
}
