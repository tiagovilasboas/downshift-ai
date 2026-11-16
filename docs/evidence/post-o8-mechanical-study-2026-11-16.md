# Post-O8 mechanical study — routed vs control (2026-11-16)

Downshift by Tiago Vilas Boas
https://github.com/tiagovilasboas/downshift-ai

This records a controlled study of the current Downshift (after owner decision
O8) on Claude Code. It asks one question: with the same tasks and the same
session model, does a session with Downshift's routing hooks spend fewer tokens
than one without, at the same acceptance? The answer, by the pre-registered
criterion, is **no**: acceptance held, tokens did not drop.

## Setup

- Harness: Claude Code, headless (`claude -p`), session model `opusplan`.
  Headless Claude Code has no plan mode, so `opusplan` runs on Sonnet in both
  arms; this compares Sonnet sessions, not Opus planning.
- Tasks: the four frozen mechanical tasks (`mech-rename`, `mech-doc-comments`,
  `mech-constant`, `mech-test-mirror`), 5 reps per arm, run order shuffled,
  3 in parallel. Each run gets its own scratch copy and its own state dir.
- Arms:
  - `control`: no Downshift hooks at all (`disableAllHooks`, the user's
    Downshift plugin turned off for the run).
  - `routed`: the runner wires Downshift's `PreToolUse`, `PostToolUse`,
    `PostToolUseFailure` and `SubagentStop` hooks; the user's plugin is turned
    off so each hook runs once. Default policy (`{"version":1}`).
- Acceptance: each task's own acceptance check (tests and gofmt on the patch).
- Metric: tokens only (work = input + output, cache = read + write, all
  models). The parity criterion is pre-registered: the lower bound of the
  paired 95% savings interval on total tokens per accepted task must be above
  zero.
- Runner: the private lab harness (`tools/harness-runs`, `battery.py` and
  `aggregate.py`), with the control arm fixed on 2026-11-16: it relied on
  `DOWNSHIFT_NO_ROUTE`, which Downshift removed on 2026-11-02.

Validation before the batch, one forced run per arm: control made 5 spawns with
0 routing decisions and no event log; routed made 5 spawns with 5 decisions and
no duplicate events.

## Run 1 — delegation requested (`--delegate`)

The prompt asks the agent to use subagents. 40 runs, 0 errors, 20.5M tokens.

| Arm | Accepted | Runs with a spawn | Spawns moved down |
|---|---|---|---|
| control | 20/20 | 2/20 | 0 |
| routed | 20/20 | 5/20 | 3 |

The model did these tasks itself; no run reached the 3-spawn floor the
comparison needs. This run does not measure Downshift. It does show that on
small mechanical tasks a Sonnet session rarely delegates, so routing has little
to act on.

## Run 2 — delegation forced (`--force-delegate`)

The prompt requires subagents. 40 runs, 0 errors, 70.5M tokens.

| Arm | Accepted (Wilson 95%) | Spawns | Spawns moved down | Tokens on Haiku | Work / accepted | Cache / accepted |
|---|---|---|---|---|---|---|
| control | 20/20 (84–100%) | 100 | 0 | 0% | 10,012 | 1,730,983 |
| routed | 20/20 (84–100%) | 100 | 29 | 22% | 11,141 | 1,773,724 |

- Paired total tokens per accepted task (20 matched pairs, 10,000 resamples,
  seed 20261029): mean savings **−4.4%**, 95% interval **[−13.0%, +3.8%]**.
- **Parity: FAIL.** The lower bound is below zero. The routed arm spent the
  same or slightly more tokens.
- Acceptance: no loss. Moving 29 of 100 spawns to the small tier failed no
  task.

## What moved the spawns

Every spawn in both arms asked for no model (`inherit`). The 29 moves came from
the decider's class step (`classChoice`, `internal/routing/decide/class.go`):
a decisive TRIVIAL or SIMPLE classification with no risk category moves the
spawn down to small. It is on by default (`routing.by_class`, since
2026-11-01). No move came from an orchestrator hint. So this study measures
classifier-driven routing under O8's precedence, not the orchestrator's own
model choice.

## Reading

- Quality preserved: yes, on these four tasks.
- Token savings: no. Routing moves work to a smaller tier; it does not cut the
  tokens a task takes, and the small worker spent slightly more.
- Where value could show: the share of work on a smaller tier (22% here), which
  this project does not count as savings; or tasks where the orchestrator
  delegates heavier work, which these four are not.

## Limits

- Four mechanical tasks, one harness, one session model, n = 20 per arm.
- Forced delegation is not natural behaviour: Run 1 shows a session rarely
  delegates these tasks on its own.
- Sonnet session, not Opus planning, because headless Claude Code has no plan
  mode.
