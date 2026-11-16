<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Performance & Architecture Hardening Roadmap

Downshift by Tiago Vilas Boas
https://github.com/tiagovilasboas/downshift-ai

**Branch:** `perf/arch-hardening-waves`
**Status:** Active (wave-by-wave delivery)
**Scope:** Hot-path performance bottlenecks, cold-start regex cost, redundant transcript I/O, targeted format detection, and automated `archtest` layer enforcement.

---

## Wave Plan

| Wave | Task | Story Points | Target Packages | Status |
| :---: | :--- | :---: | :--- | :---: |
| **W0** | Map roadmap and wave tasks | 1 SP | `docs/product/` | In progress |
| **W1** | `ccr.Redact` fast-path literal prefilters & single-pass replacement | 1 SP | `internal/ccr` | Planned |
| **W2** | `classify` cold-start & hot-loop optimization (skip runtime embed re-validation, single `FindAllStringIndex` pass) | 2 SP | `internal/classify` | Planned |
| **W3** | `claudecode` `SubagentStop` settle-before-parse & single-pass transcript read (usage + verification) | 2 SP | `internal/adapters/claudecode` | Planned |
| **W4** | `execwrap` → `compressor` targeted format detection (`expectedFormats` hint) | 2 SP | `internal/compressor`, `internal/execwrap` | Planned |
| **W5** | Enforce adapter boundary in `archtest/imports_test.go` (forbid `routing/policy` and `routing/decide`) | 2 SP | `internal/archtest` | Planned |

---

## Wave Details & Acceptance Criteria

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
