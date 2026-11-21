// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

// SpawnOrigin says who put a spawn on the model it ran on, measured against
// the session model. It is the one placement rule shared by the token view
// (routing_tokens.go) and any outcome view that groups spawns by origin.
type SpawnOrigin string

const (
	// OriginDownshiftRewrite: ran below the session model through an applied
	// Downshift rewrite the harness did not refute.
	OriginDownshiftRewrite SpawnOrigin = "downshift_rewrite"
	// OriginOrchestratorBelow: ran below the session model because the caller
	// or the session already chose it (no applied, unrefuted rewrite).
	OriginOrchestratorBelow SpawnOrigin = "orchestrator_below"
	// OriginOrchestratorSession: ran on the session model (or above it).
	OriginOrchestratorSession SpawnOrigin = "orchestrator_session"
	// OriginUnattributed: no usable session model to place the spawn against.
	OriginUnattributed SpawnOrigin = "unattributed"
)

// OriginOf derives the origin from the facts the routing view collects per
// spawn: onSession (some usage record ranked against the session model),
// belowSession (some record ran below it) and rewritten (the decision was an
// applied model rewrite the harness did not refute). A rewrite that is refuted
// or that did not end below the session model is not counted as one.
func OriginOf(onSession, belowSession, rewritten bool) SpawnOrigin {
	switch {
	case !onSession:
		return OriginUnattributed
	case !belowSession:
		return OriginOrchestratorSession
	case rewritten:
		return OriginDownshiftRewrite
	default:
		return OriginOrchestratorBelow
	}
}
