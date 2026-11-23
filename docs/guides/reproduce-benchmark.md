<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Reproducing the Downshift Benchmarks

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift-ai

Every performance, signal-retention, and cache-safety claim in Downshift is backed by a runnable command you can audit locally.

## 1. One-Command Offline Proof Benchmark (0 API cost, ~5 seconds)

Replays real repository outputs (`internal/compressor/testdata/balanced/`), simulated failing test runs, subagent spawns, and `.downshift/policy.json` guardrails directly against your compiled `downshift` binary in an isolated temporary `DOWNSHIFT_STATE_DIR` (nothing touches `~/.downshift`):

```bash
make bench-proof
```

What it verifies end-to-end:

1. **Compaction + 100% lossless CCR retrieval:** Replays large `git log -p`, `git show`, `go test -v`, and `grep -rn` outputs through `downshift claude-code-post-tool-use`, verifies byte/token cuts, runs `downshift retrieve` on every CCR id, and asserts byte-for-byte identity plus `retrieved_pct` in `downshift report --json`.
2. **Debug signal preservation + active anti-loop bypass:** Confirms failing `go test` assertions and panic stack traces survive verbatim, and proves that on a 3rd consecutive identical failing `fallback_cut` command (`make test`), Downshift automatically bypasses lossy compaction (`passthrough_detail: anti_loop_bypass`) so the model sees 100% raw output.
3. **Prompt-cache safety invariance:** Verifies main-session tool calls (`Read`) stay completely untouched and subagent `Task` rewrites mutate only `model`, preserving `prompt`, `description`, and sibling fields verbatim.
4. **User-owned policy floor:** Confirms `min_tier_by_risk: {"authz": "frontier"}` keeps security tasks on frontier while routine searches downshift.

Add `--json` for machine output or `--write path.md` to export the Markdown summary table:

```bash
DOWNSHIFT_BIN=./downshift python3 -I scripts/bench-proof/run.py --json
```

## 2. Corpus Compaction Benchmark (`make bench`)

Regenerates [docs/reference/bench.md](../reference/bench.md) from the 128 fixtures in `internal/compressor/testdata/corpus` and the 4 large outputs in `internal/compressor/testdata/balanced`:

```bash
make bench
```

## 3. Live Paired A/B Kit (Off vs Downshift on Claude Code / Codex)

`scripts/bench-ab/` runs the same self-contained refactor (`refactor-clamp`) and UTF-8 bug-fix (`debug-reverse`) tasks with hooks disabled (`off`) vs wired to an isolated `DOWNSHIFT_STATE_DIR` (`on`), scoring acceptance objectively with hidden `go test ./...` checks.

Verify workspaces and baseline failures first without spending any tokens:

```bash
python3 -I scripts/bench-ab/run_ab.py --harness claude --dry-run
```

Run a live paired comparison and generate a Markdown scorecard (`Billable Tokens`, `Cache Read`, `Cache Hit %`, `List-Rate Est. (USD)` reference, `Compactor Cut`, `CCR Retrieved`):

```bash
make build
python3 -I scripts/bench-ab/run_ab.py --harness claude --task all --reps 3 --out /tmp/ab-results.json
python3 -I scripts/bench-ab/score_ab.py /tmp/ab-results.json
```

> **Note:** Switch `--harness codex` to benchmark Codex CLI (`-c features.hooks=false` vs wired hooks). List-rate USD uses Anthropic API catalog rates from [docs/guides/evidence.md](evidence.md) strictly as a normalized reference; subscription plans convert usage into 5h/weekly quota windows rather than API dollars.
