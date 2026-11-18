<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Compaction Expansion Roadmap (C1–C5)

Downshift by Tiago Vilas Boas\
https://github.com/tiagovilasboas/downshift-ai

**Status: Done in `v0.18.0` (`2026-11-17`, `#137`–`#147`) with follow-on fixes in `v0.18.1` (`2026-11-18`, `#149`–`#150`).**

Follows the [impact roadmap](impact-roadmap.md). Targeted the compaction
gaps across `execwrap` (command allowlist & compressor wiring) and `postrun`
(`PostToolUse` shell extraction, large `Read` range-cuts, `WebFetch`/`mcp__*`,
and CCR-backed fallback compaction from 4 KB).

- **Shipped (`v0.18.0`):** `C1` (`#144`), `C2` (`#137`, widened further in `#142`/`#147`), `C3` (`#140`), `C4 / M3` (`#138`), and `C5` (`#140`, `#143`, `#145`, `#146`, `#147`).
- **Follow-on (`v0.18.1`):** `WouldRewriteEvent` evaluation for `PostToolUse` in suppressed permission modes and background runs, `agent-desktop` read-only classification, `direct`/retrieval token accounting (`#149`), and 6-generation rotated log retention (`#150`).
- **Remaining verification:** Post-release 7-day measurement (`downshift stats` and `downshift discover --dedup --since=7d`) against the [2026-11-17 compaction projection](../evidence/compaction-projection-2026-11-17.md) (~3.5M–4.5M tokens/month projected vs ~1.0M baseline; not yet measured post-release).

Every task was capped at **5 story points**, shipped as independent PRs passing
`make check` with [coverage.md](../contrib/coverage.md) filled first.

## Tasks (C1–C5)

| ID | Task | Pts | Wave | Status | Shipped |
|---|---|---|---|---|---|
| C1 | Wire existing unwired compressors into `expected_format.go` (`terraform plan`, `cargo clippy`, `cargo tree`, `docker logs`, `kubectl logs`, `composer`, `artisan`/`sail artisan`, `az pipelines`) | 2 | Wave 1 | **Done** | `v0.18.0` (`#144`) |
| C2 | Widen `maxCommandBytes` (`512` → `2048`) in `policy.go` and `SimpleFamily` (plus `;`/`&&`, quoted globs, known bin dirs, bare `\| head`/`\| tail`) | 2 | Wave 1 | **Done** | `v0.18.0` (`#137`, `#142`, `#147`) |
| C3 | Extract inner/leading allowlisted `Family` in `compactShell` (`PostToolUse`) + auto-mode `WouldRewriteEvent` fix | 3 | Wave 2 | **Done** | `v0.18.0` (`#140`), `v0.18.1` (`#149`) |
| C4 / M3 | `Read` > 64 KB range-cut preserving verbatim lines, line numbers, and CCR `retrieve id` | 5 | Wave 1 | **Done** | `v0.18.0` (`#138`) |
| C5 | 4 KB `PostToolUse` fallback (`PostRunBalanceThreshold`), JSON pruning (`gh api`/`curl`), `gh pr checks`/`diff`, git lists/`wc`, large `git diff`/`show`/`status`, `WebFetch`/`mcp__*`, read-only `downshift` CLI, `discover --dedup` | 3 | Wave 2 | **Done** | `v0.18.0` (`#140`, `#141`, `#143`, `#145`, `#146`, `#147`) |

## Parallelization Matrix

Wave 1 tasks touch disjoint files and have no shared state or decision
dependencies, so they run in parallel. Wave 2 starts once its Wave 1
predecessors land.

| Task | Wave | Owned Files / Packages | Conflicts With | Can Run in Parallel With |
|---|---|---|---|---|
| C1 | Wave 1 | `internal/execwrap/expected_format.go` (+ `internal/execwrap/` tests) | — | C2, C4 / M3 |
| C2 | Wave 1 | `internal/execwrap/policy.go`, `internal/execwrap/captured.go` (+ `policy_test.go`, `captured_test.go`) | — | C1, C4 / M3 |
| C4 / M3 | Wave 1 | `internal/postrun/read.go`, `internal/adapters/claudecode/tooloutput.go` (+ `internal/postrun/` & adapter tests) | — | C1, C2 |
| C3 | Wave 2 | `internal/postrun/postrun.go` (+ `internal/postrun/` tests) | C5 (both touch `internal/postrun/postrun.go` flow) | — |
| C5 | Wave 2 | `internal/postrun/postrun.go`, `internal/compressor/` (cloud / `gh api` / `kubectl describe`), `internal/execwrap/expected_format.go` | C3 | — |

## Task Specifications

### C1 — Wire existing unwired compressors (2 SP, Wave 1 — **Done in `#144`, `v0.18.0`**)

- **Problem:** Eight compressors already implemented in `internal/compressor` are
  not mapped in `internal/execwrap/expected_format.go`, so allowlisted runs
  fall through without their dedicated formatters.
- **Target files & symbols:**
  - `internal/execwrap/expected_format.go`: wire `terraform-plan`,
    `cargo-clippy`, `docker-logs`, `kubectl-logs`, `composer`,
    `artisan` / `sail-artisan`, `az-pipelines`, and `cargo-tree` to their
    corresponding `compressor.Format*` chains.
  - Adjacent `internal/execwrap/*_test.go`: table-driven tests verifying each
    family resolves to its expected format chain and compacts sample output.
- **Verification criteria:**
  - Every one of the 8 families maps to a non-empty expected format slice in
    `internal/execwrap/expected_format.go`.
  - `make check` passes (`go test ./internal/execwrap/...` and cross-OS `go vet`).

### C2 — Widen `maxCommandBytes` from `512` to `2048` (2 SP, Wave 1 — **Done in `#137`, `v0.18.0`**)

- **Problem:** Long but valid allowlisted commands (e.g., multi-flag test runs,
  long paths, or `gh`/`kubectl` invocations) exceed `maxCommandBytes = 512` and
  are skipped as `too_long`.
- **Target files & symbols:**
  - `internal/execwrap/policy.go`: update `const maxCommandBytes = 512` to `2048`.
  - `internal/execwrap/captured.go`: `SimpleFamily(command string) (Family, []string)`
    reuses `maxCommandBytes` and automatically honors the `2048`-byte ceiling.
  - `internal/execwrap/policy_test.go` and `internal/execwrap/captured_test.go`:
    update boundary tests for `<= 2048` accepted vs `> 2048` rejected.
- **Verification criteria:**
  - Allowlisted commands between 513 and 2048 bytes are accepted by policy and
    classified by `SimpleFamily` instead of skipping as `too_long`.
  - Commands > 2048 bytes still fail safe as `too_long`.
  - `make check` passes.

### C3 — Extract inner/leading allowlisted `Family` in `compactShell` (3 SP, Wave 2 — **Done in `#140`, `v0.18.0`**; `#149`, `v0.18.1`)

- **Problem:** `internal/postrun/postrun.go` (`compactShell`) originally called
  `execwrap.SimpleFamily(in.Command)`, which rejected safe compound, quoted, or
  absolute-path commands and fell back to `DetailNoCompressor`.
- **Target files & symbols:**
  - `internal/postrun/postrun.go`: update `compactShell(in hookport.ToolOutput, o Options) Result`
    to extract the inner or leading allowlisted `execwrap.Family` for safe
    compound/quoted/absolute-path commands so `PostToolUse` applies family
    compressors instead of `DetailNoCompressor`.
  - `internal/postrun/postrun_test.go`: table-driven cases for quoted binaries,
    absolute paths (e.g., `/usr/local/bin/go test ./...`), env-prefixed
    commands, and safe compound invocations.
- **Verification criteria:**
  - `PostToolUse` applies the matching family compressor on safe compound,
    quoted, and absolute-path commands instead of emitting `DetailNoCompressor`.
  - Unsafe metacharacters or unrecognized commands still fail open safely.
  - `make check` passes.

### C4 / M3 — `Read` > 64 KB range-cut (5 SP, Wave 1 — **Done in `#138`, `v0.18.0`**)

- **Problem:** Large `Read` outputs (> 64 KB) enter context whole, while
  rewriting or truncating individual lines breaks subsequent `Edit` tool
  matches.
- **Target files & symbols:**
  - `internal/postrun/read.go`: implement `Read` > 64 KB range-cut, preserving
    verbatim lines and line numbers, storing raw content in CCR, and emitting a
    marker with the cut range and CCR `retrieve id`.
  - `internal/adapters/claudecode/tooloutput.go`: extract `resp["file"]["content"]`
    (alongside top-level `resp["content"]`) when parsing Claude Code `Read`
    `PostToolUse` payloads.
  - `internal/postrun/read_test.go` and `internal/adapters/claudecode/tooloutput_test.go`:
    verify verbatim line preservation, line numbering, CCR `retrieve id`
    emission, `<= 64 KB` passthrough, and `Edit` compatibility on retained
    ranges.
- **Verification criteria:**
  - A `Read` output > 64 KB (e.g., lockfile or generated file) is range-cut
    with verbatim lines, intact line numbers, and a CCR `retrieve id` marker.
  - An `Edit` targeting any retained range matches without line-drift or
    reformatted text.
  - `Read` outputs <= 64 KB pass through untouched; `make check` passes.

### C5 — Lower-threshold `PostToolUse` fallback & high-volume compressors (3 SP, Wave 2 — **Done in `#140`/`#143`/`#145`/`#146`/`#147`, `v0.18.0`**)

- **Problem:** Commands classified as `other` (`execwrap.FamilyNone`) and
  high-volume cloud/API/cluster inspection outputs (`gh api`, `kubectl describe`,
  cloud CLIs) pass through `PostToolUse` uncompacted even when CCR has stored
  the raw output.
- **Target files & symbols:**
  - `internal/postrun/postrun.go`: lower-threshold `PostToolUse` fallback
    compaction using `compressor.BalanceLog` / `compressor.FormatGeneric` for
    `execwrap.FamilyNone` (`other`) when CCR is enabled (`o.Keep != nil`) and
    `exit == 0`.
  - `internal/compressor/` and `internal/execwrap/expected_format.go`: add and
    wire compressors for high-volume cloud CLIs, `gh api`, and
    `kubectl describe`.
  - Unit tests in `internal/postrun/` and `internal/compressor/`.
- **Verification criteria:**
  - `FamilyNone` (`other`) outputs with `exit == 0` and `o.Keep != nil` compact
    via `BalanceLog` / `FormatGeneric` at the lower threshold while preserving
    raw output in CCR; non-zero exits or `o.Keep == nil` remain safe.
  - High-volume cloud, `gh api`, and `kubectl describe` fixtures compact cleanly
    and pass `make check`.
