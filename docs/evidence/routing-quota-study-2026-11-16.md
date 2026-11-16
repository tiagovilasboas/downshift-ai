<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Routing study: quota units on tasks that delegate (2026-11-16)

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift-ai

Protocol: [routing-quota-study-protocol-2026-11-16.md](routing-quota-study-protocol-2026-11-16.md), merged before the
first run. Verdict by its §7 rule: **FAIL**. Routing moved 18 of 25 spawns to the small tier and kept acceptance, and
the point estimate is a quota-unit saving, but the interval includes zero.

## Result

48 Claude Code runs (4 tasks × 2 arms × 6), session model `opusplan` (Sonnet headless), delegate nudge in both arms,
Downshift at `cb73aab`. B = no hooks, C = Downshift hooks with the protocol's policy.

| Rule | B | C | Holds |
|---|---|---|---|
| 1. Harness crashes, schema errors, hook timeouts in C | | 0 | yes |
| 2. Every moved spawn ran on its `final_model` | | 18 moved; every run with a move used `claude-haiku-5-5` | yes |
| 3. Acceptance C ≥ B − 0.10 | 19/24 (79%) | 18/24 (75%) | yes |
| 4. C runs routed ≥ half | | 18/24 | yes |
| 5. Lower bound of quota-unit savings > 0 | | −6.7% | **no** |

Quota-unit savings, C against B, all runs, stratified bootstrap (10,000 resamples, seed 20261029):
**+8.9%, 95% interval [−6.7%, +22.3%]**. Accepted runs only: +4.5% [−16.0%, +21.9%]. No run needed the long small-tier
card (`quota_card_approx` = 0).

Unweighted, alongside, no gate: billable tokens **−5.7% [−15.7%, +4.0%]** and cache reads −10.9% [−28.4%, +4.0%]. C
spent slightly more tokens and less quota on the point estimates, neither established: the work moved to a cheaper tier and the small worker used more tokens to do
it, as in the [mechanical study](post-o8-mechanical-study-2026-11-16.md).

| Task | B: accepted, quota / run | C: accepted, quota / run | Change |
|---|---|---|---|
| R1 `setup-named-group` | 6/6, 893,091 | 6/6, 755,535 | −15% |
| R2 `agent-type-telemetry` | 6/6, 2,454,490 | 6/6, 2,451,876 | 0% |
| R3 `explore-plan-baseline` | 1/6, 1,412,302 | 0/6, 1,123,324 | −20% |
| R4 `c01-dijkstra` | 6/6, 335,573 | 6/6, 310,486 | −7% |

Batch total: 5.8M billable tokens, under the 15M budget.

## What moved

All 18 moves were `Explore` spawns sent to small by the policy's `explore` profile, with a confident classification
(15 MEDIUM, 3 TRIVIAL). The 7 spawns left in place were `general-purpose` (6, five with a risk category) and one
unspecified COMPLEX spawn. No move came from an orchestrator `[tier=...]` hint.

## Reading

- Routing does what it claims mechanically: spawns moved, honored, no acceptance loss.
- The quota effect points the right way on three of four tasks but is not established at n = 6 per cell. Most of a
  run's quota is the parent session's own cache reads, which routing does not touch, so a moved Explore child is a small
  share of the total.
- R3's acceptance is low in both arms, as in the saved runs; its −20% includes runs that failed.
- No token saving: the unweighted totals went slightly up.

## Deviation from the protocol

The runner also ran 24 Codex runs per batch (its default arm). They are outside the protocol and are excluded from every
number above. Nothing else changed after the first run.

## Privacy

Counts, token totals, model ids and verdicts only. Prompts, task text and paths stayed in the lab workspace.

## Related documentation

- [routing-quota-study-protocol-2026-11-16.md](routing-quota-study-protocol-2026-11-16.md)
- [compaction-token-study-2026-11-16.md](compaction-token-study-2026-11-16.md)
- [post-o8-mechanical-study-2026-11-16.md](post-o8-mechanical-study-2026-11-16.md)
