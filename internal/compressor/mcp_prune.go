// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
)

const (
	mcpMaxStackFrames = 5
	mcpMaxURLLen      = 40
	mcpMaxUnwrapDepth = 3
)

var mcpEnvelopeKeys = []string{"result", "stdout", "data", "payload", "text", "content"}

var mcpProtectedKeys = map[string]bool{
	"id": true, "shortid": true, "short_id": true, "eventid": true, "event_id": true, "title": true,
	"culprit": true, "level": true, "status": true, "count": true, "usercount": true, "user_count": true,
	"firstseen": true, "first_seen": true, "lastseen": true, "last_seen": true, "timestamp": true,
	"timestamps": true, "datecreated": true, "date_created": true, "trace": true, "traceid": true,
	"trace_id": true, "span": true, "spanid": true, "span_id": true, "message": true, "error": true,
	"err": true, "value": true, "type": true,
}

var mcpBoilerplateKeys = map[string]bool{
	"permalink": true, "weburl": true, "shareid": true, "node_id": true, "_links": true, "links": true,
	"sdk": true, "breadcrumbs": true, "annotations": true, "activity": true, "pluginactions": true,
	"plugincontexts": true, "pluginissues": true, "ispublic": true, "isbookmarked": true,
	"issubscribed": true, "hasseen": true, "vars": true, "pre_context": true, "post_context": true,
}

var mcpRegexes = sync.OnceValue(func() struct {
	mdLink, emptyField, permalink, stackFrame *regexp.Regexp
} {
	return struct {
		mdLink, emptyField, permalink, stackFrame *regexp.Regexp
	}{
		mdLink:     regexp.MustCompile(`\[([^\]]+)\]\((https?://[^)\s]+)\)`),
		emptyField: regexp.MustCompile(`(?i)^\s*(?:[-*]\s+)?(?:\*\*[^*]+\*\*\s*:\s*|"?[a-z0-9_]+"?\s*:\s*)(?:null|none|n/a|""|\[\]|\{\})\s*,?\s*$`),
		permalink:  regexp.MustCompile(`(?i)^\s*(?:[-*]\s+)?(?:\*\*)?(?:permalink|web_?url|url|view in (?:sentry|datadog)|share_?url)(?:\*\*)?\s*:\s*https?://\S+\s*$`),
		stackFrame: regexp.MustCompile(`^\s*(?:at\s+\S+|File\s+"[^"]+",\s+line\s+\d+|in\s+\S+\s+at\s+|\d+:\s+\S+\()`),
	}
})

// PruneMCP prunes verbose Sentry (search_issues, search_events), Datadog
// (execute_code), and generic mcp__* payloads in JSON and Markdown/text formats.
func PruneMCP(toolName string, input []byte) ([]byte, bool) {
	if len(bytes.TrimSpace(input)) == 0 {
		return nil, false
	}
	target := detectMCPTarget(toolName)
	if target == "" {
		return pruneGenericMCP(toolName, input)
	}
	cur, unwrapped := unwrapMCPEnvelope(input)
	trim := bytes.TrimSpace(cur)
	if len(trim) > 0 && (trim[0] == '{' || trim[0] == '[') && json.Valid(trim) {
		if out, ok := pruneMCPJSON(target, trim); ok && len(out) < len(input) {
			return out, true
		}
		return trim, unwrapped && len(trim) < len(input)
	}
	if out, ok := pruneMCPText(cur); ok && len(out) < len(input) {
		return out, true
	}
	return cur, unwrapped && len(cur) < len(input)
}

func detectMCPTarget(toolName string) string {
	lower := strings.ToLower(toolName)
	switch {
	case strings.Contains(lower, "sentry") && strings.Contains(lower, "search_issues"):
		return "sentry_issues"
	case strings.Contains(lower, "sentry") && strings.Contains(lower, "search_events"):
		return "sentry_events"
	case strings.Contains(lower, "datadog") && strings.Contains(lower, "execute_code"):
		return "datadog_code"
	default:
		return ""
	}
}

func unwrapMCPEnvelope(input []byte) ([]byte, bool) {
	cur := bytes.TrimSpace(input)
	changed := false
	for depth := 0; depth < mcpMaxUnwrapDepth && len(cur) > 0 && cur[0] == '{'; depth++ {
		var obj map[string]any
		if json.Unmarshal(cur, &obj) != nil || len(obj) == 0 {
			break
		}
		var next string
		if len(obj) == 1 {
			for _, v := range obj {
				if s, ok := v.(string); ok {
					next = s
				}
			}
		}
		for _, k := range mcpEnvelopeKeys {
			if next != "" {
				break
			}
			v, ok := obj[k]
			if !ok {
				continue
			}
			if s, ok := v.(string); ok {
				if ts := strings.TrimSpace(s); ts != "" && (k == "stdout" || k == "result" || ts[0] == '{' || ts[0] == '[') {
					next = ts
				}
			} else if blocks, ok := v.([]any); ok && k == "content" && len(blocks) == 1 {
				if m, ok := blocks[0].(map[string]any); ok {
					next, _ = m["text"].(string)
				}
			}
		}
		if next = strings.TrimSpace(next); next == "" {
			break
		}
		cur, changed = []byte(next), true
	}
	return cur, changed
}

func pruneMCPJSON(target string, input []byte) ([]byte, bool) {
	val, err := decodeSingleJSON(input)
	if err != nil {
		return nil, false
	}
	pruned, _ := pruneMCPValue(target, val, "")
	out, err := encodeJSON(pruned, false)
	if err != nil {
		return nil, false
	}
	if out = bytes.TrimRight(out, "\n"); len(out) >= len(input) {
		return nil, false
	}
	return out, true
}

func pruneMCPValue(target string, v any, parentKey string) (any, bool) {
	switch x := v.(type) {
	case map[string]any:
		return pruneMCPObject(target, x)
	case []any:
		return pruneMCPArray(target, x, parentKey)
	case string:
		if strings.Contains(x, "\n") && (strings.Contains(parentKey, "stack") || strings.Contains(x, "\n    at ") || strings.Contains(x, "\n  File ")) {
			if cut, ok := pruneMCPText([]byte(x)); ok {
				return string(cut), true
			}
		}
		return v, false
	default:
		return v, false
	}
}

func pruneMCPObject(target string, obj map[string]any) (map[string]any, bool) {
	out := make(map[string]any, len(obj))
	changed := false
	for k, v := range obj {
		if isRedundantMCPKey(target, k, v) {
			changed = true
			continue
		}
		pv, childChanged := pruneMCPValue(target, v, strings.ToLower(k))
		changed = changed || childChanged
		if isRedundantMCPKey(target, k, pv) {
			changed = true
			continue
		}
		out[k] = pv
	}
	return out, changed
}

func pruneMCPArray(target string, arr []any, parentKey string) ([]any, bool) {
	changed := false
	limit, truncated, fromEnd := len(arr), 0, false
	if (parentKey == "frames" || parentKey == "stacktrace") && limit > mcpMaxStackFrames {
		truncated, limit, fromEnd, changed = limit-mcpMaxStackFrames, mcpMaxStackFrames, strings.HasPrefix(target, "sentry_"), true
	}
	out := make([]any, 0, limit+1)
	start := 0
	if fromEnd {
		start = len(arr) - limit
		out = append(out, fmt.Sprintf("... (%d more frames)", truncated))
	}
	for i := start; i < start+limit; i++ {
		pv, childChanged := pruneMCPValue(target, arr[i], parentKey)
		changed = changed || childChanged
		if m, ok := pv.(map[string]any); pv == nil || (ok && len(m) == 0) {
			changed = true
			continue
		}
		out = append(out, pv)
	}
	if !fromEnd && truncated > 0 {
		out = append(out, fmt.Sprintf("... (%d more frames)", truncated))
	}
	if collapseRepeatedArrayKeys(out, 2, func(l string) bool { return mcpProtectedKeys[l] }) {
		changed = true
	}
	return out, changed
}

func isRedundantMCPKey(target, key string, val any) bool {
	lower := strings.ToLower(key)
	if mcpProtectedKeys[lower] {
		return false
	}
	switch v := val.(type) {
	case nil:
		return true
	case string:
		if strings.TrimSpace(v) == "" || ((strings.HasPrefix(v, "http://") || strings.HasPrefix(v, "https://")) && len(v) > mcpMaxURLLen) {
			return true
		}
	case []any:
		return len(v) == 0
	case map[string]any:
		return len(v) == 0
	}
	return mcpBoilerplateKeys[lower] || strings.HasSuffix(lower, "_url") || strings.HasSuffix(lower, "url") ||
		(target == "sentry_events" && (lower == "headers" || lower == "cookies" || lower == "env"))
}

func pruneMCPText(input []byte) ([]byte, bool) {
	re := mcpRegexes()
	lines := strings.Split(string(input), "\n")
	out := make([]string, 0, len(lines))
	stackRun, blankRun := 0, 0
	flushStack := func() {
		if stackRun > mcpMaxStackFrames {
			out = append(out, fmt.Sprintf("  ... (%d more frames)", stackRun-mcpMaxStackFrames))
		}
		stackRun = 0
	}
	for _, line := range lines {
		trimmed := strings.TrimRight(line, " \t\r")
		if re.permalink.MatchString(trimmed) || re.emptyField.MatchString(trimmed) {
			continue
		}
		trimmed = re.mdLink.ReplaceAllStringFunc(trimmed, func(m string) string {
			if sub := re.mdLink.FindStringSubmatch(m); len(sub) == 3 && len(sub[2]) > 30 {
				return sub[1]
			}
			return m
		})
		if re.stackFrame.MatchString(trimmed) {
			if stackRun++; stackRun <= mcpMaxStackFrames {
				out = append(out, trimmed)
			}
			continue
		}
		flushStack()
		if strings.TrimSpace(trimmed) == "" {
			if blankRun++; blankRun <= 1 {
				out = append(out, "")
			}
			continue
		}
		blankRun = 0
		out = append(out, trimmed)
	}
	flushStack()
	if res := []byte(strings.TrimSpace(strings.Join(out, "\n"))); len(res) > 0 && len(res) < len(input) {
		return res, true
	}
	return nil, false
}
