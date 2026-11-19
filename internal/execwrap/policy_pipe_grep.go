// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import "strings"

type pipeFilterSpec struct {
	shortPlain string
	shortCount string
	longPlain  []string
	longCount  []string
}

var grepPipeSpec = pipeFilterSpec{
	shortPlain: "ivEFGnclLwxoqsHhIa",
	shortCount: "ABCm",
	longPlain: []string{
		"ignore-case", "invert-match", "extended-regexp", "fixed-strings",
		"basic-regexp", "line-number", "count", "files-with-matches",
		"files-without-match", "word-regexp", "line-regexp", "only-matching",
		"quiet", "silent", "no-messages", "no-filename", "with-filename",
		"text", "line-buffered",
	},
	longCount: []string{"after-context", "before-context", "context", "max-count"},
}

var rgPipeSpec = pipeFilterSpec{
	shortPlain: "niSswxFHhNvcIloqaL",
	shortCount: "ABCm",
	longPlain: []string{
		"line-number", "no-line-number", "ignore-case", "smart-case",
		"case-sensitive", "invert-match", "word-regexp", "line-regexp",
		"fixed-strings", "with-filename", "no-filename", "no-heading",
		"count", "files-with-matches", "only-matching", "quiet", "text",
		"line-buffered",
	},
	longCount: []string{"after-context", "before-context", "context", "max-count"},
}

// parsePipeGrep validates a trailing `| grep ...` or `| rg ...` stdin filter
// stage. Only safe filtering flags and a single pattern (positional or via
// -e/--regexp) are accepted; file operands, -f/--file, -r/-R, --pre, -z and
// unsafe redirects are refused.
func parsePipeGrep(tail string) (outSuffix string, segs []string, quoted, ok bool) {
	core, redir, redirOK := extractStderrRedirect(tail)
	if !redirOK || core == "" {
		return "", nil, false, false
	}
	words, bad := lexQuoted(core)
	if bad >= 0 || len(words) < 2 || !words[0].plain {
		return "", nil, false, false
	}
	argv := make([]string, len(words))
	for i, w := range words {
		argv[i] = w.text
		if !w.plain {
			quoted = true
		}
	}
	if !validGrepPipeSpec(argv[0], argv[1:]) {
		return "", nil, false, false
	}
	outSuffix, segs = formatPipeGrepSuffix(words, argv, redir, quoted)
	return outSuffix, segs, quoted, true
}

func validGrepPipeSpec(prog string, args []string) bool {
	switch prog {
	case "grep", "egrep", "fgrep":
		return validPipeFilterArgs(args, grepPipeSpec)
	case "rg":
		return validPipeFilterArgs(args, rgPipeSpec)
	default:
		return false
	}
}

func validPipeFilterArgs(args []string, spec pipeFilterSpec) bool {
	var operands []string
	explicit, endOpts := false, false
	for i := 0; i < len(args); i++ {
		a := args[i]
		if endOpts {
			operands = append(operands, a)
			continue
		}
		switch {
		case a == "--":
			endOpts = true
		case a == "-":
			return false
		case strings.HasPrefix(a, "--"):
			nextIdx, exp, ok := parsePipeLongFlag(a, args, i, spec)
			if !ok {
				return false
			}
			explicit = explicit || exp
			i = nextIdx
		case strings.HasPrefix(a, "-"):
			nextIdx, exp, ok := parsePipeShortFlags(a[1:], args, i, spec)
			if !ok {
				return false
			}
			explicit = explicit || exp
			i = nextIdx
		default:
			operands = append(operands, a)
		}
	}
	if explicit {
		return len(operands) == 0
	}
	return len(operands) == 1
}

func parsePipeLongFlag(a string, args []string, idx int, spec pipeFilterSpec) (nextIdx int, explicit, ok bool) {
	name, val, hasVal := strings.Cut(a[2:], "=")
	switch {
	case inList(spec.longPlain, name) && !hasVal:
		return idx, false, true
	case name == "color" || name == "colour":
		return idx, false, hasVal && (val == "never" || val == "auto" || val == "always")
	case name == "regexp":
		if hasVal {
			return idx, true, true
		}
		if idx+1 >= len(args) {
			return idx, false, false
		}
		return idx + 1, true, true
	case inList(spec.longCount, name):
		if !hasVal {
			if idx+1 >= len(args) {
				return idx, false, false
			}
			val = args[idx+1]
			idx++
		}
		return idx, false, count(val)
	default:
		return idx, false, false
	}
}

func parsePipeShortFlags(cluster string, args []string, idx int, spec pipeFilterSpec) (nextIdx int, explicit, ok bool) {
	if cluster == "" {
		return idx, false, false
	}
	for j := 0; j < len(cluster); j++ {
		c := cluster[j]
		switch {
		case strings.IndexByte(spec.shortPlain, c) >= 0:
		case c == 'e':
			if j != len(cluster)-1 || idx+1 >= len(args) {
				return idx, false, false
			}
			return idx + 1, true, true
		case strings.IndexByte(spec.shortCount, c) >= 0:
			if j+1 < len(cluster) {
				return idx, false, count(cluster[j+1:])
			}
			if idx+1 >= len(args) || !count(args[idx+1]) {
				return idx, false, false
			}
			return idx + 1, false, true
		default:
			return idx, false, false
		}
	}
	return idx, false, true
}

func formatPipeGrepSuffix(words []shellWord, argv []string, redir stderrRedirect, quoted bool) (string, []string) {
	rawWords := make([]string, len(words))
	quotedWords := make([]string, len(words))
	for i, w := range words {
		rawWords[i] = w.raw
		quotedWords[i] = QuoteWord(w.text)
	}
	rawSeg := strings.Join(rawWords, " ")
	unquotedSeg := strings.Join(argv, " ")
	var segs []string
	out := unquotedSeg
	if quoted {
		out = strings.Join(quotedWords, " ")
		segs = append(segs, rawSeg)
		if unquotedSeg != rawSeg {
			segs = append(segs, unquotedSeg)
		}
	} else {
		segs = append(segs, unquotedSeg)
	}
	switch redir {
	case redirMerge:
		out += " 2>&1"
		segs = append(segs, unquotedSeg+" 2>&1")
	case redirDiscard:
		out += " 2>/dev/null"
		segs = append(segs, unquotedSeg+" 2>/dev/null")
	}
	return out, segs
}
