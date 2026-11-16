# Post-O8 smoke — three arms, protocol run (2026-11-16)

Downshift by Tiago Vilas Boas
https://github.com/tiagovilasboas/downshift-ai

This records the first run of the pre-registered
[post-O8 benchmark protocol](post-o8-benchmark-protocol-2026-11-07.md): four
frozen tasks, three arms, three repetitions, 36 runs. The verdict by the
protocol's own gate (§6.2) is **FAIL on one criterion**: S1 (mechanical) never
reached a subagent in Arm C, so it could not show a downshift. The other four
criteria hold. Per the protocol, the benchmark stops here and the S1 failure is
triaged before more tokens are spent.

## Setup

- Harness: Claude Code, headless (`claude -p`). Headless Claude Code has no plan
  mode, so `opusplan` runs on Sonnet in Arms B and C.
- Arms, as the protocol specifies:
  - **A** — `claude-opus-5-5`, no Downshift hooks.
  - **B** — `opusplan`, no Downshift hooks (`disableAllHooks`).
  - **C** — `opusplan`, the runner wires Downshift's `PreToolUse`,
    `PostToolUse`, `PostToolUseFailure` and `SubagentStop` hooks, with an
    isolated state dir and the declared policy below.
  - Every arm turns the user's Downshift plugin off, so no hook leaks into A or
    B and none runs twice in C.
- Declared policy for C. The protocol's JSON used keys the current schema does
  not accept; this is the same intent in the valid schema, checked with
  `downshift policy check`:
  ```json
  {"version":1,"min_tier":"small","routing":{"raise_complex":true,
   "profiles":[{"name":"explore","tier":"small","harness":"claude-code"},
               {"name":"plan","tier":"mid","harness":"claude-code"}]}}
  ```
- Tasks (one per category, the protocol's first choice): S1 `mech-rename`,
  S2 `setup-named-group`, S3 `c01-dijkstra`, S4 `t02-reverse`. Each hidden
  check was validated first: it fails on the base or stub and passes on the
  reference.
- k = 3 per task per arm, run order shuffled across arms.
- Metric: tokens only. Billable = input + output + cache write, all models,
  parent and children; cache reads reported apart. Paired by task and
  repetition; 10,000 bootstrap resamples, seed 20261029.

## Acceptance

All 36 runs: 0 errors, 0 timeouts, exit code 0.

| Arm | Accepted | Wilson 95% |
|---|---|---|
| A — Opus | 12/12 | 76–100% |
| B — `opusplan` | 12/12 | 76–100% |
| C — `opusplan` + Downshift | 12/12 | 76–100% |

**Re-judged fixtures.** On the first pass t02 failed 0/9 and c01 7/9 across all
arms. The cause was the runner, not the agents: an agent that wrote its own
`*_test.go` could redeclare a function of the hidden `check_test.go`, which
broke the build of a correct solution. `internal/outcome` judges the solution
alone, so the runner now deletes agent-written test files before the hidden
check, and the 18 t02 and c01 runs were re-judged from their saved patches. All
18 pass. The original verdicts stay in each run's record next to the re-judged
one.

## Tokens

Billable per run:

| Task | A — Opus | B — `opusplan` | C — + Downshift |
|---|---|---|---|
| S1 `mech-rename` | 71,918 | 54,992 | 45,781 |
| S2 `setup-named-group` | 145,181 | 98,383 | 116,961 |
| S3 `c01-dijkstra` | 78,868 | 64,883 | 62,070 |
| S4 `t02-reverse` | 70,513 | 39,701 | 30,211 |
| **All** | **91,620** | **64,489** | **63,756** |

Paired billable savings:

| Comparison | Mean | 95% interval | n |
|---|---|---|---|
| C vs B (the Downshift effect) | +2.9% | [−9.7%, +16.5%] | 12 |
| C vs B, S2 only (the task C routed) | −18.8% | [−31.4%, −7.9%] | 3 |
| B vs A (`opusplan` alone) | +28.7% | [+15.2%, +41.5%] | 12 |
| C vs A | +33.5% | [+21.3%, +44.9%] | 12 |

Almost all of the gain over fixed Opus comes from the session model, which
Downshift does not choose. Downshift's own effect, C against B, is not
distinguishable from zero.

## Routing in Arm C

| Task | Spawns in C (r1, r2, r3) | Decisions | Moved down |
|---|---|---|---|
| S1 `mech-rename` | 0, 0, 0 | 0 | 0 |
| S2 `setup-named-group` | 1, 1, 1 | 3 | 3 (Explore, mid → small, by the profile; served on Haiku) |
| S3 `c01-dijkstra` | 1, 0, 1 | 2 | 0 (general-purpose held at mid) |
| S4 `t02-reverse` | 0, 0, 0 | 0 | 0 |

By the protocol's attribution rule (§6.1.4), the C–B differences on S1, S3 and
S4 are agent variation, not routing: C applied no rewrite there. On S2, the one
task C routed, the routed arm spent more (−18.8%): the Haiku explorer read
more than the Sonnet one it replaced.

## Gate (§6.2)

| Criterion | Result |
|---|---|
| Zero crashes, schema errors or hook timeouts in C | **Pass** |
| Executed child model matches `final_model` (honor) | **Pass** — every moved Explore spawn ran on Haiku |
| Acceptance C ≥ B − 0.10 | **Pass** — 12/12 vs 12/12 |
| S1 shows a clean downshift to small with non-zero savings | **Fail** — no spawn in C on S1, so nothing to move |
| S3 shows `FRONTIER→SMALL == 0` | **Pass** |

**Verdict: FAIL.** The benchmark stops at the smoke.

## Triage of the S1 failure

The failure is not a wrong routing decision; there was no decision to make. On
S1 the Sonnet session did the rename itself in all three C runs (and in two of
three B runs). The protocol assumed S1 would fan out to subagents; under a
Sonnet session it does not. Two readings, for the owner to choose:

1. **Task choice.** S1 should be a task that delegates on its own, or the
   prompt should require delegation. The earlier
   [mechanical study](post-o8-mechanical-study-2026-11-16.md) forced delegation
   on the same task family and found routing moved 29 of 100 spawns down with
   no acceptance loss and no token saving.
2. **Product reading.** Under O8, when the orchestrator does mechanical work
   inline, Downshift has nothing to apply. That is consistent with
   [delegation-boundaries-and-overhead-2026-11-07.md](delegation-boundaries-and-overhead-2026-11-07.md):
   the gain of delegation sits in broad, exploratory work, not small edits.

## Limits

- Four tasks, one harness, k = 3: a directional smoke, as the protocol says.
- Sonnet sessions, not Opus planning, because headless Claude Code has no plan
  mode.
- The fixture re-judge changes the acceptance of t02 and c01 in all three arms
  equally; it does not favor an arm.
- Runner: the private lab harness (`tools/harness-runs`), with the control arm,
  plugin isolation, fixture tasks, the policy write and the fixture judge fixed
  on 2026-11-16.
