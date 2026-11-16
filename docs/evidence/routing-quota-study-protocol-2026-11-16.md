<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Routing study protocol: quota units on tasks that delegate (2026-11-16)

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift-ai

Status: **pre-registered.** This file is merged before the first run. Any change after the first run is recorded in a
dated note at the end, never by editing the rules above it.

## 1. Why a second metric

Three results on Claude Code show that routing does not lower **tokens**:

- [post-o8-mechanical-study-2026-11-16.md](post-o8-mechanical-study-2026-11-16.md): 29 of 100 spawns moved to small,
  acceptance kept, billable tokens −4.7% (the routed arm spent more).
- [post-o8-smoke-2026-11-16.md](post-o8-smoke-2026-11-16.md): the one task Arm C routed spent 18.8% more tokens.
- [controlled-suite-opusplan-natural-2026-10-29.md](controlled-suite-opusplan-natural-2026-10-29.md), corrected: no
  spawn was moved, so its saving was variation.

The reason is structural. A subagent carries a fixed first-turn context (about 44k tokens) whatever its model, and a
small-tier worker makes as many calls or more. Routing does not shrink the work; it moves the work to a tier that uses
less of the user's quota per token. A tokens-only metric cannot see that by construction.

The owner decided on 2026-11-16 to measure routing in **quota units**, and compaction separately in tokens
([compaction-token-study-protocol-2026-11-16.md](compaction-token-study-protocol-2026-11-16.md)). The two metrics are
never mixed in one verdict.

## 2. Quota units

A quota unit is a token weighted by how much of a provider's quota its tier and token class use, relative to the mid
tier. Weights are the ratio of the provider's public per-token list rates, normalized so that mid = 1 in every token
class. They are unitless: no price, cost or currency appears in any result.

Source: Claude API public rate card, read 2026-11-16. Haiku 5.5 = small, Sonnet 5.5 = mid, Opus 5.5 = frontier.

| Tier | Input | Output | Cache write (5 min) | Cache read |
|---|---|---|---|---|
| small, prompt ≤ 100k tokens | 0.05 | 0.05 | 0.05 | 0.1 |
| small, prompt > 100k tokens | 0.25 | 0.25 | 0.25 | 0.5 |
| mid | 1 | 1 | 1 | 1 |
| frontier | 2 | 2 | 2 | 2 |

- The small tier has two rate cards chosen per call by prompt length (input + cache read + cache write of that call).
  When per-call usage is not available, the ≤ 100k card is used and the run is flagged `quota_card_approx`.
- An unknown model id fails the analysis; it is never weighted as 1 by default.
- The weights are frozen for this study. A later price change does not change these results; it would need a new
  protocol.

**Limit of the metric.** Subscription plans meter usage in ways the provider does not publish. List-rate ratios are the
best public proxy for how much more of a quota a frontier token uses than a small one; they are not a measurement of
any plan's meter.

Every run also reports billable tokens (input + output + cache write) and cache reads, unweighted.

## 3. Arms

Same as the post-O8 protocol's Arms B and C; Arm A (fixed Opus) is dropped, because the smoke already measured it and
Downshift does not choose the session model.

- **B**: Claude Code headless, session model `opusplan`, no Downshift hooks (`disableAllHooks`; the user's Downshift
  plugin off).
- **C**: same session model; the runner wires Downshift's `PreToolUse`, `PostToolUse`, `PostToolUseFailure` and
  `SubagentStop` hooks with an isolated state dir and this policy, checked with `downshift policy check`:

  ```json
  {"version":1,"min_tier":"small","routing":{"raise_complex":true,
   "profiles":[{"name":"explore","tier":"small","harness":"claude-code"},
               {"name":"plan","tier":"mid","harness":"claude-code"}]}}
  ```

  `routing.by_class` stays at its default (on), by owner decision 2026-11-16.

Headless Claude Code has no plan mode, so `opusplan` runs on Sonnet in both arms.

## 4. Tasks

Four frozen tasks that delegate on their own under a Claude Code session, chosen from saved lab runs before this
protocol was merged (spawn rate and acceptance from those runs). The prompt carries the lab's delegate nudge in both
arms; no unnudged data exists, and the nudge is identical across arms.

| Id | Task | Saved runs with ≥ 1 spawn | Mean spawns | Subagent types seen | Saved acceptance |
|---|---|---|---|---|---|
| R1 | `setup-named-group` | 31/31 | 1.4 | Explore, general-purpose | 30/31 |
| R2 | `agent-type-telemetry` | 25/25 | 2.2 | Explore, general-purpose | 23/25 |
| R3 | `explore-plan-baseline` | 24/24 | 1.5 | Explore, general-purpose | 8/24 |
| R4 | `c01-dijkstra` (fixture) | 7/9 | 0.8 | general-purpose | 7/9 |

R3's acceptance is low in every arm; it stays because it delegates reliably, and acceptance is gated per arm, not per
task. R4 has the fewest saved runs; it is the only fixture task that delegated. No saved run used Plan.

Each task's acceptance check is validated before the batch: it fails on the base and passes on a reference solution.

## 5. Run configuration

- k = 6 repetitions per task per arm: 48 runs. Run order shuffled across arms, 3 in parallel, each run with its own
  scratch copy and state dir.
- Token budget for the batch: 15M billable tokens. If the batch reaches it, it stops and the partial result is
  published as partial.
- Runner: the private lab harness (`tools/harness-runs`), with the quota-unit aggregation added for this study.

## 6. Analysis

Primary statistic, per arm pair (C against B), on all runs:

$$\text{savings} = 1 - \frac{\sum_{\text{task}} \overline{Q}_{C,\text{task}}}{\sum_{\text{task}} \overline{Q}_{B,\text{task}}}$$

where $\overline{Q}$ is the mean quota units per run in that cell. All runs count, failed ones included, so an arm cannot
look cheaper by failing; acceptance is gated separately (§7). The same statistic on accepted runs only is reported as a
secondary line.

Token totals per model come from the session's final usage record (`modelUsage`), not from summing streamed messages:
the stream under-reports output tokens. The stream is used only to tell whether a small-tier call crossed the 100k
prompt card. The interval is a stratified bootstrap:
runs are resampled with replacement within each task × arm cell, 10,000 resamples, seed 20261029, 95% percentile
interval. Runs are not paired by repetition index; the index carries no meaning.

Reported alongside, with the same method and no gate: billable tokens, cache reads, spawns per run, spawns moved down,
and the share of child quota units that ran below the requested tier.

**Attribution.** A C run counts as routed when Downshift moved at least one of its spawns down. Results are also
reported on routed C runs only. If fewer than half of the C runs are routed, the verdict is INCONCLUSIVE, whatever the
interval says.

## 7. Decision rule

**PASS** when all hold:

1. Zero harness crashes, schema errors or hook timeouts in C.
2. Every moved spawn ran on the model in its `final_model` record (honor).
3. Acceptance C ≥ acceptance B − 0.10.
4. At least half of the C runs are routed.
5. The lower bound of the 95% interval of quota-unit savings is above zero.

**FAIL** when 1, 2 or 3 fails, or when 5 fails with 4 holding. **INCONCLUSIVE** when 4 fails.

The result is published whatever it is, in `docs/evidence/routing-quota-study-<date>.md`, with the billable-token result
next to it. A PASS supports one claim only: on these four tasks, routing moved work to a tier that uses less quota per
token, at the same acceptance. It does not support a token-saving claim.

## 8. Privacy

Run records keep counts, token totals, model ids and verdicts. Prompts, task text and file paths stay in the lab
workspace and are not published.

## Related documentation

- [post-o8-benchmark-protocol-2026-11-07.md](post-o8-benchmark-protocol-2026-11-07.md)
- [post-o8-smoke-2026-11-16.md](post-o8-smoke-2026-11-16.md)
- [compaction-token-study-protocol-2026-11-16.md](compaction-token-study-protocol-2026-11-16.md)
