# Frozen packages

Downshift by Tiago Vilas Boas — https://github.com/tiagovilasboas/downshift-ai

The owner decided on 2026-11-20 to freeze these packages so that work goes to the
paths that already show measured value: routing, compaction and outcome. Frozen
packages and commands stay in the binary and keep working.

## What frozen means

- **Allowed:** security fixes, bug fixes, removals, and changes another package
  forces (a renamed type, a moved helper).
- **Not allowed:** new features, new flags, new commands, and new callers from
  outside the package.
- **To unfreeze:** the owner records the decision and the usage data behind it,
  then removes the row from this table.

## The list

| Package | What it does | Who calls it today | Note |
|---|---|---|---|
| `internal/loop` | Batch runner and acceptance report for `downshift loop` | `cmd/downshift/loop*.go` | The report's acceptance text needs a fix (roadmap F4). |
| `internal/cascade` | Sensor-guided upgrade and the dead-end record | `cmd/downshift/exec.go`, `internal/loop` | Dead-end fingerprints use a per-install salted HMAC since #176. |
| `internal/contextopt`, `context` command surface | `downshift context`, `context enable`, `context compress` | `cmd/downshift/context*.go` | Freeze covers only the command surface. Balanced config, the CCR keep path and the post-run sensor still use the package and are not frozen. |
| `internal/gateway` | Router-agnostic multi-provider gateway | `cmd/downshift/cmd_gateway*.go`, `cmd_plan.go`, `doctor*.go` | Roadmap F2 marks `gateway` as `Experimental` in clispec. |
| `internal/missedsavings` | Scans harness transcripts for commands that could have been compacted | `cmd/downshift/cmd_discover.go` | Roadmap F2 marks `discover` as `Experimental`. |
| `internal/operatorexport` | Builds the artefact an outside operator shares | `cmd/downshift/export.go` | |
| `internal/cursorbridge` | Cursor extension and its packaging script; it contains no Go code | none in Go | Moves out of `internal/` in its own PR. |

## Enforcement

Today this list is a review rule. Roadmap F3 adds an archtest check that fails
when new code imports a frozen package from a new caller without an exception.
