#!/usr/bin/env bash
export DOWNSHIFT_CONTROL_SHARE=0  # the control group would leave ~10% of spawns unrouted
# Adapter smoke: pipe example PreToolUse payloads through downshift (no live harness).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

# Keep fixture decisions out of the operator's real stats. DOWNSHIFT_STATE_DIR
# moves every state file (exec-events.jsonl included); DOWNSHIFT_EVENT_LOG
# moves the routing log, which is the one the assertions read.
SMOKE_DIR="$(mktemp -d)"
trap 'rm -rf "$SMOKE_DIR"' EXIT
export DOWNSHIFT_STATE_DIR="$SMOKE_DIR/state"
export DOWNSHIFT_EVENT_LOG="$SMOKE_DIR/events.jsonl"

if ! command -v downshift >/dev/null 2>&1; then
  echo "downshift not in PATH — install first (see docs/guides/install.md)" >&2
  exit 1
fi

# count_rows <harness> prints how many event rows carry that harness value.
count_rows() {
  if [ -f "$DOWNSHIFT_EVENT_LOG" ]; then
    grep -c "\"harness\":\"$1\"" "$DOWNSHIFT_EVENT_LOG" || true
  else
    echo 0
  fi
}

# run_fixture <label> <harness> <payload>: runs the fixture and fails unless
# the event log gained at least one row written for that harness.
run_fixture() {
  label="$1"
  harness="$2"
  payload="$3"
  echo "== $label =="
  before="$(count_rows "$harness")"
  downshift "$harness" < "$payload" >/dev/null
  after="$(count_rows "$harness")"
  if [ "$after" -le "$before" ]; then
    echo "FAIL: $label wrote no event row with harness=\"$harness\" to $DOWNSHIFT_EVENT_LOG" >&2
    exit 1
  fi
}

run_fixture "claude-code (task fixture)" claude-code examples/claude-code-pretooluse-task.json
run_fixture "cursor (task fixture)" cursor examples/cursor-pretooluse-task.json
run_fixture "claude-code (session_models rewrite)" claude-code examples/claude-code-pretooluse-session.json

echo "OK: all adapter smoke checks passed"
