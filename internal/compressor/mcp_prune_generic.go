// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"bytes"
	"encoding/json"
	"strings"
)

const (
	mcpGenericMinBytes     = 512
	mcpGenericGainSmallPct = 20 // required savings when 512..1023 bytes
	mcpGenericGainLargePct = 15 // required savings when >= 1024 bytes
	mcpGenericMaxHTMLURL   = 80
	mcpGenericMaxURL       = 100
	mcpGenericMinArraySame = 3
)

var genericExtraProtectedKeys = map[string]bool{
	"name": true, "key": true, "summary": true, "errors": true, "code": true, "uid": true, "slug": true,
}

var genericBoilerplateKeys = map[string]bool{
	"_links": true, "node_id": true, "gravatar_id": true, "avatar_url": true, "avatarurls": true,
}

var genericBoilerplateURLKeys = map[string]bool{
	"followers_url": true, "following_url": true, "gists_url": true, "starred_url": true,
	"subscriptions_url": true, "organizations_url": true, "repos_url": true, "events_url": true,
	"received_events_url": true, "labels_url": true, "comments_url": true, "commits_url": true,
	"git_commits_url": true, "issue_events_url": true, "assignees_url": true, "branches_url": true,
	"tags_url": true, "blobs_url": true, "git_tags_url": true, "git_refs_url": true,
	"trees_url": true, "merges_url": true, "downloads_url": true, "deployments_url": true,
	"statuses_url": true, "hooks_url": true, "teams_url": true, "milestones_url": true,
	"notifications_url": true, "pulls_url": true, "releases_url": true, "stargazers_url": true,
	"subscribers_url": true, "contributors_url": true, "collaborators_url": true, "keys_url": true,
	"compare_url": true, "contents_url": true, "issue_comment_url": true, "issues_url": true,
	"languages_url": true, "archive_url": true, "mirror_url": true, "icon_url": true, "iconurl": true,
}

func pruneGenericMCP(toolName string, input []byte) ([]byte, bool) {
	if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(toolName)), "mcp__") {
		return nil, false
	}
	if len(bytes.TrimSpace(input)) < mcpGenericMinBytes {
		return nil, false
	}
	cur, _ := unwrapMCPEnvelope(input)
	trim := bytes.TrimSpace(cur)
	if len(trim) == 0 {
		return nil, false
	}
	var out []byte
	var ok bool
	switch {
	case (trim[0] == '{' || trim[0] == '[') && json.Valid(trim):
		out, ok = pruneGenericMCPJSON(trim)
	case trim[0] != '{' && trim[0] != '[' && json.Valid(trim):
		return nil, false // never alter JSON primitives
	default:
		out, ok = pruneGenericMCPText(cur)
	}
	if !ok || len(out) == 0 || len(out) >= len(input) {
		return nil, false
	}
	minPct := mcpGenericGainLargePct
	if len(input) < 1024 {
		minPct = mcpGenericGainSmallPct
	}
	if len(out)*100 > len(input)*(100-minPct) {
		return nil, false
	}
	return out, true
}

func pruneGenericMCPJSON(input []byte) ([]byte, bool) {
	val, err := decodeSingleJSON(input)
	if err != nil {
		return nil, false
	}
	if root, ok := val.(map[string]any); ok {
		if isErr, ok := root["isError"].(bool); ok && isErr {
			return nil, false
		}
	} else if _, ok := val.([]any); !ok {
		return nil, false
	}
	pruned, changed := pruneGenericValue(val)
	if !changed {
		return nil, false
	}
	out, err := encodeJSON(pruned, false)
	if err != nil {
		return nil, false
	}
	return bytes.TrimRight(out, "\n"), true
}

func pruneGenericValue(v any) (any, bool) {
	switch x := v.(type) {
	case map[string]any:
		return pruneGenericObject(x)
	case []any:
		return pruneGenericArray(x)
	default:
		return v, false
	}
}

func pruneGenericObject(obj map[string]any) (map[string]any, bool) {
	out := make(map[string]any, len(obj))
	changed := false
	for k, v := range obj {
		lower := strings.ToLower(k)
		if isGenericProtectedKey(lower) {
			if _, isMap := v.(map[string]any); isMap {
				pv, childChanged := pruneGenericValue(v)
				changed = changed || childChanged
				out[k] = pv
			} else if _, isArr := v.([]any); isArr {
				pv, childChanged := pruneGenericValue(v)
				changed = changed || childChanged
				out[k] = pv
			} else {
				out[k] = v
			}
			continue
		}
		if isGenericBoilerplateKey(lower, v) {
			changed = true
			continue
		}
		pv, childChanged := pruneGenericValue(v)
		changed = changed || childChanged
		if isGenericBoilerplateKey(lower, pv) {
			changed = true
			continue
		}
		out[k] = pv
	}
	return out, changed
}

func pruneGenericArray(arr []any) ([]any, bool) {
	changed := false
	out := make([]any, 0, len(arr))
	for _, elem := range arr {
		pv, childChanged := pruneGenericValue(elem)
		changed = changed || childChanged
		if m, ok := pv.(map[string]any); pv == nil || (ok && len(m) == 0) {
			changed = true
			continue
		}
		out = append(out, pv)
	}
	if collapseRepeatedArrayKeys(out, mcpGenericMinArraySame, isGenericProtectedKey) {
		changed = true
	}
	return out, changed
}

func isGenericBoilerplateKey(lower string, val any) bool {
	if genericBoilerplateKeys[lower] {
		return true
	}
	switch v := val.(type) {
	case nil:
		return true
	case string:
		if strings.TrimSpace(v) == "" {
			return true
		}
		if lower == "self" && (strings.HasPrefix(v, "http://") || strings.HasPrefix(v, "https://")) {
			return true
		}
		if lower == "html_url" {
			return len(v) > mcpGenericMaxHTMLURL
		}
		if lower == "url" || strings.HasSuffix(lower, "_url") {
			return len(v) > mcpGenericMaxURL ||
				(strings.HasSuffix(lower, "_url") && (genericBoilerplateURLKeys[lower] || strings.Contains(v, "{/")))
		}
	case []any:
		return len(v) == 0
	case map[string]any:
		return len(v) == 0
	}
	return false
}

func isGenericProtectedKey(lower string) bool {
	return mcpProtectedKeys[lower] || genericExtraProtectedKeys[lower]
}

func collapseRepeatedArrayKeys(out []any, minLen int, isProtected func(string) bool) bool {
	if len(out) < minLen {
		return false
	}
	first, ok := out[0].(map[string]any)
	if !ok {
		return false
	}
	changed := false
	for k, fv := range first {
		if isProtected(strings.ToLower(k)) {
			continue
		}
		if s, ok := fv.(string); ok && reKeepLine.MatchString(s) {
			continue
		}
		fb, err := json.Marshal(fv)
		if err != nil {
			continue
		}
		sameAll := true
		for i := 1; i < len(out) && sameAll; i++ {
			m, ok := out[i].(map[string]any)
			mb, err := json.Marshal(m[k])
			sameAll = ok && m[k] != nil && err == nil && bytes.Equal(fb, mb)
		}
		for i := 1; sameAll && i < len(out); i++ {
			delete(out[i].(map[string]any), k)
			changed = true
		}
	}
	return changed
}

func pruneGenericMCPText(input []byte) ([]byte, bool) {
	re := mcpRegexes()
	lines := strings.Split(string(input), "\n")
	out := make([]string, 0, len(lines))
	blankRun := 0
	for _, line := range lines {
		trimmed := strings.TrimRight(line, " \t\r")
		if reKeepLine.MatchString(trimmed) {
			blankRun = 0
			out = append(out, trimmed)
			continue
		}
		k, _, _ := strings.Cut(trimmed, ":")
		k = strings.ToLower(strings.Trim(k, " \t-*\""))
		if re.emptyField.MatchString(trimmed) && !isGenericProtectedKey(k) {
			continue
		}
		if re.permalink.MatchString(trimmed) && (k != "url" || len(trimmed) > mcpGenericMaxURL) {
			continue
		}
		trimmed = re.mdLink.ReplaceAllStringFunc(trimmed, func(m string) string {
			if sub := re.mdLink.FindStringSubmatch(m); len(sub) == 3 && len(sub[2]) > mcpGenericMaxHTMLURL {
				return sub[1]
			}
			return m
		})
		if strings.TrimSpace(trimmed) == "" {
			if blankRun++; blankRun <= 1 {
				out = append(out, "")
			}
			continue
		}
		blankRun = 0
		out = append(out, trimmed)
	}
	res := []byte(strings.TrimSpace(strings.Join(out, "\n")))
	if len(res) == 0 || len(res) >= len(input) {
		return nil, false
	}
	return res, true
}
