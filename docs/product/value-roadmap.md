# Value roadmap: compaction where the tokens are, proof, and cheaper workers

Owner direction, 2026-11-06 (status updated 2026-11-18 for `v0.18.1`). Downshift by Tiago Vilas Boas\
https://github.com/tiagovilasboas/downshift-ai

## Status as of 2026-11-18 (v0.18.1)

Waves A, B1–B2, C1, D1–D4, E1–E4, and F1 are **shipped** across `v0.7.0`–`v0.18.1` (`#137`–`#153`), followed by the [impact roadmap](impact-roadmap.md) and [compaction expansion (C1–C5)](roadmap-compaction-expansion.md). Remaining open items: **B3** (7-day post-`v0.18.0` real-use verification against the [2026-11-17 projection](../evidence/compaction-projection-2026-11-17.md)), **D5** (generic CLI worker adapter beyond Claude/Codex), **F2** (`prisma`), and **F3** (Cursor paid-plan honor + Antigravity live `run_command` verification).

## Why (2026-11-06 baseline)

Downshift's unique part is **per-subagent model routing**: the orchestrator picks
the model of each spawn and Downshift applies it within the declared policy. No
comparable tool does this. Its compactor, though, saves little in real use. On the
maintainer's machine, from 2026-11-02 to 2026-11-06 (binary 0.7.0):

| Measure | Value |
|---|---|
| Commands that ran through the compactor | 2,584 |
| Of those, compacted | 272 (10.5%) |
| Bytes saved | 4.0% (12.15 MB to 11.66 MB) |
| Output in a shape no compressor recognised (`unrecognized_output`) | 1,120 runs, 7.3 MB, **60% of all bytes** |
| Biggest unrecognised: `go test` (9 runs) / `npm test` / `git push` | 2.7 MB / 1.9 MB / 1.3 MB |
| Failed runs passed through raw (`nonzero_exit`) | 307 runs, 2.3 MB (`make` alone 1.3 MB) |
| Commands the hook skipped | 7,769: metacharacter 5,191 (67%), too long 1,745, already wrapped 533, not allowlisted 267 |

Part of this is fixed on main and not in 0.7.0 (quoted words, `git commit` with
hooks). The rest is this roadmap. The 533 "already wrapped" came from a global
rule that told Claude Code to call the compactor directly; that rule is gone
(harness-hub #97), so those calls now go through the hook and are counted.

Two references set the bar:

- **Single-purpose CLI compactors**: CLI rewrite hooks across agents and
  commands. They read files with filter levels, group `grep`/`rg` by file, keep the
  full output of a failure for recall, and scan past sessions for missed savings.
  They do not route models.
- **Alethe** (github.com/Kc1t/alethe-orchestrate-skill, MIT): no compaction. It
  saves by delegating to cheaper CLI workers (Codex, Kimi, any CLI) in waves, with
  a playbook catalogue, a quota stop line before each wave, per-wave budgets and a
  per-worker report where the lead reads only the result.

Neither tool touches the agent's own Read, Grep and Glob tools, where most file
content enters the context.

## Phases

Points are story points (1, 2, 3, 5, 8). Each task is one PR that passes `make
check` on its own. A task that touches the binary fills
[coverage.md](../contrib/coverage.md) first.

### A. Compaction where the bytes are

| ID | Task | Pts | Status | Evidence / Shipped |
|---|---|---|---|---|
| A1 | Recognise large `go test -v` and `go test -json` output; keep every FAIL, panic and the summary | 3 | **Done** | `v0.11.0`, `v0.16.0` (`go_test_*`, `go_verbose*`) |
| A2 | Recognise `npm test` / `pnpm test` / `yarn test` when they wrap jest or vitest (script banner, workspace prefixes) | 3 | **Done** | `v0.16.0` (`js_test_runner.go`, `script_banner.go`, `workspace_prefix.go`) |
| A3 | Failure summary for `make` and test runners: the failing target, the first error block and the exit line, nothing cut from a failure | 3 | **Done** | `v0.7.0`–`v0.14.0` (`failed_make.go`, `failed_run.go`, `#134` G2 stderr hold) |
| A4 | File-read family: `cat`, `head`, `tail`, `sed -n 'a,bp'`, `wc -l` on source files, with a mode for long files (outline: signatures and line numbers, full text kept for small files) | 5 | **Done (evolved)** | `file_read.go`, `wc` in `v0.18.0` (`#143`), plus `PostToolUse` `Read` > 64 KB range-cut (`v0.18.0`, `#138`) |
| A5 | Search breadth: `rg` and `grep -r` without `-n`, `--files`, `-l`, `-c`; grouped by file with per-file and total caps | 3 | **Done** | `v0.7.0` (`#845`), `v0.17.0`–`v0.17.1` search cut, `v0.14.0`/`#133` Grep/Glob post-run |
| A6 | Simple pipes and chains: `cmd \| tail -n N`, `cmd \| head`, `cmd \| grep x`, `cd dir && cmd`, `a && b` where every part is allowlisted | 5 | **Done** | `v0.9.0` `ShellDialect`, `v0.14.0` quoted chains, `v0.18.0` (`#137`, `#140`, `#142`, `#147`: `;`/`&&`, quoted globs, known bins, bare `\| head`/`\| tail`, 2,048 B limit, `PostToolUse` inner family) |
| A7 | Keep the full output of a compacted failure by default (CCR on failure only), with `downshift retrieve` | 3 | **Done** | `v0.9.0` CCR on failure; `v0.10.0` CCR on by default; `v0.18.1` (`#150`) 6-gen log retention |

### B. Prove the value

| ID | Task | Pts | Status | Notes |
|---|---|---|---|---|
| B1 | `downshift discover`: scan Claude Code transcripts (and Codex sessions) for Bash calls the compactor would have changed, with bytes and the top missed commands | 3 | **Done** | `v0.9.0`–`v0.18.0`; `discover --dedup` + Bash vs non-Bash coverage (`#139`) |
| B2 | `downshift report --daily` and `--history`, tokens only, per harness | 2 | **Done** | `v0.9.0`, `v0.14.0` |
| B3 | After one week on the new binary, publish the real numbers in the README next to the bench | 1 | **Open** | Pre-`v0.18.0` 48-run study (`compaction-token-study-2026-11-16.md`) motivated `v0.18.0`; awaiting 7-day post-`v0.18.0`/`v0.18.1` run (`downshift stats` & `discover --dedup --since=7d`) |

### C. The agent's own tools

| ID | Task | Pts | Status |
|---|---|---|---|
| C1 | Spike: can Read, Grep and Glob output be shortened per harness (Claude Code PostToolUse, an MCP read tool, or a steering rule toward `rg`/`cat` through the hook)? Write the evidence and a go/no-go | 3 | **Done & shipped** (`v0.14.0` `#72`/`#133` Grep/Glob; `v0.18.0` `#138` `Read` > 64 KB range-cut, `#145`/`#147` `WebFetch`/`mcp__*` compaction; `v0.18.1` `#149` auto-mode postrun) |

### D. Cheaper workers (from Alethe)

| ID | Task | Pts | Status |
|---|---|---|---|
| D1 | Playbook catalogue: scout-then-build, cascade, audit-at-scale, best-of-n; each with tier, effort and what it saves | 2 | **Done** (`cmd_playbook.go`, `internal/playbook`) |
| D2 | Quota stop line before `delegate`: read headroom, refuse past the line and say why | 3 | **Done** (`mcp_quota_stop.go`) |
| D3 | Token budget per wave and per orchestration | 3 | **Done** (`v0.8.1`, `mcp_budget_tokens.go`) |
| D4 | Per-worker report: result kept, transcript left on disk, used or discarded marked | 3 | **Done** (`v0.9.0`, `mcp_reporter.go`) |
| D5 | Codex worker adapter, then a generic CLI worker adapter | 5 | **Partial** — Claude & Codex adapters shipped (`v0.8.1`–`v0.9.0`); generic CLI worker adapter **Open** |

### E. Setup fixes found while reinstalling

| ID | Task | Pts | Status |
|---|---|---|---|
| E1 | Antigravity: when `~/.gemini/AGENTS.md` is a link, skip only that file; still register the MCP server and write the other files | 2 | **Done** (`v0.9.x`; plus `v0.18.1` `#149`/`#150` `SkillsDir` and `SessionIdentifier`) |
| E2 | Undo markers (`*.downshift-bak-*.new`) are written inside harness plugin folders (`~/.config/opencode/plugins/downshift/`); keep them in `~/.downshift/backups`. Also explain why the dry run said "would change" and the write said "new file" for the OpenCode plugin | 2 | **Done** (`v0.9.x`) |
| E3 | Claude Code quota in the desktop app: the statusline never runs there, so quota routing reads a stale value; find another source or say so in `doctor` once | 3 | **Done** (`v0.14.0`, quota gate + `doctor` + `docs/guides/quota.md`) |
| E4 | Claude Desktop rewrites its config while open: detect that it is running and hold the MCP entry until it quits instead of writing a value it will drop | 2 | **Done** (`v0.9.x`) |

### F. Reach

| ID | Task | Pts | Status |
|---|---|---|---|
| F1 | `json` (schema view), log dedup, `curl`/`wget` headers and body caps, `env` | 3 | **Done** (`v0.9.0` `json_schema.go`, `log_dedup.go`, `http_fetch.go`, plus `v0.18.0` `#143` `gh api`/`curl` JSON pruning) |
| F2 | `pnpm`/`bun install`, `rspec`, `rubocop`, `glab`, `prisma` | 3 | **Mostly done** (`pnpm`/`bun install` `v0.7.0`, `rspec` `v0.7.x`/`v0.16.0`, `rubocop` & `glab` `v0.9.0`; `prisma` **Open**) |
| F3 | Automatic rewrite in more harnesses where the hook allows it (spike per harness) | 3 each | **Partial** — OpenCode bridge (`v0.8.0`), Cursor/Codex/Antigravity read-only shell rewrite (`v0.9.0`), Claude Code `PostToolUse` replacement (`v0.14.0`–`v0.18.1`), `kiro` `PostToolUse` (`v0.18.1`); Cursor paid-plan honor & Antigravity live verification **Open** |

## Order

1. **Wave A1–A2 & E1–E4**: **Done** (`v0.7.0`–`v0.18.1`).
2. **Wave B**: D1–D4, C1, B1–B2 **Done**; B3 awaiting 7-day post-`v0.18.0` verification.
3. **Later**: D5 (generic CLI worker), F2 (`prisma`), F3 (remaining harness honor/live verification).

At most three PRs open at once. A wave's PRs merge before the next wave opens new ones.

## Out of scope

- An API-traffic proxy: Downshift stays hook-shaped.
- A desktop workspace (Alethe's app): Downshift has the panel mod and the TUI.
- Any money figure: savings stay in tokens per harness.
