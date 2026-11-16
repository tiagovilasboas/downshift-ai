<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Performance & Architecture Hardening Roadmap

Downshift by Tiago Vilas Boas
https://github.com/tiagovilasboas/downshift-ai

**Status:** Phase 1 Done (W0–W5 merged in #114) · Phase 2 Planned (W6–W11)
**Scope:** Hot-path performance bottlenecks, cold-start regex cost, redundant transcript I/O, targeted format detection, automated `archtest` layer enforcement, and zeroing `funcCountBaseline` SRP debt.

---

## Phase 1 — Delivered Waves (W0–W5, PR #114)

| Wave | Task | Story Points | Target Packages | Status |
| :---: | :--- | :---: | :--- | :---: |
| **W0** | Map roadmap and wave tasks | 1 SP | `docs/product/` | Done |
| **W1** | `ccr.Redact` fast-path literal prefilters & single-pass replacement | 1 SP | `internal/ccr` | Done |
| **W2** | `classify` cold-start & hot-loop optimization (skip runtime embed re-validation, single `FindAllStringIndex` pass) | 2 SP | `internal/classify` | Done |
| **W3** | `claudecode` `SubagentStop` settle-before-parse & single-pass transcript read (usage + verification) | 2 SP | `internal/adapters/claudecode` | Done |
| **W4** | `execwrap` → `compressor` targeted format detection (`expectedFormats` hint; superseded in #117 by full `detectFormat` parity) | 2 SP | `internal/compressor`, `internal/execwrap` | Done (reverted in #117) |
| **W5** | Enforce adapter boundary in `archtest/imports_test.go` (forbid `routing/policy` and `routing/decide`) | 2 SP | `internal/archtest` | Done |

---

## Phase 2 — Planned Waves (W6–W11)

### Engineering Principles & Constraints (Guia × Sensor)
1. **SRP & Domain of Action:** Split files by *what the code does in the system*, never into `utils.go`, `helpers.go`, or `constants.go` catchalls. Keep constants next to the domain functions that use them.
2. **Hard File Limits (Enforced by `internal/archtest`):**
   - `<= 300` lines per production `.go` file (`TestProductionFileSize`).
   - `<= 10` functions/methods per production `.go` file (`TestProductionFuncCount`).
   - Zero `regexp.Compile` / `MustCompile` at package `init()` (`TestNoRegexpCompiledAtInit`).
3. **Layering & Dependency Rule:** Dependencies point inward (`cmd` → `adapters` / `accounting` → `core` / `telemetry` → leaf packages). Adapters hold no routing policy (`TestAdaptersDoNotImportRoutingPolicy`).
4. **Delivery Discipline:** Maximum 5 SP per wave; parallelize only across disjoint files; serialize when sharing files (`internal/telemetry/*`, `func_count_test.go`).

### Wave Plan & Parallelization Matrix (W6–W11)

| Wave | Task | Story Points | Depends On | Parallel Group | Target Files / Packages | Status |
| :---: | :--- | :---: | :---: | :---: | :--- | :---: |
| **W6** | Circuit Breaker fast-path tail read (skip `linkLateUsage` on hook hot path) | 1 SP | — | **Group A** | `internal/telemetry/tail.go`, `internal/telemetry/event_log_read.go`, `cmd/downshift/hook_circuit.go` | Planned |
| **W7** | Per-category lazy regex compilation in `risk.go` (`lazyre` behind `mayMatch`) | 1 SP | — | **Group A** | `internal/classify/risk.go` | Planned |
| **W8** | CCR Anti-Loop 24h TTL & stale `ccr-loop-*.json` pruning | 1 SP | — | **Group A** | `internal/execwrap/antiloop.go`, `internal/execwrap/antiloop_test.go` | Done (#117) |
| **W9** | Consolidate redundant O(N) passes in `telemetry.Aggregate` | 2 SP | W6 | **Group B** | `internal/telemetry/aggregate.go` | Planned |
| **W10** | Zero `funcCountBaseline` (Part 1: 10 files in Core, Adapters, Catalog, Discovery, Paths, Workers) | 3 SP | — | **Group A** | `internal/adapters/{claudecode,codex,kirocrew}`, `internal/catalog`, `internal/core`, `internal/discovery`, `internal/paths`, `internal/workers` | Planned |
| **W11** | Zero `funcCountBaseline` (Part 2: 11 files in Telemetry, Console, ContextOpt, Cmd; empty baseline map) | 3 SP | W9, W10 | **Group B** | `internal/telemetry`, `internal/console`, `internal/contextopt`, `cmd/downshift`, `internal/archtest/func_count_test.go` | Planned |

* **Out of Scope for Phase 2:**
  * Extracting `cmd/downshift/doctor_*.go` (26 files) into `internal/doctor` (deferred to a dedicated structural PR to avoid mixing package moves with W11 function splits).
  * Reading rotated `events.jsonl.*` backups in `stats` / `report` (requires separate memory-cap specification).

---

## Detailed Task Specifications (W6–W11)

### Wave 6 — Circuit Breaker Fast-Path Tail Read (1 SP)
* **Problem:** On every subagent spawn (`cmd/downshift/hookrun.go:143`), `breaker.hold()` calls `telemetry.ReadEventsTail(128 KiB)`. `ReadEventsTail` invokes `parseEvents`, which unmarshals ~500 events and unconditionally runs `linkLateUsage(events)` (3 extra O(N) passes allocating maps to link post-hoc usage), which `openCircuits` never uses.
* **Design (SRP / KISS):**
  - Add `ReadEventsTailUnlinked(maxBytes int64) ([]Event, error)` (or `parseEventsRaw` internal helper in `internal/telemetry/event_log_read.go` + `tail.go`) that parses the tail slice without running `linkLateUsage`.
  - Call `ReadEventsTailUnlinked` from `hookCircuit.state()` in `cmd/downshift/hook_circuit.go`.
* **Guia × Sensor (DoD):**
  - Unit test in `internal/telemetry/` and `cmd/downshift/` verifying circuit breaker trip/close behavior is unchanged while avoiding `linkLateUsage` on the hook path.
  - All files remain `<= 300` lines and `<= 10` functions.

### Wave 7 — Per-Category Lazy Regex in `internal/classify/risk.go` (1 SP)
* **Problem:** `riskSignals` in `internal/classify/risk.go` uses `sync.OnceValue` to compile all `RawRiskSignals` upfront (~545 µs per hook process), even though each `riskSignal` already guards execution with `s.mayMatch(folded)` (literal substring prefilter).
* **Design (KISS / Performance):**
  - Change `riskSignal.re` from `*regexp.Regexp` to `*lazyre.Regexp` (`lazyre.New("(?i)" + r.Pattern)`).
  - Because `s.mayMatch(folded)` runs before `s.re.MatchString(prompt)`, categories whose literal prefilter is `false` never compile their regex during a hook invocation.
  - Ensure `lazyre.CompileAll()` in `internal/classify/lazyre_test.go` still compiles and validates every risk pattern during `go test`.
* **Guia × Sensor (DoD):**
  - `go test ./internal/classify/...` (`risk_scan_equiv_test.go`, `lazyre_test.go`) and `TestNoRegexpCompiledAtInit` pass.

### Wave 8 — CCR Anti-Loop 24h TTL & Stale File Pruning (1 SP)
* **Problem:** `internal/execwrap/antiloop.go` removes `ccr-loop-<ref>.json` on `exitCode == 0`, but sessions that end after a failing command leave small `ccr-loop-*.json` files behind indefinitely.
* **Design (SRP / Clean Code):**
  - Add `antiLoopTTL = 24 * time.Hour` (matching `ccr.Store` TTL).
  - In `readAntiLoopState(path)`: if `time.Since(info.ModTime()) > antiLoopTTL`, remove `path` and return an empty `antiLoopState{}`.
  - In `writeAntiLoopState`: opportunistically remove any sibling `ccr-loop-*.json` or stale `.ccr-loop-*.tmp` older than `antiLoopTTL`.
* **Guia × Sensor (DoD):**
  - Table-driven test in `internal/execwrap/antiloop_test.go` verifying expired loop state resets count to 1 and prunes stale files.

### Wave 9 — `telemetry.Aggregate` Pass Consolidation (2 SP)
* **Problem:** `Aggregate(events)` in `internal/telemetry/aggregate.go` traverses `events` 8 separate times (`indexDecisions`, main loop, `CountInferredHonored`, `tallySpawnSurface`, `ObservedChildTokens`, `RespawnAfterFailure`, `RoutingTokens`, `DuplicateToolUseCount`).
* **Design (SRP / Performance):**
  - Fold the single-pass tallies (`DuplicateToolUseCount` and `CountInferredHonored` / `ObservedChildTokens` where independent) so `Aggregate` avoids redundant full-slice scans while keeping identical results.
* **Guia × Sensor (DoD):**
  - All `internal/telemetry` and `cmd/downshift` parity tests (`accounting_parity_test.go`) pass 100%.

### Wave 10 — Zero `funcCountBaseline` Part 1 (3 SP)
* **Goal:** Split 10 production files by cohesive domain responsibility so each file has `<= 10` functions, and remove them from `funcCountBaseline` in `internal/archtest/func_count_test.go`:

| Current File (Funcs) | Domain A (Retained File) | Domain B (Extracted Cohesive File) |
| :--- | :--- | :--- |
| `internal/adapters/claudecode/event.go` (11) | `event.go` — Hook context & call identifiers (7 funcs) | `event_model.go` — Model, effort & task extraction (4 funcs) |
| `internal/adapters/claudecode/posttooluse.go` (12) | `posttooluse.go` — Child-end & session/model parsing (8 funcs) | `posttooluse_sensor.go` — Context sensor tool observation (4 funcs) |
| `internal/adapters/codex/codex.go` (13) | `codex.go` — Routing handler & wire response (3 funcs) | `event.go` — Codex payload & model/task accessors (10 funcs) |
| `internal/adapters/kirocrew/kirocrew.go` (12) | `kirocrew.go` — Routing handler & policy exit (5 funcs) | `event.go` — KiroCrew payload & tool classification (7 funcs) |
| `internal/catalog/catalog_entries.go` (14) | `catalog_entries.go` — Model & gateway lookup (10 funcs) | `catalog_effort.go` — Reasoning effort capabilities (4 funcs) |
| `internal/core/session_resolver.go` (13) | `session_resolver.go` — Session model resolution & candidates (8 funcs) | `session_resolver_caps.go` — Effort, shadow & explicit-only queries (5 funcs) |
| `internal/discovery/sources.go` (13) | `sources.go` — Registry defaults & Codex cache source (5 funcs) | `sources_cli.go` — Kiro & Grok CLI discovery sources (8 funcs) |
| `internal/paths/paths.go` (11) | `paths.go` — State directory resolution (5 funcs) | `files.go` — Named state artifact paths (6 funcs) |
| `internal/workers/write.go` (14) | `write.go` — Launcher registry & direct execution (5 funcs) | `worktree_launcher.go` — Git worktree isolation & cleanup (9 funcs) |
| `internal/console/shared/diffview/diffview.go` (11) | `diffview.go` — Diff state, file switching & scroll (9 funcs) | `render.go` — ANSI diff view rendering (2 funcs) |

* **Guia × Sensor (DoD):**
  - Every split file is named after its domain action (never `utils.go` or `helpers.go`).
  - `TestProductionFuncCount` and `TestProductionFileSize` in `internal/archtest` pass with those 10 entries removed.

### Wave 11 — Zero `funcCountBaseline` Part 2 (3 SP)
* **Goal:** Split the remaining 11 production files by cohesive domain responsibility so `funcCountBaseline` in `internal/archtest/func_count_test.go` is **completely empty (`map[string]int{}`)**:

| Current File (Funcs) | Domain A (Retained File) | Domain B (Extracted Cohesive File) |
| :--- | :--- | :--- |
| `cmd/downshift/apply_core.go` (12) | `apply_core.go` — Spawn decision orchestration (7 funcs) | `apply_result.go` — Input validation & result mapping (5 funcs) |
| `cmd/downshift/doctor_checks.go` (11) | `doctor_checks.go` — Binary, state & catalog checks (8 funcs) | `doctor_context_checks.go` — Context, companion & override checks (3 funcs) |
| `cmd/downshift/doctor_plugin_hooks.go` (11) | `doctor_plugin_hooks.go` — Installed plugin inspection (5 funcs) | `doctor_plugin_hooks_diff.go` — Hook comparison & diff formatting (6 funcs) |
| `internal/console/app.go` (13) | `app.go` — TUI lifecycle, routing & view (8 funcs) | `app_exec.go` — Async command runner & spinner (5 funcs) |
| `internal/console/reportview/model.go` (14) | `model.go` — Tab & scroll state machine (10 funcs) | `view.go` — Key dispatch & table rendering (4 funcs) |
| `internal/contextopt/providers/headroom/headroom.go` (15) | `headroom.go` — Provider identity, capabilities & detection (9 funcs) | `lifecycle.go` — Health, configuration & metrics (6 funcs) |
| `internal/contextopt/providers/native/native.go` (14) | `native.go` — Provider identity, capabilities & detection (8 funcs) | `lifecycle.go` — Health, configuration & metrics (6 funcs) |
| `internal/telemetry/aggregate.go` (13) | `aggregate.go` — Event pass & decision/usage indexing (8 funcs) | `aggregate_counts.go` — Dimension counters: agent, model, gateway, verdict (5 funcs) |
| `internal/telemetry/panel.go` (12) | `panel.go` — Panel builder & observed model matching (8 funcs) | `panel_lines.go` — Token & agent-type line formatting (4 funcs) |
| `internal/telemetry/routing_tokens.go` (11) | `routing_tokens.go` — Routing token accumulation (8 funcs) | `routing_tokens_format.go` — Summary line formatting & harness order (3 funcs) |
| `internal/telemetry/spawn_state.go` (14) | `spawn_state.go` — Pure spawn state classification & line (9 funcs) | `live_state.go` — `live.json` atomic I/O & session activity line (5 funcs) |

* **Guia × Sensor (DoD):**
  - `funcCountBaseline` in `internal/archtest/func_count_test.go` is empty (`var funcCountBaseline = map[string]int{}`).
  - `go test ./internal/archtest/...` and `make check` pass 100%.

---

## Phase 1 Wave Details & Acceptance Criteria (Completed)

### Wave 1 — `ccr.Redact` Fast-Path Prefilters (1 SP)
* **Problem:** `redact()` splits input by `\n` and executes 15 `(?i)` regexes per line unconditionally, plus double-scans (`.Match` followed by `.ReplaceAll`). Measured: **1.74s** on 2.1 MB (40k lines of `go test` output), risking timeouts on larger logs up to 16–32 MiB.
* **Fix:** Add a fast ASCII case-insensitive literal prefilter per line (checking for credential/PEM/AWS markers before invoking regexes) and merge `.Match` + `.ReplaceAll` into single calls where appropriate.
* **Acceptance:** `go test ./internal/ccr/...` passes 100%; `BenchmarkRedact` demonstrates > 50x speedup on 2 MB typical test output.

### Wave 2 — `classify` Cold-Start & Hot-Loop Optimization (2 SP)
* **Problem:** Each hook is a fresh OS process, so `sync.OnceValue` cold-start pays ~10.8 ms: `parseDomainLexicon` runs `regexp.Compile` on every static embedded JSON entry and discards the result; `Classify` runs `MatchString` then `FindAllStringIndex` (2x regex scans per matched signal).
* **Fix:** Keep JSON lexicon regex validation in `domain_lexicon_test.go` (and out of runtime hot path), and call `FindAllStringIndex` once in `Classify`.
* **Acceptance:** All `internal/classify` and `internal/core` tests pass; `make check` green.

### Wave 3 — `claudecode` `SubagentStop` Single-Pass Transcript Read (2 SP)
* **Problem:** `readSettledSubagentUsage` re-parses the full JSONL transcript from byte 0 on every 25ms growth tick during `settleWindow`, and `parseSubagentStop` immediately opens and parses the same up-to-32 MiB file a second time for `readSubagentVerification`.
* **Fix:** Wait for size stability (`quiet` / `deadline`) before parsing the settled transcript, and extract both `subagentUsage` and `Verification` in a single pass.
* **Acceptance:** All `internal/adapters/claudecode` tests (including `transcript_settle_test.go` and `subagentstop_tail_test.go`) pass.

### Wave 4 — Targeted Format Detection (`execwrap` → `compressor`) (2 SP)
* **Problem:** `execwrap` already knows `expectedFormats[family]`, yet `compactSafely` runs `compressor.CompressExit` which tests 30+ unrelated format detectors sequentially.
* **Fix:** Expose `CompressExitExpected(input []byte, mode Mode, exitCode int, expected ...Format) Result` in `internal/compressor` to check expected format(s) first (falling back identically when empty/unmatched).
* **Acceptance:** `internal/compressor` and `internal/execwrap` tests pass with identical compaction results.

### Wave 5 — Automated Adapter Import Boundary in `archtest` (2 SP)
* **Problem:** `internal/archtest/imports_test.go` enforces `core`, `session`, `fuse`, `telemetry`, `execwrap`, and `accounting` boundaries, but adapter isolation from `routing/policy` and `routing/decide` is still manual.
* **Fix:** Add `TestAdapterDirectImports` / rules in `internal/archtest/imports_test.go` asserting no `internal/adapters/*` package imports `internal/routing/policy` or `internal/routing/decide`.
* **Acceptance:** `go test ./internal/archtest/...` and `make check` pass.
