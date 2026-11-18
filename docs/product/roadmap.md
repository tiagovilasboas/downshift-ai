<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Product roadmap: from router to cost control plane

Downshift by Tiago Vilas Boas
https://github.com/tiagovilasboas/downshift-ai

Owner-facing plan. Status as of 2026-11-18 (`v0.18.1`, [release notes](../releases/v0.18.1.md)). The public summary stays in
[roadmap.md](../roadmap.md).
Staff review follow-up checklist: [review-2026-10-20.md](review-2026-10-20.md).
Context protection & reasoning roadmap (empirical evolution, **Done**): [roadmap-context-and-reasoning.md](roadmap-context-and-reasoning.md).
Performance & architecture hardening roadmap (waves W0–W5): [roadmap-perf-and-architecture.md](roadmap-perf-and-architecture.md).
Value roadmap (compaction, proof, cheaper workers): [value-roadmap.md](value-roadmap.md).
Impact roadmap (compactor blind spots, latency, proof in real use): [impact-roadmap.md](impact-roadmap.md).
Compaction expansion roadmap (C1–C5, **Done in v0.18.0**): [roadmap-compaction-expansion.md](roadmap-compaction-expansion.md).

**2026-11-02:** the built-in control group (P1.1) was removed by owner decision, so every spawn is routed. Where this plan says "routed vs control", that comparison is the original design and cannot be measured now. `downshift report` is routed-only.

**2026-11-18 (`v0.18.0`–`v0.18.1`, `#137`–`#153`):** Compaction expansion (`C1`–`C5`), context protection (`P1`–`P3`), 4 KB post-run cut (`Bash`, `WebFetch`, `mcp__*`, `Grep`, `Glob`, `Read` > 64 KB), `agent-desktop` read-only pre-run + delta, `ponytail` + `OutcomeSkillUsed` skill telemetry, 6-gen log retention, `discover --dedup`, cross-harness `SkillsDir` parity (Claude Code, Codex, Cursor, OpenCode, Antigravity), and `web-motion` shipped. Remaining beta exit & product focus: multi-operator beta evidence ([beta-exit.md](../reference/beta-exit.md) P2), Cursor paid-plan rewrite honor (P1.6), post-`v0.18.0` 7-day compaction measurement against the [2026-11-17 projection](../evidence/compaction-projection-2026-11-17.md), and the blind outcome suite (`GAP-SUITE`).

## Positioning

**Today:** "picks a cheaper model for each subagent".
**Target:** "the neutral cost control plane for AI coding agents: proves how much
you saved without losing quality, corrects itself when it is wrong, and gives a
team one policy and one report across every harness".

Why this position: harnesses are absorbing basic routing (Claude Code
`opusplan` and per-agent models, Cursor Auto). A better classifier alone becomes
someone's built-in feature. Cross-harness policy, proof and reporting do not,
because each vendor only sees its own product.

## North-star metric

**Cost per completed task, at equal quality**, routed vs control, with a
confidence interval. Quality = the task did not need a retry, respawn or a
higher-tier redo.

Guard metrics: frontier-on-doubt count = 0; real overspend (USD) visible; retry
rate per tier; share of spawns routed; share of spend covered (subagents vs main
session).

## Milestones

| # | Milestone | Outcome | Gate to exit |
|---|-----------|---------|--------------|
| M0 | Trustworthy core | No silent overspend; metrics can show a loss | **done** 2026-10-13, see "Done" |
| MX | Visible in the flow | The developer sees subagents, compaction, and routing live | PX.1–PX.2 & PX.4 (`downshift watch`) live; `/ds-agents` panel shipped in plugin |
| M1 | Proof of value | Verified token savings and quality parity across operators | control arm removed 2026-11-02; remaining: multi-operator exports ([beta-exit](../reference/beta-exit.md) P2), blind outcome suite, post-`v0.18.0` 7-day check |
| M2 | Self-correcting | Downshift raises its own tier when cheap fails | retry-rate guard & CCR anti-loop guard (`P3`) live; explainable decisions & learned scorer open |
| M3 | Zero config, every harness | Install and it works; consistent skills, hooks, and discovery | `SkillsDir` parity (Cursor, OpenCode, Antigravity) & `kiro`/`antigravity` wiring shipped in `v0.18.1`; Cursor paid honor & session-start discovery open |
| M4 | Covers where the money goes | Main session and workflow agents, not only subagents | Read/Grep/Glob/WebFetch/MCP post-run compaction shipped (`v0.14.0`–`v0.18.0`); main-session plan/execute & workflow hooks open |
| M5 | Team and company | Central policy, budgets, aggregated prompt-free reporting | first team pilot |

## M0: trustworthy core (done)

| ID | Item | Status |
|----|------|--------|
| P0.1 | R7/R8: doubt never spends money; semantic boost capped; PT-BR signals | done (`c8f1f8b`) |
| P0.2 | Inferred session on Claude Code (`InferSession`) | done (`c25afe7`, `aa3a538`) |
| P0.3 | `SEMANTIC_BOOST` provenance and frontier-on-doubt alarm | done |
| P0.4 | One `deterministicLift`; tier floor no longer held; dead gate removed (T1) | done |
| P0.5 | Target by catalog tier, not list position (T2) | done |
| P0.6 | `stats` shows a loss: signed delta, overspend, upshift cost (T3) | done |
| P0.7 | `CLAUDE_CODE_SUBAGENT_MODEL` baseline; 32 KB classify cap (T4, T5) | done |
| P0.8 | Final-decision guardrail invariant after every Route layer | done (`8e5cd24`) |

## MX: visible in the flow

Why: the product's work is invisible unless the developer opens a report. Seeing
each subagent's model, cost and redo inside the harness is what builds trust.

| ID | Task | Acceptance | Size | Depends |
|----|------|------------|------|---------|
| PX.1 | Statusline session summary (U1) | live line, < 50 ms, never breaks the statusline | S | T3 |
| PX.2 | Subagent end line (U2) | one line with model, tokens, cost, would-be cost | S | — |
| PX.3 | `/ds-agents` session tree & `/downshift` command (U3) | `/ds-agents` live in Claude Code mod; unified `/downshift` pending | M | PX.2 |
| PX.4 | `downshift watch` replaces the `dsmon` build (U4) | **done** (`cmd/downshift/watch.go`), same binary, every harness | M | PX.3 |

## M1: proof of value

Why first: without it nobody defends the tool in a budget review, and every
later milestone needs its numbers.

| ID | Task | Acceptance | Size | Depends |
|----|------|------------|------|---------|
| P1.1 | **Removed 2026-11-02 (owner decision).** Built-in control group: a configurable share of spawns (default 10%) is classified and logged but not rewritten, chosen by a hash of the spawn id | control share within 1 pp of target over 500 spawns; `outcome: baseline`; prompt-free | M | — |
| P1.2 | ~~Cost per completed task: link decision, usage and child status into one task record; mark retry/respawn within the window as rework~~ | superseded by 2026-10-31 (no money) & 2026-11-02 (no control arm); `downshift report` is tokens-only and routed-only | M | P1.1 |
| P1.3 | ~~Savings report with a confidence interval (bootstrap over tasks)~~ | superseded with P1.1/P1.2; `downshift report --daily` / `--history` shipped in tokens | S | P1.2 |
| P1.4 | Billing reconciliation when an admin usage key exists (Anthropic, OpenAI) | deferred / out of tokens-only scope | M | P1.3 |
| P1.5 | ~~Dogfood: two weeks on the maintainer Mac with P1.1 on~~ | superseded; see [compaction-token-study-2026-11-16.md](../evidence/compaction-token-study-2026-11-16.md) & [post-o8-mechanical-study-2026-11-16.md](../evidence/post-o8-mechanical-study-2026-11-16.md) | S | P1.3 |
| P1.6 | Gate the eval pin in CI (T6, option 2: vendored prompt-free summary) | CI fails when the 37/40 floor regresses | S | owner decision |
| P1.7 | Post-`v0.18.0` 7-day compaction measurement (`downshift stats` + `discover --dedup --since=7d`) | verify [2026-11-17 projection](../evidence/compaction-projection-2026-11-17.md) (~3.5M–4.5M tokens/month projected) | S | — |
| P1.8 | Multi-operator beta evidence ([beta-exit.md](../reference/beta-exit.md) P2) | ≥2 non-maintainer exports (`downshift export --days=7`) | M | community |

## M2: self-correcting

| ID | Task | Acceptance | Size | Depends |
|----|------|------------|------|---------|
| P2.1 | Retry-rate guard: per task shape, if rework after a downshift exceeds a threshold, raise that shape's floor automatically and say so | simulated failures raise the floor; a clean run lowers it back after N successes | M | P1.2 |
| P2.2 | Explainable decision: one human line per spawn ("haiku: rename, no risk, confident") in `systemMessage` and `stats --explain` | covers every reason code; no prompt text | S | — |
| P2.3 | Upshift by capability, not "last id in the list" | decision recorded; tests; `explicit_upshift` keeps the strongest | S | **owner decision** |
| P2.4 | Haiku 5.5 / Sonnet 5.5 measured on the 40-task suite (separate `runs-claude/`) | pass rate per label; decides how much MEDIUM can go to Haiku | S | owner OK to spend plan usage |
| P2.5 | Learned complexity scorer EN/PT-BR (shadow, abstain to MEDIUM) | beats regex on held-out set incl. PT-BR; ~2,400 labels | L | labels |

## M3: zero config, every harness

| ID | Task | Acceptance | Size | Depends |
|----|------|------------|------|---------|
| P3.1 | Public score index (S1): Epoch AI + OpenRouter, single source per comparison, id map as data | removed with the `scoreindex` package (`2877d995`); `downshift models scores` is not in the binary | M | removed |
| P3.2 | Setup + daily refresh (S2) | superseded/removed with `scoreindex` (`2877d995`) | S | removed |
| P3.3 | Capability selection (S3), shadow by default | superseded/removed with `scoreindex` (`2877d995`) | M | removed |
| P3.4 | Promote capability selection to live | superseded with `scoreindex` removal | S | — |
| P3.5 | Session-start discovery for Codex, Cursor, Kiro (their session-start hooks) | lists refreshed per session; `SessionFromCatalog` enabled only with evidence | M | per-harness evidence |
| P3.6 | Docs for scores (S4) | superseded/removed with `scoreindex` (`2877d995`) | S | removed |
| P3.7 | Cross-harness `SkillsDir`, `SessionIdentifier`, and `PostToolUse` parity | **done** in `v0.18.1` (`#149`, `#150`): `SkillsDir` for Cursor, OpenCode, Antigravity; `antigravity` `SessionIdentifier`; `kiro.HandlePostToolUse` | S | — |

## M4: cover where the money goes

| ID | Task | Acceptance | Size | Depends |
|----|------|------------|------|---------|
| P4.1 | Measure the share of spend that is subagents vs main session | `stats` shows coverage % | S | P1.2 |
| P4.2 | Plan-expensive / execute-cheap mode for the main session where the harness allows (Claude Code `opusplan`-like policy) | opt-in; measured in M1 terms | M | P4.1 |
| P4.3 | Workflow-created agents (GAP-WORKFLOW-AGENTS) | routed or documented as out of reach per harness | M | vendor hook support |

## M5: team and company

| ID | Task | Acceptance | Size | Depends |
|----|------|------------|------|---------|
| P5.1 | Central policy file (allowed models, floors, budgets) distributable by MDM/managed settings | one file applied to every harness on a machine | M | — |
| P5.2 | Aggregated prompt-free reporting: merge exports from many machines, per team/project | one report for N developers | M | P1.3 |
| P5.3 | Budget alerts and overspend alerts (frontier-on-doubt, real overspend) | alert hooks (webhook/Slack) opt-in | S | P5.2 |
| P5.4 | Pilot with one team | written case study with numbers | M | M1-M3 |

## Decisions pending (owner)

1. **P2.3** Upshift target: last id in the list (today) or smallest model that
   meets the tier (recommended).
2. **P1.6/T6** CI gate for the eval pin: option 2 recommended.
3. **P2.4** Spend plan usage to measure Haiku/Sonnet 5.5 (80 short calls) alongside the blind outcome suite (`GAP-SUITE`).
4. `downshift-labs` suite freeze ("Congelo?").
5. **OpenRouter parts 4–6** — owner 2026-10-24: last in queue; finish multi-operator beta evidence, post-`v0.18.0` compaction verification, and Cursor paid-plan honor first.

## What not to do now

- Do not tune the regex classifier further: precision is not the bottleneck;
  proof and trust are.
- Do not let a public index decide a task's tier: it measures models, not tasks.
- Do not turn capability selection live before M1 numbers exist.
- Do not start OpenRouter parts 4–6 before multi-operator beta evidence, post-`v0.18.0` compaction verification, and Cursor paid-plan honor land (owner 2026-10-24, updated 2026-11-18).
