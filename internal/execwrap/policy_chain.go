// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import "strings"

// Chains: a list of commands joined by `;` or `&&`, the form agents use to
// run a formatter or a diff before the tests.
//
//	gofmt -l x; go test ./pkg
//	git diff && go test ./... 2>&1 | tail -15
//
// Each segment is decided alone, by the simple policy or a compound shape;
// only allowlisted segments are wrapped, every other segment and every
// separator stays byte-identical. The wrapper keeps each segment's exit code,
// so `;` still returns the last segment's code and `&&` still short-circuits.
//
// A wrapped segment may hold quoted words (decideQuotedPart); a segment kept
// as written may not. splitChain respects single and double quotes (including
// backslash escapes inside double quotes), so `;` and `&&` inside quotes stay
// part of their segment instead of splitting it. An unclosed quote fails open
// (returns ok=false / unsafe_character).
//
// One segment the policy cannot prove safe (a quote outside a wrapped
// command, `||`, a lone `&`, a redirect, an expansion, a pipe that is not
// `| tail/head -N`) refuses the whole command: no segment is wrapped. So does a segment kept as written that
// names a guarded program (rm, mysql, redis-cli): the guard refused it or never
// saw it, and the rewritten line must not carry it, because a harness that
// approves the rewrite approves the whole line.

// ReasonAllowlistedChain is the reason for a wrapped chain.
const ReasonAllowlistedChain = "allowlisted_chain"

// maxChainSegments bounds how many commands one chain may hold; a longer
// chain passes raw.
const maxChainSegments = 32

// ReasonGuardedSegment is the reason for a chain refused because a segment
// kept as written names a guarded program.
const ReasonGuardedSegment = "guarded_segment"

// wrappedMarks find an already wrapped command, bare or with a quoted path.
var wrappedMarks = []string{"downshift compact", "downshift' compact"}

// decideChain wraps the allowlisted segments of a `;`/`&&` chain, or refuses
// the whole command.
func decideChain(trimmed, exe string) Decision {
	for _, m := range wrappedMarks {
		if strings.Contains(trimmed, m) {
			return Decision{Reason: ReasonAlreadyWrapped}
		}
	}
	segs, seps, ok := splitChain(trimmed)
	if !ok {
		return Decision{Reason: ReasonUnsafeChar}
	}
	if len(segs) > maxChainSegments {
		return Decision{Reason: ReasonTooLong}
	}
	var out strings.Builder
	var first *Decision
	var others []string
	quoted := false
	for i, seg := range segs {
		if i > 0 {
			out.WriteString(seps[i-1])
		}
		d, keep := decideSegment(seg, exe)
		if !keep && !d.Rewrite {
			return Decision{Family: d.Family, Reason: d.Reason}
		}
		if !d.Rewrite {
			if mentionsGuarded(strings.Fields(seg)) {
				return Decision{Reason: ReasonGuardedSegment}
			}
			out.WriteString(seg)
			others = append(others, strings.Join(strings.Fields(seg), " "))
			continue
		}
		out.WriteString(d.Command)
		quoted = quoted || d.Reason == ReasonAllowlistedQuoted
		others = append(others, d.Segments...)
		if first == nil {
			d := d
			first = &d
			continue
		}
		others = append(others, d.Argv)
	}
	if first == nil {
		return Decision{Reason: ReasonNotAllowlisted}
	}
	return Decision{
		Rewrite:  true,
		Command:  out.String(),
		Family:   first.Family,
		Argv:     first.Argv,
		Segments: others,
		Reason:   chainReason(quoted),
	}
}

// chainReason keeps ReasonAllowlistedQuoted on a chain with a quoted segment,
// so the guards on the quoted path see it.
func chainReason(quoted bool) string {
	if quoted {
		return ReasonAllowlistedQuoted
	}
	return ReasonAllowlistedChain
}

// decideSegment decides one chain segment. keep is true when a segment that
// is not wrapped is still provably safe to leave as written: only safe bytes,
// a compound shape around a safe command, or a safe command with a trailing
// /dev/null discard redirect. A segment with quoted words is wrapped or
// refused, never kept (decideQuotedPart).
func decideSegment(seg, exe string) (d Decision, keep bool) {
	d = decideSimple(seg, exe)
	if d.Reason == ReasonUnsafeChar {
		d = decideCompound(seg, exe)
	}
	if d.Reason == ReasonUnsafeChar {
		d = decideQuotedPart(seg, exe, false)
	}
	return d, d.Rewrite || d.Reason == ReasonNotAllowlisted || safeKeptDiscardSegment(seg)
}

// safeKeptDiscardSegment reports whether seg is a plain safe-byte command with
// only a trailing >/dev/null, 2>/dev/null, or &>/dev/null redirect.
func safeKeptDiscardSegment(seg string) bool {
	trimmed := strings.TrimRight(seg, " ")
	for _, tail := range []string{" 2>/dev/null", " >/dev/null", " &>/dev/null"} {
		if core, ok := strings.CutSuffix(trimmed, tail); ok {
			core = strings.TrimSpace(core)
			return core != "" && allSafe(core)
		}
	}
	return false
}

// splitChain cuts a command at each unquoted `;` and `&&`. seps[i] is the exact
// text between segs[i] and segs[i+1], spaces included. ok is false with fewer
// than two segments, an empty segment, or an unclosed quote.
func splitChain(command string) (segs, seps []string, ok bool) {
	start := 0
	var quote byte
	for i := 0; i < len(command); i++ {
		c := command[i]
		if quote != 0 {
			switch {
			case quote == '"' && c == '\\':
				if i+1 < len(command) {
					i++
				}
			case c == quote:
				quote = 0
			}
			continue
		}
		if c == '\\' {
			if i+1 < len(command) {
				i++
			}
			continue
		}
		if c == '\'' || c == '"' {
			quote = c
			continue
		}
		n := 0
		switch {
		case c == ';':
			n = 1
		case c == '&' && i+1 < len(command) && command[i+1] == '&':
			n = 2
		default:
			continue
		}
		seg := strings.TrimRight(command[start:i], " ")
		next := i + n
		for next < len(command) && command[next] == ' ' {
			next++
		}
		if strings.TrimSpace(seg) == "" {
			return nil, nil, false
		}
		segs = append(segs, seg)
		seps = append(seps, command[start+len(seg):next])
		start = next
		i = next - 1
	}
	if quote != 0 {
		return nil, nil, false
	}
	last := command[start:]
	if len(segs) == 0 || strings.TrimSpace(last) == "" {
		return nil, nil, false
	}
	return append(segs, last), seps, true
}
