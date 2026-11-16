<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Compaction study protocol: tokens with and without the compactor (2026-11-16)

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift-ai

Status: **pre-registered.** This file is merged before the first run. Any change after the first run is recorded in a
dated note at the end, never by editing the rules above it.

## 1. Question

Does Downshift's output compaction lower the tokens a Claude Code session spends on tasks that run tests and builds, at
the same acceptance?

No controlled study has measured it. In every earlier batch the compactor was not wired or measured nothing
([controlled-suite-opusplan-natural-2026-10-29.md](controlled-suite-opusplan-natural-2026-10-29.md): 0 bytes raw).
Compaction is where a **token** saving can exist: it removes text from the context, while routing only moves work
between tiers (measured separately in quota units:
[routing-quota-study-protocol-2026-11-16.md](routing-quota-study-protocol-2026-11-16.md)).

## 2. What is compared

The compactor has three parts, all on by default:

- `downshift compact -- <command>`: cuts the output of an allowlisted test, build or status command to its pass/fail
  summary and keeps the exit code. On Claude Code a bare allowlisted command is rewritten into this form by the hook.
- CCR: stores full command output so a cut can be retrieved ([ccr.md](../guides/ccr.md)).
- Balanced compaction: cuts an output over 16 KB once CCR stored it.

Arms, both with the same Downshift hook wiring so the hooks' own overhead is equal:

- **off**: `DOWNSHIFT_EXEC=0`, `DOWNSHIFT_CCR=0`, `DOWNSHIFT_BALANCED=0`. Nothing is compacted.
- **on**: the defaults (all three on).

Session model: `claude-sonnet-5-5`, fixed, in both arms. Default policy (`{"version":1}`) in both arms; no task is
expected to delegate, and routing is not what this study measures. Each run has its own scratch copy and state dir;
the user's Downshift plugin is off.

## 3. Tasks

Four frozen tasks in which the agent runs test or build commands with long output, more than once, chosen from saved
lab runs before this protocol was merged.

| Id | Task | Main command families | Saved Bash calls per run | Saved outputs ≥ 4 KB |
|---|---|---|---|---|
| K1 | `agent-type-telemetry` | `go test`, `gofmt`, `grep` | 20–32 | 218 in 25 runs |
| K2 | `explore-plan-baseline` | `go test`, `git stash` | 8–16 | 33 in 24 runs |
| K3 | `setup-named-group` | `go test`, `grep`, `gofmt` | 7–10 | 26 in 31 runs |
| K4 | `mech-rename` | `grep`, `gofmt`, `go test`, `git status` | 10.5 | 9 in 17 runs |

The saved counts come from runs with a delegate nudge. This study uses the plain task prompt, no nudge, in both arms,
because delegation is not what it measures; Bash counts may differ from the table. The fixture tasks were not chosen:
their test output is under 20k characters per run.

Acceptance is each task's hidden check, run by the runner on the final work tree after the session ends, with full
output. The compactor never touches the acceptance check. Each check is validated before the batch: it fails on the
base and passes on a reference solution.

## 4. Run configuration

- k = 6 repetitions per task per arm: 48 runs. Run order shuffled across arms, 3 in parallel.
- Token budget for the batch: 10M billable tokens. If the batch reaches it, it stops and the partial result is
  published as partial.
- Runner: the private lab harness (`tools/harness-runs`), with the `compact-off` and `compact-on` arms added for this
  study. Before the batch, one dry run per arm asserts the generated hook settings and environment.

## 5. Metric and analysis

- Billable tokens = input + output + cache write, all models, parent and children, from the session's final usage
  record (`modelUsage`). Cache reads are reported apart.
- Primary statistic, on all runs (failed ones included, so an arm cannot look cheaper by failing; the accepted-only
  statistic is a secondary line):

  $$\text{savings} = 1 - \frac{\sum_{\text{task}} \overline{T}_{\text{on},\text{task}}}{\sum_{\text{task}} \overline{T}_{\text{off},\text{task}}}$$

  with a stratified bootstrap: runs resampled with replacement within each task × arm cell, 10,000 resamples, seed
  20261029, 95% percentile interval. Runs are not paired by repetition index.
- Reported alongside, no gate: cache reads, Bash calls per run, compacted runs per run, bytes in and out of the
  compactor, CCR retrievals, and the same statistic on cache reads.

**Attribution.** An on-arm run counts as compacted when the compactor cut at least one output. If fewer than half of
the on-arm runs are compacted, the verdict is INCONCLUSIVE.

## 6. Decision rule

**PASS** when all hold:

1. Zero harness crashes, hook errors or hook timeouts in either arm.
2. Acceptance on ≥ acceptance off − 0.10.
3. At least half of the on-arm runs are compacted.
4. The lower bound of the 95% interval of billable-token savings is above zero.

**FAIL** when 1 or 2 fails, or when 4 fails with 3 holding. **INCONCLUSIVE** when 3 fails.

The result is published whatever it is, in `docs/evidence/compaction-token-study-<date>.md`. A PASS supports one claim:
on these four tasks, compaction lowered billable tokens at the same acceptance. It says nothing about other harnesses
or about real use, which the operator export measures.

## 7. Privacy

Run records keep counts, token totals, byte counts and verdicts. Command output, prompts and file paths stay in the lab
workspace and are not published. CCR stores live in each run's isolated state dir and are deleted with the scratch
copy.

## Related documentation

- [routing-quota-study-protocol-2026-11-16.md](routing-quota-study-protocol-2026-11-16.md)
- [ccr.md](../guides/ccr.md)
- [harness-matrix.md](../reference/harness-matrix.md)
