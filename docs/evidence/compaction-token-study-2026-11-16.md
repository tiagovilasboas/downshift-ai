<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Compaction study: tokens with and without the compactor (2026-11-16)

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift-ai

Protocol: [compaction-token-study-protocol-2026-11-16.md](compaction-token-study-protocol-2026-11-16.md), merged before
the first run. Verdict by its §6 rule: **FAIL**. With the compactor on, a Claude Code session spent the same billable
tokens as with it off, at the same acceptance.

## Result

48 Claude Code runs (4 tasks × 2 arms × 6), session model `claude-sonnet-5-5`, plain prompt, Downshift at `cb73aab`.

| Rule | Off | On | Holds |
|---|---|---|---|
| 1. Harness crashes, hook errors, hook timeouts | 0 | 0 | yes |
| 2. Acceptance on ≥ off − 0.10 | 18/24 (75%) | 19/24 (79%) | yes |
| 3. On-arm runs compacted ≥ half | | 15/24 | yes |
| 4. Lower bound of billable savings > 0 | | −2.9% | **no** |

Billable-token savings, on against off, all runs, stratified bootstrap (10,000 resamples, seed 20261029):
**+0.9%, 95% interval [−2.9%, +4.4%]**. Accepted runs only: −1.0% [−5.4%, +3.3%]. Cache reads: +0.6% [−14.1%, +12.5%].

| Task | Off: accepted, billable / run | On: accepted, billable / run |
|---|---|---|
| K1 `agent-type-telemetry` | 4/6, 84,068 | 4/6, 82,391 |
| K2 `explore-plan-baseline` | 3/6, 60,774 | 3/6, 58,537 |
| K3 `setup-named-group` | 6/6, 51,234 | 6/6, 53,173 |
| K4 `mech-rename` | 5/6, 45,256 | 6/6, 45,075 |

Batch total: 2.9M billable tokens, under the 10M budget.

## Why the compactor saved nothing here

The compactor ran: 99 commands passed through it in the on arm and 19 were compacted, in 15 of 24 runs. It removed
**2,258 of 214,372 bytes, about 1%** of the command output it saw, which is about 560 tokens over 24 runs.

There was little to cut. Without a nudge, the Sonnet session chains commands and pipes them through `tail` or `grep`
itself, so most outputs arrive short. The probe before the batch showed the passthrough reasons: `small_output` (go vet,
git stash), `nonzero_exit` (a failing `go test`, kept whole on purpose) and `unrecognized_output` (git status and others).
Balanced compaction only acts above 16 KB, which no output reached.

## What this supports

- On these four tasks, on Claude Code with a Sonnet session, compaction did not lower billable tokens. The effect, if
  any, is under the batch's noise (about ±4%).
- It does not show that compaction never helps. Sessions or harnesses that print long, unfiltered test and build output
  are where it can act; this batch had almost none.
- Acceptance did not drop with the compactor on.

## Deviation from the protocol

The runner also ran 24 Codex runs per batch (its default arm). They are outside the protocol and are excluded from every
number above. Nothing else changed after the first run.

## Privacy

Counts, token totals, byte counts and verdicts only. Command output, prompts and paths stayed in the lab workspace.

## Related documentation

- [compaction-token-study-protocol-2026-11-16.md](compaction-token-study-protocol-2026-11-16.md)
- [routing-quota-study-2026-11-16.md](routing-quota-study-2026-11-16.md)
- [ccr.md](../guides/ccr.md)
