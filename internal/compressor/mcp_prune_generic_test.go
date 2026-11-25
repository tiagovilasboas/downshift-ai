// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func buildAtlassianJiraFixture() string {
	issues := make([]map[string]any, 4)
	for i := range issues {
		issues[i] = map[string]any{
			"id":     fmt.Sprintf("1000%d", i),
			"key":    fmt.Sprintf("ENG-%d", 100+i),
			"self":   fmt.Sprintf("https://acme.atlassian.net/rest/api/3/issue/1000%d", i),
			"expand": "renderedFields,names,schema,operations,editmeta,changelog,versionedRepresentations",
			"fields": map[string]any{
				"summary":     fmt.Sprintf("Fix webhook retry %d", i),
				"status":      map[string]any{"name": "In Progress", "statusCategory": map[string]any{"key": "indeterminate"}},
				"description": "",
				"labels":      []any{},
				"resolution":  nil,
				"customfield": map[string]any{},
				"project":     map[string]any{"id": "100", "key": "ENG", "name": "Engineering", "avatarUrls": map[string]any{"48x48": "https://acme.atlassian.net/avatar/48.png", "24x24": "https://acme.atlassian.net/avatar/24.png"}},
			},
		}
	}
	b, _ := json.Marshal(map[string]any{"issues": issues, "total": 4})
	return string(b)
}

func buildGitHubIssuesFixture() string {
	items := make([]map[string]any, 3)
	for i := range items {
		items[i] = map[string]any{
			"id":             500 + i,
			"node_id":        fmt.Sprintf("I_kwDOAbc%03d", i),
			"gravatar_id":    "",
			"avatar_url":     "https://avatars.githubusercontent.com/u/1?v=4",
			"html_url":       fmt.Sprintf("https://github.com/org/repo/issues/%d", i+1), // <= 80 chars, kept
			"url":            fmt.Sprintf("https://api.github.com/repos/org/very-long-organization-name/very-long-repository-name-here/issues/%d/extra", i+1),
			"followers_url":  "https://api.github.com/users/dev/followers",
			"following_url":  "https://api.github.com/users/dev/following{/other_user}",
			"labels_url":     fmt.Sprintf("https://api.github.com/repos/org/repo/issues/%d/labels{/name}", i+1),
			"_links":         map[string]any{"self": map[string]any{"href": "https://api.github.com/x"}},
			"title":          fmt.Sprintf("Bug %d: race on close", i+1),
			"status":         "open",
			"milestone":      nil,
			"assignees":      []any{},
			"repository_org": "acme-platform",
		}
	}
	b, _ := json.Marshal(items)
	return string(b)
}

func buildGrafanaDashboardsFixture() string {
	rows := make([]map[string]any, 4)
	for i := range rows {
		rows[i] = map[string]any{
			"id":          i + 1,
			"uid":         fmt.Sprintf("dash-%03d", i),
			"title":       fmt.Sprintf("Payments Latency %d", i),
			"type":        "dash-db",
			"url":         fmt.Sprintf("/d/dash-%03d", i), // short <= 100 chars, kept
			"folder_url":  fmt.Sprintf("https://grafana.internal.example.com/dashboards/f/folder-payments-team-observability-prod/sub-%d?orgId=1", i),
			"folderTitle": "Payments Observability",
			"orgName":     "Production",
			"tags":        []any{},
			"meta":        map[string]any{},
			"isStarred":   false,
			"description": nil,
		}
	}
	b, _ := json.Marshal(map[string]any{"dashboards": rows})
	return string(b)
}

func buildChromeNetworkFixture() string {
	reqs := make([]map[string]any, 4)
	for i := range reqs {
		reqs[i] = map[string]any{
			"id":           fmt.Sprintf("req-%d", i),
			"status":       200,
			"type":         "xhr",
			"url":          fmt.Sprintf("https://app.example.com/api/v2/checkout/orders/%d/confirm?include=items,discounts,taxes,shipping,metadata", i),
			"html_url":     fmt.Sprintf("https://trace.example.com/sessions/session-abcdef-123456-789012/requests/request-detail-view-%d", i),
			"error":        nil,
			"timings":      map[string]any{},
			"redirects":    []any{},
			"resourceType": "Fetch",
			"protocol":     "h2",
			"initiator":    map[string]any{"type": "script", "stack": "app.bundle.js"},
		}
	}
	b, _ := json.Marshal(map[string]any{
		"content": []map[string]any{{"type": "text", "text": func() string { s, _ := json.Marshal(reqs); return string(s) }()}},
	})
	return string(b)
}

func TestPruneMCP_GenericPayloads(t *testing.T) {
	cases := []struct {
		name     string
		toolName string
		input    string
		preserve []string
		absent   []string
	}{
		{
			name:     "atlassian jira search",
			toolName: "mcp__atlassian__jira_search",
			input:    buildAtlassianJiraFixture(),
			preserve: []string{"10000", "ENG-100", "ENG-103", "Fix webhook retry 0", "In Progress", "Engineering"},
			absent:   []string{"avatarUrls", `"self":`, `"description":""`, `"labels":[]`},
		},
		{
			name:     "github list issues",
			toolName: "mcp__github__list_issues",
			input:    buildGitHubIssuesFixture(),
			preserve: []string{"Bug 1: race on close", "Bug 3: race on close", "https://github.com/org/repo/issues/1", "acme-platform"},
			absent:   []string{"node_id", "gravatar_id", "avatar_url", "followers_url", "labels_url", "_links", "very-long-organization-name"},
		},
		{
			name:     "grafana search dashboards",
			toolName: "mcp__grafana__search_dashboards",
			input:    buildGrafanaDashboardsFixture(),
			preserve: []string{"dash-000", "dash-003", "Payments Latency 0", "/d/dash-000", "Payments Observability"},
			absent:   []string{"folder-payments-team-observability-prod", `"tags":[]`, `"meta":{}`},
		},
		{
			name:     "chrome devtools network envelope",
			toolName: "mcp__chrome-devtools__list_network_requests",
			input:    buildChromeNetworkFixture(),
			preserve: []string{"req-0", "req-3", `"status":200`, `"error":null`, "Fetch", "h2"},
			absent:   []string{"include=items,discounts", "request-detail-view", `"timings":{}`},
		},
		{
			name:     "generic markdown with reKeepLine and protected empty fields",
			toolName: "mcp__confluence__get_page",
			input: strings.Join([]string{
				"# Runbook: Payment Gateway",
				"- **status**: \"\"",
				"- **error**: null",
				"- **Assignee**: None",
				"- **Reviewer**: N/A",
				"- **Permalink**: https://confluence.internal.example.com/display/PAY/Gateway+Runbook+Very+Long+Page+Title",
				"- **Web_url**: https://confluence.internal.example.com/pages/viewpage.action?pageId=12345678901234567890",
				"- See [Architecture Spec](https://confluence.internal.example.com/spaces/ENG/pages/998877665544/Architecture+Spec+Detailed+Notes)",
				"",
				"",
				"",
				"WARNING: retry queue depth exceeded 500",
				"error: upstream timeout on /v1/charge",
				"- **Metadata**: {}",
				"- **Tags**: []",
				"- **Share_url**: https://confluence.internal.example.com/x/AbCdEfGhIjKlMnOpQrStUvWxYz",
				"- **Notes**: None",
				"- **Owner**: null",
				"- **Extra**: \"\"",
				"- **Links**: []",
				"- **Archive**: {}",
				"- **URL**: https://confluence.internal.example.com/spaces/ENG/pages/998877665544/Very+Long+URL+Path+Beyond+100+Chars",
			}, "\n"),
			preserve: []string{
				"# Runbook: Payment Gateway",
				`- **status**: ""`,
				`- **error**: null`,
				"Architecture Spec",
				"WARNING: retry queue depth exceeded 500",
				"error: upstream timeout on /v1/charge",
			},
			absent: []string{
				"Assignee", "Reviewer", "Permalink", "Web_url", "Share_url",
				"Architecture+Spec+Detailed+Notes", "Metadata", "Tags", "Extra",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, ok := PruneMCP(tc.toolName, []byte(tc.input))
			if !ok {
				t.Fatalf("PruneMCP(%q) returned ok=false (len=%d)", tc.toolName, len(tc.input))
			}
			if len(out)*100 > len(tc.input)*85 {
				t.Fatalf("expected >=15%% reduction: %d -> %d", len(tc.input), len(out))
			}
			got := string(out)
			for _, want := range tc.preserve {
				if !strings.Contains(got, want) {
					t.Errorf("missing preserved token %q in:\n%s", want, got)
				}
			}
			for _, drop := range tc.absent {
				if strings.Contains(got, drop) {
					t.Errorf("expected %q to be stripped in:\n%s", drop, got)
				}
			}
		})
	}
}

func TestPruneMCP_GenericGuards(t *testing.T) {
	longPad := strings.Repeat("useful data ", 90)
	cases := []struct {
		name     string
		toolName string
		input    string
	}{
		{
			name:     "small payload under 512 bytes",
			toolName: "mcp__github__list_issues",
			input:    `{"id":1,"node_id":"abc","gravatar_id":"","_links":{"self":"x"},"title":"small"}`,
		},
		{
			name:     "non mcp tool name",
			toolName: "bash",
			input:    buildGitHubIssuesFixture(),
		},
		{
			name:     "json primitive string",
			toolName: "mcp__custom__echo",
			input:    fmt.Sprintf("%q", longPad),
		},
		{
			name:     "json primitive number",
			toolName: "mcp__custom__count",
			input:    strings.Repeat("1", 600),
		},
		{
			name:     "isError true envelope preserved",
			toolName: "mcp__github__get_issue",
			input:    fmt.Sprintf(`{"isError":true,"node_id":"abc","gravatar_id":"","_links":{},"error":%q}`, longPad),
		},
		{
			name:     "insufficient savings under 15 percent",
			toolName: "mcp__github__get_issue",
			input:    fmt.Sprintf(`{"id":1,"title":"keep","body":%q,"node_id":"123"}`, longPad),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if out, ok := PruneMCP(tc.toolName, []byte(tc.input)); ok {
				t.Errorf("expected ok=false, got ok=true (%d -> %d):\n%s", len(tc.input), len(out), out)
			}
		})
	}
}
