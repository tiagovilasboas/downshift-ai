// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package claudecode

import (
	"github.com/tiagovilasboas/downshift-ai/internal/contextopt"
	"github.com/tiagovilasboas/downshift-ai/internal/sensor"
)

func recordSensorToolObservation(payload map[string]any, sessionHash string) {
	if sessionHash == "" || !nativeContextObserveOn() {
		return
	}
	st, err := sensor.DefaultStore()
	if err != nil {
		return
	}
	toolName := stringFieldAny(payload, "tool_name", "toolName")
	if toolName == "" {
		toolName = "tool"
	}

	var outputBytes []byte
	isError := false
	exitCode := -1
	if resp, ok := payload["tool_response"].(map[string]any); ok {
		outputBytes = toolOutputBytes(resp)
		if status, ok := resp["status"].(string); ok && (status == "error" || status == "failed") {
			isError = true
		}
		if code, ok := exitCodeField(resp); ok {
			exitCode = code
		}
	}
	_ = st.RecordToolResult(sessionHash, harnessID, toolName, outputBytes, exitCode, isError)
}

// toolOutputBytes is the text the model reads from one tool result, in the
// shapes Claude Code sends: Bash {stdout, stderr}, a plain content string, an
// array of content blocks ({type:"text", text}), or a generic output string.
// Only the length and the observe-mode compressor ever see it; nothing here is
// stored. Unknown shapes give nil, which records zero bytes as before.
func toolOutputBytes(resp map[string]any) []byte {
	var b []byte
	if out, ok := resp["stdout"].(string); ok {
		b = append(b, out...)
	}
	if errOut, ok := resp["stderr"].(string); ok && errOut != "" {
		if len(b) > 0 && b[len(b)-1] != '\n' {
			b = append(b, '\n')
		}
		b = append(b, errOut...)
	}
	if b != nil {
		return b
	}
	switch c := resp["content"].(type) {
	case string:
		return []byte(c)
	case []any:
		for _, block := range c {
			if m, ok := block.(map[string]any); ok {
				if text, ok := m["text"].(string); ok {
					b = append(b, text...)
				}
			}
		}
		return b
	}
	if out, ok := resp["output"].(string); ok {
		return []byte(out)
	}
	return nil
}

// nativeContextObserveOn is the persisted native-provider flag. Missing
// config uses the default (enabled, provider native). A read error fails
// closed. The stored record is observe-mode only; this hook does not
// replace the tool result the model sees.
func nativeContextObserveOn() bool {
	cfg, err := contextopt.LoadConfig()
	if err != nil || !cfg.Enabled {
		return false
	}
	return cfg.Provider == "" || cfg.Provider == "native"
}

func exitCodeField(resp map[string]any) (int, bool) {
	for _, key := range []string{"exit_code", "exitCode"} {
		n, ok := resp[key].(float64)
		if !ok || n < 0 || n > 255 || n != float64(int(n)) {
			continue
		}
		return int(n), true
	}
	return -1, false
}
