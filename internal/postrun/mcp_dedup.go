// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package postrun

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/hookport"
	"github.com/tiagovilasboas/downshift-ai/internal/paths"
)

const (
	mcpDedupSubdir   = "mcp-dedup"
	mcpDedupTTL      = time.Hour
	mcpDedupMinBytes = 512
)

// mcpDocAllowlist lists normalized MCP action names that return static docs,
// skills, or schemas. Live/query/search/execution/mutation tools (search_*,
// query_*, get_*, execute_code, etc.) must NEVER appear here.
var mcpDocAllowlist = map[string]bool{
	"load_datadog_skill":          true,
	"list_datadog_skills":         true,
	"resolve_library_id":          true,
	"fetch_library_documentation": true,
	"library_docs":                true,
	"list_docs":                   true,
	"read_doc":                    true,
	"read_docs":                   true,
	"load_doc":                    true,
	"load_docs":                   true,
	"fetch_doc":                   true,
	"fetch_docs":                  true,
	"list_skills":                 true,
	"load_skill":                  true,
	"read_skill":                  true,
	"list_schemas":                true,
	"describe_schema":             true,
	"introspect_schema":           true,
	"read_schema":                 true,
	"openapi_schema":              true,
	"graphql_schema":              true,
	"list_tools":                  true,
	"describe_tool":               true,
	"list_resources":              true,
	"read_resource":               true,
}

// dedupMCPDoc deduplicates idempotent static MCP doc/skill/schema tool outputs
// within the same session and TTL. On hit, it returns a short pointer with a
// `downshift retrieve` hint without re-writing CCR. On miss (>= mcpDedupMinBytes),
// it stores the raw output in CCR and writes a snapshot.
func dedupMCPDoc(in hookport.ToolOutput, o Options) (out, id string, changed bool) {
	defer func() {
		if recover() != nil {
			out, id, changed = in.Content, "", false
		}
	}()
	if in.VerboseKind != VerboseKindMCP || o.Keep == nil || !isMCPDocTool(in.ToolName) {
		return in.Content, "", false
	}
	sess := seenSession(o, in.Session)
	if sess == "" {
		return in.Content, "", false
	}
	dir, err := paths.Join(mcpDedupSubdir)
	if err != nil {
		return in.Content, "", false
	}
	return dedupMCPDocAt(in, o, sess, dir, time.Now())
}

func dedupMCPDocAt(in hookport.ToolOutput, o Options, sess, dir string, now time.Time) (string, string, bool) {
	keyPath := mcpDedupKeyPath(dir, sess, in.ToolName, in.ToolInput)
	if prev, hit := readDedupSnapshot(keyPath, mcpDedupTTL, now); hit && prev.ID != "" {
		ptr := mcpDocPointer(in.ToolName, countLines(in.Content), len(in.Content), readRetrieveArg(sess, prev.ID))
		if len(ptr) < len(in.Content) {
			return ptr, prev.ID, true
		}
	}
	if len(in.Content) < mcpDedupMinBytes {
		return in.Content, "", false
	}
	id, ok := o.Keep([]byte(in.Content))
	if !ok || id == "" {
		return in.Content, "", false
	}
	writeDedupSnapshot(keyPath, readSnapshot{Size: int64(len(in.Content)), At: now.UnixNano(), ID: id}, now)
	pruneDedupDir(dir, keyPath, mcpDedupTTL, now)
	return in.Content, "", false
}

func mcpDedupKeyPath(dir, sessionID, toolName string, toolInput json.RawMessage) string {
	sessSum := sha256.Sum256([]byte("downshift-read-dedup-session-v1:" + sessionID))
	sessHash := hex.EncodeToString(sessSum[:])
	scope := "downshift-mcp-dedup-v1:" + sessHash + "\x00" + toolName + "\x00" + canonicalMCPArgs(toolInput)
	keySum := sha256.Sum256([]byte(scope))
	return filepath.Join(dir, hex.EncodeToString(keySum[:])[:24]+".json")
}

func isMCPDocTool(toolName string) bool {
	action := normalizeMCPAction(toolName)
	if action == "" {
		return false
	}
	if strings.HasPrefix(action, "search_") ||
		strings.HasPrefix(action, "query_") ||
		strings.HasPrefix(action, "get_") ||
		action == "execute_code" {
		return false
	}
	return mcpDocAllowlist[action]
}

func normalizeMCPAction(toolName string) string {
	s := strings.ToLower(strings.TrimSpace(toolName))
	if i := strings.LastIndex(s, "__"); i >= 0 {
		return s[i+2:]
	}
	if i := strings.LastIndexAny(s, ":/."); i >= 0 {
		return s[i+1:]
	}
	return s
}

func canonicalMCPArgs(toolInput json.RawMessage) string {
	trimmed := bytes.TrimSpace(toolInput)
	if len(trimmed) == 0 {
		return ""
	}
	var v any
	if err := json.Unmarshal(trimmed, &v); err == nil {
		if b, err := json.Marshal(v); err == nil {
			return string(b)
		}
	}
	return string(trimmed)
}

func mcpDocPointer(toolName string, lines, size int, arg string) string {
	action := normalizeMCPAction(toolName)
	if action == "" {
		action = "mcp"
	}
	return fmt.Sprintf("[downshift: unchanged %s (%d lines, %d B) already loaded in this session; downshift retrieve %s]\n",
		action, lines, size, arg)
}
