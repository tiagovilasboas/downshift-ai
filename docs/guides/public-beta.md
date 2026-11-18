# Public beta — honesty, blockers, and how to help

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift-ai

The **beta** label stays until [beta-exit.md](../reference/beta-exit.md) is signed.
This page is what an external developer can verify locally, and what still needs
a real operator. It is not a release announcement.

## What you can trust today

| Surface | Claim that matches the code |
|---------|-----------------------------|
| Claude Code | Primary path (`Supported` tier, `Completed` status). PreToolUse can rewrite a subagent model and allowlisted commands; `PostToolUse` compacts shell, search, `WebFetch`, and `mcp__*` outputs over 4 KB and `Read` over 64 KB (including suppressed permission modes and background runs). Honor evidence exists for one paid session (2026-10-06) via the child transcript model, not an invoice. `SubagentStop` records transcript token counts and model; current Downshift reports show no dollar amount (`internal/adapters/claudecode/subagentstop_test.go`). |
| Codex | `Beta` tier (`Functional` status). Rewrite emitted; honor write-up is inferred (2026-10-02), not a billing receipt. Needs the hooks feature flag and `/hooks` trust. |
| OpenCode, KiroCrew, Kiro IDE | `Beta` tier. OpenCode routes and reports child usage via the bridge plugin; KiroCrew (`Functional`) enforces policy blocks/retries (no in-place rewrite); Kiro IDE (`Functional`) is observation-only (`PostToolUse` wired). |
| Cursor, Antigravity | `Experimental` tier. **Cursor rewrite honored is not closed** (beta-exit P1.6); Antigravity has one observed child on one build and session IDs wired. Do not treat a hook JSON rewrite as proof the child ran that model. |
| Classifier | Deterministic regex (EN + PT) plus an in-process FNV hash boost. No Graphify, no E5, no external MiniLM command. |
| Usage reporting | `stats`, `report` and `stats --export` report observed tokens and `ponytail` skill adoption (`downshift stats`); they show no dollar amount. `report` shows routed spawns only: billable tokens per completed task and rework. It is not a billing comparison and shows no saving. |

Local checks that do not spend provider credit:

```bash
go test ./...
go run ./cmd/downshift route --harness claude-code --model claude-opus-5-5
go run ./cmd/downshift doctor
```

Synthetic usage fixtures prove the **recorder**. They do not close pillar P3.

## Blockers that still keep the beta label

Copied from beta-exit (2026-10-15), still open after the 2026-10-17 cleanup:

| ID | Blocker | Why this repo cannot close it alone |
|----|---------|--------------------------------------|
| P1.6 | Cursor rewrite honored | Needs a paid Cursor session and a matrix row. Not run here. |
| P2.3–P2.4 | Two or more non-maintainer `stats --export` | Needs other operators for a week. |
| P3.4 | ≥50 `usage` events linked to PreToolUse | Needs real Claude Code SubagentStop traffic. Fixtures are not that count. |
| P3.7 | A real seven-day routed-versus-control comparison in tokens (effect, tokens per completed task, rework) | Cannot be measured as designed since 2026-11-02: the control arm was removed by owner decision and issue #103 is closed. See [beta-exit.md](../reference/beta-exit.md) P3.7; no money is measured. |
| P4 | Blind set ≥200 tasks never used to tune signals | Public `docs/benchmark/report.md` splits are not that set. Do not tune on them. |
| P5.3–P5.4 | Quarterly Cursor and Codex revalidation | Dogfood, not a code change. |

### Protocol for a future P3.4 run (do not treat as done)

1. Install hooks with `downshift setup` on **Claude Code only**, including SubagentStop (`docs/guides/install.md`).
2. Use the product for real work until `downshift stats --export` shows `usage_linked` ≥ 50.
3. Paste the export with prompts and paths removed ([stats-export.md](../reference/stats-export.md)).

## Roadmap for contributors

**Now**

- Misroute issues with `downshift session --why` output (EN or PT).
- Docs that still describe a removed flag (Graphify, `DOWNSHIFT_E5_*`, `DOWNSHIFT_MINILM_EMBED`).
- Adapter tests that use fixtures already in `examples/`.

**Next**

- Anonymised exports from a second machine (P2).
- A blind prompt list of 200+ tasks stored outside the signal-tuning loop (P4). Do not land signal changes in the same PR.

**Later**

- Cursor honor evidence (P1.6) and any harness usage API the vendor actually exposes (P3.8).
- Promoting shadow paths (NB, scores, adapt memory) only after an independent eval.

## Proposed issues (not opened)

Open these only if a search shows no duplicate:

1. `good first issue` — `misroute:` template walkthrough in CONTRIBUTING is enough; add one fixture prompt that is currently Medium with no signal and must stay Medium.
2. `help wanted` — collect two redacted `stats --export` JSON files from non-maintainers (P2.3).
3. `help wanted` — draft 200 blind prompts (id, prompt, expected tier) in a gist or labs repo, with a written rule that `signals.go` must not change in that PR.

## LinkedIn draft (do not post until the maintainer says so)

Downshift is a local hook that applies the orchestrator's model choice for a coding-agent subtask within your declared policy, with no LLM call in its own code. (Since O8, 2026-10-28, it no longer picks a model with classifier rules.) It is still **beta**: Claude Code is the path with honor evidence; Cursor honor and real billing comparisons are open. If you run it for a week, `downshift stats --export` (counts only, no prompts) is the most useful thing you can send. Apache-2.0. https://github.com/tiagovilasboas/downshift-ai

## Cost of this readiness pass

Documentation and local `go test` only. Incremental provider spend: none.
