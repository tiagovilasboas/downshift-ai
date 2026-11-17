<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Compaction projection for v0.18.0 (2026-11-17)

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift-ai

> **This is a projection, not a measurement.** It applies the v0.18.0 compaction rules to the tool output that one
> operator's Claude Code sessions produced in the 30 days before the release. Nothing here was measured after the
> release. On the [evidence ladder](../guides/evidence.md) it sits below rung B: an estimate from recorded bytes, not
> a saving observed in use.

## Result

| | Before v0.18.0 (measured, 30 days) | After v0.18.0 (projected, same workload) |
|---|---|---|
| Tokens cut by the compactor | about **1.0M** | about **3.5M to 4.5M** |
| Ratio | 1x | **3.5x to 4.5x** |
| Share of all tool output cut | about 3.6% (4.0 MB of 110.5 MB) | about 12% to 16% |

Tokens are estimated as saved bytes / 4, the same rule `downshift stats` uses. They are not billed tokens: each tool
result also stays in context and is read again, cached, on every later turn, so the effect on cost depends on the
model, the cache hit rate and how long the session runs. This page does not convert the figure into money.

## Inputs

All inputs come from one machine (macOS, Claude Code as the main harness), the 30 days ending 2026-11-17.

1. **Baseline saving.** `downshift stats` (binary at `b2701ca7`): `Token ledger (last 30 days): 1.0M cut by the
   compactor (estimate: saved bytes / 4)`, about 4.0 MB.
2. **Coverage before the release.** `downshift discover --dedup --since=30d`: 1,539 transcript files, 74,158 duplicate
   tool results skipped by `tool_use_id`, 43,361 shell commands (1,721 compacted, 5,767 compactible but run bare,
   35,873 skipped; 32,024 of the skips were shell metacharacters). Compacted output was 1.7 MB of 110.5 MB of tool
   output (Bash 1.7 of 87.2 MB, non-Bash 0 of 23.3 MB).
3. **Size distribution by tool.** Every `tool_result` in the Claude Code transcripts modified in the window
   (1,529 files, 69,341 results after deduplication by `tool_use_id`), text bytes only:

| Tool | Results | Total | 4–16 KiB | Over 16 KiB | Cut ceiling |
|---|--:|--:|--:|--:|--:|
| Bash | 45,723 | 90.1 MiB | 47.3 MiB | 8.1 MiB | 16.1 MiB |
| Read | 4,928 | 10.6 MiB | 4.6 MiB | 4.3 MiB | not cut (see below) |
| `mcp__*` | 8,965 | 7.6 MiB | 1.3 MiB | 2.3 MiB | 2.2 MiB |
| WebFetch | 241 | 0.9 MiB | 0.2 MiB | 0.4 MiB | 0.4 MiB |

The tool totals here and the `discover` totals in input 2 come from two passes with different deduplication
scopes, so they differ by a few MB.

**Cut ceiling.** For every result over 4 KiB, the bytes a balanced log cut would remove if it left about 6 KiB (the
first 60 and last 80 lines), counted only when the cut removes at least a quarter of the result. That gate is
the same one `postrun` applies before it replaces an output.

## Arithmetic

| Source | Range | Reasoning |
|---|--:|---|
| Baseline (input 1) | 3.8 MiB | Already cut before the release. |
| Bash results over 4 KiB | +8 to 11 MiB | 50% to 70% of the 16.1 MiB ceiling. Not every large result is cut: a failed run (exit ≠ 0) and a file read (`cat`, `sed -n`, `head`) are never cut, and some results were already compacted before the run. |
| `mcp__*` and WebFetch results over 4 KiB | +1.5 to 2.6 MiB | 60% to 100% of the 2.6 MiB ceiling. Error results and image or other non-text blocks stay whole. |
| **Total** | **13.3 to 17.4 MiB** | 13.9 to 18.2 million bytes, about **3.5M to 4.5M tokens**. |

The families wired by C1 and the pre-run wrapping of chained commands and absolute binary paths mostly overlap with
the post-run cut above: the same output is compacted, only earlier and by a family compressor. They are not counted a
second time.

## What v0.18.0 changes that this projects

- Post-run balanced cut from **4 KiB** instead of 16 KiB on Claude Code (`postrun.PostRunBalanceThreshold`), with the
  inner family of safe chained, quoted, `VAR=x` and absolute-path commands used for the compressor.
- Post-run compaction of **WebFetch and `mcp__*`** results (`updatedToolOutput` / `updatedMCPToolOutput`), wired by
  the new `WebFetch|mcp__.*` PostToolUse matcher in setup and in the plugin.
- Pre-run wrapping of `;` / `&&` chains, quoted globs, binaries under known bin directories and bare `| head` /
  `| tail`, a 2,048-byte command limit (was 512), and nine more families wired to their compressors.
- New compressors: JSON pruning for `gh api` / `curl`, `gh pr checks`, `gh pr diff`, git list subcommands, `wc`,
  and per-file summaries of large `git diff`, `git show` and `git status`.

## What it does not cover

- **Read is not cut**, by design, under 64 KiB, so later edits still match the file text. Its 10.6 MiB is left out.
- **Codex, Cursor and the other harnesses** are not in the projection: only Claude Code lets a PostToolUse hook replace
  a tool's result.
- **Shell commands refused for substitution, redirection or a heredoc** are still not wrapped before the run; only the
  part of their output over 4 KiB is reached, after the run.
- **Honor is not proven per call.** `downshift doctor` on the same machine reported 129 recent post-run rows: 17
  honored (the model saw the replacement), 112 unverified. The projection assumes the replacements are used.

## Preconditions

The projection only holds where all of these are true:

1. Downshift v0.18.0 on `PATH` and, for the Claude Code plugin, `claude plugin update downshift@downshift` (or
   `downshift setup --yes`) so the `WebFetch|mcp__.*` matcher is installed, then a new session.
2. CCR on (the default) and balanced compaction on (the default); `downshift doctor` shows
   `outputs over 16 KB (post-run over 4 KB)`.
3. A workload similar to the one above: shell-heavy sessions with MCP tools.

## How to check it

After a week on v0.18.0, compare against the inputs above:

```bash
downshift stats                       # token ledger for the last 30 days
downshift discover --dedup --since=7d # coverage, Bash vs non-Bash
downshift doctor                      # post-run replacement: honored vs unverified
```

The projection is confirmed if the token ledger for a comparable window lands between 3.5M and 4.5M tokens, and
refuted if it stays near 1.0M.
