// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import "bytes"

// CondenseShellJSON prunes JSON printed by a shell command (gh api, curl, a
// python one-liner) with the same rules as MCP JSON output: noisy metadata
// keys and URLs go, long arrays shrink to a summary element. It is lossy: the
// caller runs it only once the original is stored. Text that is not exactly one
// JSON value, or JSON that does not shrink, comes back unchanged with false.
func CondenseShellJSON(data []byte) ([]byte, bool) {
	t := bytes.TrimSpace(data)
	if len(t) < JSONPruneMinBytes || (t[0] != '{' && t[0] != '[') {
		return data, false
	}
	out, ok, _ := compressJSONPrune(t)
	if !ok || float64(len(data)-len(out)) < rtkMinGain*float64(len(data)) {
		return data, false
	}
	return out, true
}
