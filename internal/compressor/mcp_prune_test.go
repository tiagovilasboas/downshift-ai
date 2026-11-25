// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"strings"
	"testing"
)

const sentrySearchIssuesFixture = `{
  "result": "{\"issues\":[{\"id\":\"9812345\",\"shortId\":\"PAY-412\",\"title\":\"NullPointerException in ChargeService\",\"culprit\":\"com.example.pay.ChargeService.capture\",\"level\":\"error\",\"status\":\"unresolved\",\"count\":\"148\",\"userCount\":42,\"firstSeen\":\"2026-11-20T09:15:00Z\",\"lastSeen\":\"2026-11-25T08:40:00Z\",\"permalink\":\"https://sentry.io/organizations/acme-corp/issues/9812345/?referrer=mcp\",\"assignedTo\":null,\"annotations\":[],\"activity\":[],\"isPublic\":false,\"isBookmarked\":false,\"organization\":\"acme-corp\",\"platform\":\"java\"},{\"id\":\"9812399\",\"shortId\":\"PAY-419\",\"title\":\"TimeoutException: gateway connect timed out\",\"culprit\":\"com.example.pay.GatewayClient.post\",\"level\":\"warning\",\"status\":\"unresolved\",\"count\":\"19\",\"userCount\":7,\"firstSeen\":\"2026-11-23T14:00:00Z\",\"lastSeen\":\"2026-11-25T08:45:00Z\",\"permalink\":\"https://sentry.io/organizations/acme-corp/issues/9812399/?referrer=mcp\",\"assignedTo\":null,\"annotations\":[],\"activity\":[],\"isPublic\":false,\"isBookmarked\":false,\"organization\":\"acme-corp\",\"platform\":\"java\"}]}"
}`

const sentrySearchEventsFixture = `{
  "events": [
    {
      "eventID": "ev_01abcdef99887766",
      "title": "TypeError: Cannot read properties of undefined (reading 'amount')",
      "message": "TypeError: Cannot read properties of undefined (reading 'amount')",
      "culprit": "src/checkout/confirm.ts in confirmOrder",
      "level": "error",
      "status": "unresolved",
      "timestamp": "2026-11-25T08:12:33Z",
      "trace_id": "4bf92f3577b34da6a3ce929d0e0e4736",
      "span_id": "00f067aa0ba902b7",
      "permalink": "https://sentry.io/organizations/acme-corp/issues/9812345/events/ev_01abcdef99887766/?referrer=mcp",
      "sdk": {"name": "sentry.javascript.node", "version": "8.12.0"},
      "breadcrumbs": [],
      "headers": {"user-agent": "Mozilla/5.0", "accept": "application/json"},
      "cookies": {},
      "frames": [
        {"filename": "node:internal/process/task_queues", "function": "processTicksAndRejections", "lineno": 95, "vars": {}},
        {"filename": "node_modules/express/lib/router/layer.js", "function": "handle", "lineno": 95, "vars": {}},
        {"filename": "node_modules/express/lib/router/index.js", "function": "next", "lineno": 284, "vars": {}},
        {"filename": "src/middleware/auth.ts", "function": "verifySession", "lineno": 44, "vars": {}},
        {"filename": "src/routes/orders.ts", "function": "postConfirm", "lineno": 88, "vars": {}},
        {"filename": "src/services/cart.ts", "function": "loadCart", "lineno": 51, "vars": {}},
        {"filename": "src/checkout/confirm.ts", "function": "confirmOrder", "lineno": 127, "vars": {}}
      ],
      "organization": "acme-corp",
      "platform": "node"
    },
    {
      "eventID": "ev_02fedcba11223344",
      "title": "TypeError: Cannot read properties of undefined (reading 'amount')",
      "message": "TypeError: Cannot read properties of undefined (reading 'amount')",
      "culprit": "src/checkout/confirm.ts in confirmOrder",
      "level": "error",
      "status": "unresolved",
      "timestamp": "2026-11-25T08:10:11Z",
      "trace_id": "9a1c8e2100b34da6a3ce929d0e0e9911",
      "span_id": "11a067aa0ba902c8",
      "permalink": "https://sentry.io/organizations/acme-corp/issues/9812345/events/ev_02fedcba11223344/?referrer=mcp",
      "sdk": {"name": "sentry.javascript.node", "version": "8.12.0"},
      "breadcrumbs": [],
      "headers": {"user-agent": "Mozilla/5.0", "accept": "application/json"},
      "cookies": {},
      "organization": "acme-corp",
      "platform": "node"
    }
  ]
}`

const datadogExecuteCodeFixture = `{
  "stdout": "{\n  \"status\": \"error\",\n  \"level\": \"error\",\n  \"timestamp\": \"2026-11-25T08:30:05Z\",\n  \"trace_id\": \"dd-trace-778899001122\",\n  \"span_id\": \"dd-span-33445566\",\n  \"error\": \"Upstream 502 from payment-gateway\",\n  \"permalink\": \"https://app.datadoghq.com/logs?query=service%3Apayment-api&event=AgAAAZN123456\",\n  \"logs\": [\n    {\"id\": \"log_001\", \"title\": \"gateway_timeout\", \"status\": \"error\", \"level\": \"error\", \"timestamp\": \"2026-11-25T08:30:01Z\", \"message\": \"context deadline exceeded\", \"service\": \"payment-api\", \"env\": \"prod\", \"url\": \"https://app.datadoghq.com/logs?event=log_001&index=main\", \"tags\": []},\n    {\"id\": \"log_002\", \"title\": \"gateway_retry\", \"status\": \"warn\", \"level\": \"warning\", \"timestamp\": \"2026-11-25T08:30:03Z\", \"message\": \"retrying charge request\", \"service\": \"payment-api\", \"env\": \"prod\", \"url\": \"https://app.datadoghq.com/logs?event=log_002&index=main\", \"tags\": []}\n  ]\n}",
  "stderr": "",
  "execution_time_ms": 142
}`

func TestPruneMCP_FixturesAndPreservation(t *testing.T) {
	cases := []struct {
		name     string
		toolName string
		input    string
		preserve []string
	}{
		{
			name:     "sentry search_issues",
			toolName: "mcp__sentry__search_issues",
			input:    sentrySearchIssuesFixture,
			preserve: []string{
				"9812345", "PAY-412", "NullPointerException in ChargeService",
				"com.example.pay.ChargeService.capture", "error", "unresolved",
				"148", "42", "2026-11-20T09:15:00Z", "2026-11-25T08:40:00Z",
				"9812399", "PAY-419", "TimeoutException: gateway connect timed out",
			},
		},
		{
			name:     "sentry search_events",
			toolName: "mcp__sentry__search_events",
			input:    sentrySearchEventsFixture,
			preserve: []string{
				"ev_01abcdef99887766", "TypeError: Cannot read properties of undefined (reading 'amount')",
				"src/checkout/confirm.ts in confirmOrder", "error", "unresolved",
				"2026-11-25T08:12:33Z", "4bf92f3577b34da6a3ce929d0e0e4736", "00f067aa0ba902b7",
				"ev_02fedcba11223344", "9a1c8e2100b34da6a3ce929d0e0e9911", "11a067aa0ba902c8",
			},
		},
		{
			name:     "datadog execute_code",
			toolName: "mcp__datadog__execute_code",
			input:    datadogExecuteCodeFixture,
			preserve: []string{
				"error", "2026-11-25T08:30:05Z", "dd-trace-778899001122", "dd-span-33445566",
				"Upstream 502 from payment-gateway", "log_001", "gateway_timeout",
				"context deadline exceeded", "log_002", "gateway_retry", "retrying charge request",
			},
		},
	}

	var totalIn, totalOut int
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, ok := PruneMCP(tc.toolName, []byte(tc.input))
			if !ok {
				t.Fatalf("PruneMCP(%q) returned ok=false", tc.toolName)
			}
			totalIn += len(tc.input)
			totalOut += len(out)
			t.Logf("%s: %d -> %d bytes (%.1f%% reduction)", tc.name, len(tc.input), len(out), float64(len(tc.input)-len(out))*100/float64(len(tc.input)))
			if len(out) >= len(tc.input) {
				t.Fatalf("expected reduction, got %d >= %d", len(out), len(tc.input))
			}
			got := string(out)
			for _, want := range tc.preserve {
				if !strings.Contains(got, want) {
					t.Errorf("missing preserved token %q in:\n%s", want, got)
				}
			}
		})
	}

	avgReduction := float64(totalIn-totalOut) * 100 / float64(totalIn)
	t.Logf("Average reduction across 3 fixtures: %.1f%% (%d -> %d bytes)", avgReduction, totalIn, totalOut)
	if avgReduction < 30.0 {
		t.Fatalf("average reduction %.1f%% < 30%%", avgReduction)
	}
}

func TestPruneMCP_MarkdownAndNonTarget(t *testing.T) {
	md := `# Issues in **acme/web**

## [WEB-101](https://sentry.io/organizations/acme/issues/555111/?referrer=mcp) — ReferenceError: foo is not defined
- **ID**: 555111 (WEB-101)
- **Status**: unresolved | **Level**: error
- **Events**: 12 | **Users**: 4
- **Culprit**: src/app.ts in initApp
- **Assigned**: None
- **Permalink**: https://sentry.io/organizations/acme/issues/555111/?referrer=mcp
`
	out, ok := PruneMCP("mcp__sentry__search_issues", []byte(md))
	if !ok {
		t.Fatal("expected Markdown Sentry output to be pruned")
	}
	got := string(out)
	for _, want := range []string{"555111", "WEB-101", "ReferenceError: foo is not defined", "unresolved", "error", "src/app.ts in initApp"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "https://sentry.io") || strings.Contains(got, "Assigned") {
		t.Errorf("expected permalink/empty field stripped:\n%s", got)
	}

	if _, ok := PruneMCP("mcp__github__list_issues", []byte(md)); ok {
		t.Error("expected non-Sentry/Datadog tool to return ok=false")
	}
}
