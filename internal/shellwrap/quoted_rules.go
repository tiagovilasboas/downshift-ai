// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package shellwrap

import (
	"path"
	"path/filepath"
	"strings"

	"github.com/tiagovilasboas/downshift-ai/internal/execwrap"
)

// ReasonQuotedRule is the skip code for a quoted command left unwrapped
// because a shell deny or ask rule holds a quote.
const ReasonQuotedRule = "quoted_rule"

// ShellRuleReader is implemented by adapters that can read the harness's own
// deny and ask rules for its shell tool. The adapter only reads them; the
// policy on them lives here.
type ShellRuleReader interface {
	// ShellDenyAskRules returns the shell tool's deny and ask rules as
	// written. A read error is returned, so the policy fails closed.
	ShellDenyAskRules() ([]string, error)
}

// refuseQuotedUnderQuotedRules keeps the quoted path off when a shell deny or
// ask rule holding ' or " can plausibly apply to the command being rewritten.
// A quoted rule scoped to a different specific binary (e.g. Bash(git commit -m
// "wip") when running grep or go test) does not block unrelated commands. An
// unreadable rule set, or a quoted rule not scoped to a specific binary (such as
// Bash(*) or globbed binaries), fails closed.
func refuseQuotedUnderQuotedRules(ev any, d execwrap.Decision) execwrap.Decision {
	if !d.Rewrite || d.Reason != execwrap.ReasonAllowlistedQuoted {
		return d
	}
	r, ok := ev.(ShellRuleReader)
	if !ok {
		return d
	}
	rules, err := r.ShellDenyAskRules()
	if err != nil {
		return execwrap.Decision{Family: d.Family, Reason: ReasonQuotedRule}
	}
	if anyQuotedRuleApplies(rules, commandBinaries(ev, d)) {
		return execwrap.Decision{Family: d.Family, Reason: ReasonQuotedRule}
	}
	return d
}

func anyQuotedRuleApplies(rules []string, cmdBins map[string]bool) bool {
	for _, r := range rules {
		if !strings.ContainsAny(r, `'"`) {
			continue
		}
		if len(cmdBins) == 0 {
			return true
		}
		bins, ok := quotedRuleBinaries(r)
		if !ok {
			return true
		}
		for _, b := range bins {
			if cmdBins[b] {
				return true
			}
		}
	}
	return false
}

// commandBinaries collects normalized basenames of every command binary across
// the decision's argv, segments, and raw command text (if available).
func commandBinaries(ev any, d execwrap.Decision) map[string]bool {
	bins := map[string]bool{}
	addBins := func(s string) {
		for _, seg := range splitShellSegments(s) {
			if bin, ok := segmentBinary(seg); ok {
				bins[bin] = true
			}
		}
	}
	addBins(d.Argv)
	for _, seg := range d.Segments {
		addBins(seg)
	}
	if sc, ok := ev.(interface{ ShellCommand() (string, bool) }); ok {
		if cmd, isShell := sc.ShellCommand(); isShell {
			addBins(cmd)
		}
	}
	return bins
}

// quotedRuleBinaries extracts the normalized binary basenames from a quoted
// Bash(...) rule. It returns ok=false (fail closed) if the rule is not Bash(...),
// has an unclosed quote, or any segment lacks a concrete non-wildcard binary.
func quotedRuleBinaries(rule string) ([]string, bool) {
	trimmed := strings.TrimSpace(rule)
	if !strings.HasPrefix(trimmed, "Bash(") || !strings.HasSuffix(trimmed, ")") {
		return nil, false
	}
	pattern := strings.TrimSpace(trimmed[len("Bash(") : len(trimmed)-1])
	if pattern == "" {
		return nil, false
	}
	segs := splitShellSegments(pattern)
	if len(segs) == 0 {
		return nil, false
	}
	bins := make([]string, 0, len(segs))
	for _, seg := range segs {
		bin, ok := segmentBinary(seg)
		if !ok {
			return nil, false
		}
		bins = append(bins, bin)
	}
	return bins, true
}

// splitShellSegments splits s on unquoted ;, &&, ||, and |. Returns nil if a
// quote is unclosed so callers fail closed.
func splitShellSegments(s string) []string {
	var segs []string
	var quote byte
	start := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if quote != 0 {
			if quote == '"' && c == '\\' && i+1 < len(s) {
				i++
				continue
			}
			if c == quote {
				quote = 0
			}
			continue
		}
		if c == '\\' && i+1 < len(s) {
			i++
			continue
		}
		if c == '\'' || c == '"' {
			quote = c
			continue
		}
		sep := 0
		switch c {
		case ';':
			sep = 1
		case '&', '|':
			if i+1 < len(s) && s[i+1] == c {
				sep = 2
			} else if c == '|' {
				sep = 1
			}
		}
		if sep > 0 {
			if seg := strings.TrimSpace(s[start:i]); seg != "" {
				segs = append(segs, seg)
			}
			start = i + sep
			i = start - 1
		}
	}
	if quote != 0 {
		return nil
	}
	if seg := strings.TrimSpace(s[start:]); seg != "" {
		segs = append(segs, seg)
	}
	return segs
}

// segmentBinary returns the normalized basename of the command in seg, skipping
// leading NAME=value assignments, or ok=false if missing, unclosed, or patterned.
func segmentBinary(seg string) (string, bool) {
	i := 0
	for i < len(seg) {
		for i < len(seg) && (seg[i] == ' ' || seg[i] == '\t') {
			i++
		}
		if i >= len(seg) {
			break
		}
		word, next, hasMeta, ok := readRuleWord(seg, i)
		if !ok || hasMeta {
			return "", false
		}
		i = next
		if isEnvAssignWord(word) {
			continue
		}
		base := path.Base(filepath.ToSlash(word))
		if base == "" || base == "." || base == ".." || base == "/" {
			return "", false
		}
		return base, true
	}
	return "", false
}

// readRuleWord reads one whitespace-delimited shell word starting at i,
// unquoting '...' and "...". hasMeta reports whether any wildcard or shell
// pattern byte was seen in the word.
func readRuleWord(s string, i int) (word string, next int, hasMeta, ok bool) {
	var b strings.Builder
	var quote byte
	for i < len(s) {
		c := s[i]
		if quote != 0 {
			if quote == '"' && c == '\\' {
				return "", i, true, false
			}
			if c == quote {
				quote = 0
				i++
				continue
			}
			if isBinaryMetaByte(c) {
				hasMeta = true
			}
			b.WriteByte(c)
			i++
			continue
		}
		if c == ' ' || c == '\t' {
			break
		}
		if c == '\'' || c == '"' {
			quote = c
			i++
			continue
		}
		if isBinaryMetaByte(c) {
			hasMeta = true
		}
		b.WriteByte(c)
		i++
	}
	if quote != 0 {
		return "", i, false, false
	}
	return b.String(), i, hasMeta, b.Len() > 0
}

func isBinaryMetaByte(c byte) bool {
	return strings.IndexByte("*?[]{}()$`\\!~:<>&|;", c) >= 0
}

func isEnvAssignWord(word string) bool {
	name, _, found := strings.Cut(word, "=")
	if !found || name == "" {
		return false
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		letter := c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '_'
		if !letter && (i == 0 || c < '0' || c > '9') {
			return false
		}
	}
	return true
}
