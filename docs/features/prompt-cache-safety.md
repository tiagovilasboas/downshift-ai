<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Prompt-Cache Safety & Cache-Hit Telemetry

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift-ai

> [!NOTE]
> Prompt cache reads account for **over 98% of total token volume** in multi-turn coding sessions ([reconciliation](../evidence/anthropic-references-benchmark-reconciliation-2026-11-07.md)). Breaking a conversation prefix mid-session triggers expensive cache writes ($1.25–$5.00/M vs $0.10–$0.20/M reads) and adds latency. Downshift's hot path is designed to keep prefix caches intact.

## Three Cache-Safe Architectural Invariants

1. **Subagent spawn boundary only (`PreToolUse` / bridge):**
   Downshift never changes the main session's model mid-turn or rewrites earlier turns in the active conversation. Model choices apply strictly when spawning an isolated subagent window (`Agent`, `Task`, `spawn_agent`), which already begins with its own fresh context. Verified automatically by Pillar 3 of [`make bench-proof`](../guides/reproduce-benchmark.md): non-spawn calls (`Read`, `Edit`) leave the hook response empty (`{}`), and spawn rewrites modify only `model` (and `reasoning_effort` where supported) while keeping `prompt` and sibling fields byte-identical.
2. **Ingress-only compaction (`Exec` / `PostToolUse`):**
   Command outputs, MCP JSON results, and oversized tool payloads are compacted *before* they enter context. Downshift never rewrites historical assistant messages or earlier system prompts, so the immutable prefix shared across turns stays warm — complementary to `/compact`.
3. **Locked worker tier across follow-up turns (`delegate` / `send`):**
   Downshift MCP workers lock their initial model tier, reasoning effort, directory, and permission mode for every resumed turn ([routing.md](routing.md#guidance-to-the-orchestrator-and-its-workers)), keeping worker-level prefix caches stable instead of bouncing between tiers mid-task.

## Per-Harness Cache Telemetry

Because silent cache-prefix churn can turn a nominal discount into a net loss, Downshift exposes prompt-cache classes alongside billable tokens in `downshift stats --detail` and `downshift report`:

- **Billable vs. cache read kept apart:** `cache_read` is never conflated with uncached `input`, `output`, or `cache_write`, so cheaper-tier work cannot mask cache misses.
- **Per-harness `cache hit %` and `cache write`:** When uncached `input` or `cache_write` is recorded alongside `cache_read` (Claude Code child transcripts today), `downshift report` and `downshift stats --detail` display `cache hit % = cache_read / (cache_read + cache_write + input)` and `cache write` counts directly beneath the routing breakdown. Legacy records with only `cache_read` omit the percentage instead of inventing 100%.

## Verify Locally

```bash
make bench-proof
```

Pillar 3 of [`scripts/bench-proof/run.py`](../../scripts/bench-proof/run.py) asserts both main-session prefix invariance and verbatim spawn prompt preservation directly against the compiled binary.
