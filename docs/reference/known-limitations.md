<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Known limitations

What Downshift does not do yet, in one place. The beta label stays until the [exit criteria](beta-exit.md) close. Per-harness evidence is in the [harness matrix](harness-matrix.md).

## Harnesses

- **Claude Code** is the only harness exercised end to end every release. It is also the only harness with `ReplacesToolOutput` post-run compaction (`Bash`, `Grep`/`Glob`, `WebFetch`, `mcp__*` over 4 KB; `Read` over 64 KiB) and `PostToolUse` `Skill` activation telemetry (`OutcomeSkillUsed`); other harnesses receive embedded skills via `SkillsDir` without a `Skill` hook.
- **Cursor:** the hook is wired, but its neutral `{}` answer and the `agent_message` advice have not been verified live. The shell rewrite is limited to one read-only command.
- **KiroCrew / Kiro IDE:** KiroCrew has only an allow/block `preToolUse` hook, so it cannot rewrite a command or a model (it gets advice with no automatic routing). Kiro IDE wires `kiro.HandlePostToolUse` (`v0.18.1`) for observation-only sensor/main-work/child-end tracking, unverified against a live `PostToolUse` capture.
- **Codex, Antigravity, OpenCode:** partial routing and compaction; see the matrix for each row. Antigravity has `SessionIdentifier` wired (`v0.18.1`), while its `ChildEndEvent` and live `run_command` shell rewrite remain unverified.
- **Claude Desktop** Chat and Cowork host the MCP server only: no hooks, no rewrite, no `SkillsDir`.

## Compaction

- A shell **alias or function** hides the command from the allowlist, so it runs unwrapped.
- A command already written as `downshift compact -- <command>` passes through as typed, including a mutating one.
- `npm test` and `npm run <script>` are not compacted when the script starts a watcher: the script body is not visible. Known follow, watch and server forms (`docker logs -f`, `jest --watch`, `vite preview`) run as typed.
- Quoted shapes (a quoted word, a trailing `2>/dev/null` or `|| true`) are wrapped for every harness that declares a POSIX shell; they are not gated per operating system.
- Post-run tool replacement (`Bash`, `Grep`/`Glob`, `WebFetch`, `mcp__*` over 4 KB; `Read` over 64 KiB; also active in suppressed permission modes and background runs since `v0.18.1`) runs on Claude Code only; other harnesses rely on pre-run rewrite or manual `downshift compact --`.
- **`agent-desktop` CLI vs MCP:** `agent-desktop snapshot` and `agent-desktop find` are in `readOnlyFamilies` and get the `agent-desktop` view + delta compressor over CLI; `agent-desktop` MCP tools do not hit the `agent-desktop` view/delta compressor (on Claude Code, `mcp__*` text > 4 KB receives only the generic balanced cut).
- Rotated compactor logs (`exec-events.jsonl`) retain 6 generations (`.1`..`.6`, `v0.18.1`).

## Raw output kept by CCR

- Redaction covers common credential shapes, including AWS STS session credentials (`AccessKeyId`, `SecretAccessKey`, `SessionToken`, `ASIA...` prefixes across JSON, table, text, and environment outputs) and AWS SSM Parameter Store values (`Value` in JSON blocks, CLI text, and table outputs). For custom commands that print secrets in unsupported arbitrary shapes, turn CCR off (`"ccr": false` and `"ccr_failures": false`, see the [CCR guide](../guides/ccr.md)) before running those, or do not run them through `downshift compact`.

## Setup

- `downshift setup --undo` restores the whole backup, so a change you made to the same file after setup is lost. Review the diff first.
- Setup leaves a linked file alone and holds the Claude Desktop entry while Claude is running.

## Measurement

- Savings are tokens, never money. How a plan turns tokens into its 5-hour and weekly limits is not published, so no limit saving is claimed.
- Quota routing reads a statusline snapshot that only an interactive Claude Code terminal writes; elsewhere it can be stale.

Found something not listed? [Open an issue](https://github.com/tiagovilasboas/downshift-ai/issues).
