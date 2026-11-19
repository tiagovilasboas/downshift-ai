// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import "strings"

type textPipeSpec struct {
	shortPlain   string
	shortCount   string
	shortValue   string
	longPlain    []string
	longCount    []string
	longValue    []string
	needSelector bool
}

var sortPipeSpec = textPipeSpec{
	shortPlain: "urnhVfbiCcgsMd",
	shortValue: "kt",
	longPlain: []string{
		"unique", "reverse", "numeric-sort", "human-numeric-sort",
		"version-sort", "ignore-case", "ignore-leading-blanks",
		"ignore-nonprinting", "check", "stable",
		"general-numeric-sort", "month-sort", "dictionary-order",
	},
	longValue: []string{"key", "field-separator"},
}

var uniqPipeSpec = textPipeSpec{
	shortPlain: "cdui",
	shortCount: "wsf",
	longPlain:  []string{"count", "repeated", "unique", "ignore-case"},
	longCount:  []string{"check-chars", "skip-chars", "skip-fields"},
}

var wcPipeSpec = textPipeSpec{
	shortPlain: "lwcmL",
	longPlain:  []string{"lines", "words", "bytes", "chars", "max-line-length"},
}

var cutPipeSpec = textPipeSpec{
	shortPlain:   "s",
	shortValue:   "dfcb",
	longPlain:    []string{"complement", "only-delimited"},
	longValue:    []string{"delimiter", "fields", "characters", "bytes", "output-delimiter"},
	needSelector: true,
}

// validTextPipeSpec reports whether prog + args is a safe stdin-only text
// filter (`sort`, `uniq`, `wc`, or `cut`). Positional file operands and
// file/process/memory flags (-o, --output, --files0-from, --compress-program,
// -S/--buffer-size, ...) are strictly refused.
func validTextPipeSpec(prog string, args []string) bool {
	switch prog {
	case "sort":
		return validTextFilterArgs(args, sortPipeSpec)
	case "uniq":
		return validTextFilterArgs(args, uniqPipeSpec)
	case "wc":
		return validTextFilterArgs(args, wcPipeSpec)
	case "cut":
		return validTextFilterArgs(args, cutPipeSpec)
	default:
		return false
	}
}

func validTextFilterArgs(args []string, spec textPipeSpec) bool {
	hasSelector := false
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-" || a == "--":
			return false
		case strings.HasPrefix(a, "--"):
			nextIdx, sel, ok := parseTextLongFlag(a, args, i, spec)
			if !ok {
				return false
			}
			hasSelector = hasSelector || sel
			i = nextIdx
		case strings.HasPrefix(a, "-"):
			nextIdx, sel, ok := parseTextShortFlags(a[1:], args, i, spec)
			if !ok {
				return false
			}
			hasSelector = hasSelector || sel
			i = nextIdx
		default:
			// Positional file operands are strictly refused for stdin-only filters.
			return false
		}
	}
	return !spec.needSelector || hasSelector
}

func parseTextLongFlag(a string, args []string, idx int, spec textPipeSpec) (nextIdx int, selector, ok bool) {
	name, val, hasVal := strings.Cut(a[2:], "=")
	switch {
	case inList(spec.longPlain, name):
		if name == "check" && hasVal {
			return idx, false, val == "quiet" || val == "silent" || val == "diagnose-first"
		}
		return idx, false, !hasVal
	case inList(spec.longCount, name):
		if !hasVal {
			if idx+1 >= len(args) {
				return idx, false, false
			}
			val = args[idx+1]
			idx++
		}
		return idx, false, count(val)
	case inList(spec.longValue, name):
		if !hasVal {
			if idx+1 >= len(args) {
				return idx, false, false
			}
			val = args[idx+1]
			idx++
		}
		if val == "" && name != "delimiter" && name != "output-delimiter" {
			return idx, false, false
		}
		sel := name == "fields" || name == "characters" || name == "bytes"
		return idx, sel, true
	default:
		return idx, false, false
	}
}

func parseTextShortFlags(cluster string, args []string, idx int, spec textPipeSpec) (nextIdx int, selector, ok bool) {
	if cluster == "" {
		return idx, false, false
	}
	for j := 0; j < len(cluster); j++ {
		c := cluster[j]
		switch {
		case strings.IndexByte(spec.shortPlain, c) >= 0:
		case strings.IndexByte(spec.shortCount, c) >= 0:
			if j+1 < len(cluster) {
				return idx, false, count(cluster[j+1:])
			}
			if idx+1 >= len(args) || !count(args[idx+1]) {
				return idx, false, false
			}
			return idx + 1, false, true
		case strings.IndexByte(spec.shortValue, c) >= 0:
			var val string
			if j+1 < len(cluster) {
				val = cluster[j+1:]
				nextIdx = idx
			} else {
				if idx+1 >= len(args) {
					return idx, false, false
				}
				val = args[idx+1]
				nextIdx = idx + 1
			}
			if val == "" && c != 'd' {
				return idx, false, false
			}
			sel := c == 'f' || c == 'c' || c == 'b'
			return nextIdx, sel, true
		default:
			return idx, false, false
		}
	}
	return idx, false, true
}
