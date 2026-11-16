// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package paths

import "os"

// EventsPath is events.jsonl for routing telemetry.
func EventsPath() (string, error) { return Join("events.jsonl") }

// AgentsPath is agents.jsonl for spawn-side logging.
func AgentsPath() (string, error) { return Join("agents.jsonl") }

// CatalogPath is the user catalog override file.
func CatalogPath() (string, error) { return Join("catalog.json") }

// GatewayAllowlistPath is the user allowlist of gateway models that may
// be routing targets.
func GatewayAllowlistPath() (string, error) { return Join("openrouter.json") }

// SessionModelsPath is session-models.json unless DOWNSHIFT_SESSION_MODELS is set.
func SessionModelsPath() (string, error) {
	if p := os.Getenv("DOWNSHIFT_SESSION_MODELS"); p != "" {
		return p, nil
	}
	return Join("session-models.json")
}

// DisplaySessionModels returns the path shown in hook warnings.
func DisplaySessionModels() string {
	if p := os.Getenv("DOWNSHIFT_SESSION_MODELS"); p != "" {
		return p
	}
	return DisplayStateDir() + "/session-models.json"
}
