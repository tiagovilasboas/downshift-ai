# Skills shipped with Downshift

Seven skills, seven jobs, no overlap. They are plain `SKILL.md` files any
harness can read; the Claude Code plugin ships them as `/downshift:<name>`.

| Skill | Job | Writes? | When to stop using it |
|---|---|---|---|
| `downshift-setup` | install, repair and first validation of the hooks; dry-run before any write, backups and `--undo` | yes, after a yes | once `downshift doctor` is clean; a hand-copied install can delete it |
| `downshift-audit` | independent, read-only inspection: is it wired, what did it do, what loads every session | never | keep; it is the second opinion |
| `harness-engineer` | six steps: restate, split by story points, parallelize, keep small work local, leave mechanical spawns unpinned, check with `session --why` and `report` | never | keep while you delegate work |
| `downshift-tune` | the one entry point for "we spend too many tokens": fixed diagnosis order, biggest lever first, one change at a time | only after a yes | keep; it defers to setup when nothing is wired |
| `downshift-squad` | invoke specialized agent squads for coordinated multi-agent work: `squad coder "task"`, `squad core "task"`, `squad downshift "full project"` | never | keep; the orchestration layer |
| `ponytail` | lazy senior dev mode: smallest change that fully solves the task, delete > add, no abstraction for later | never | keep; the default coding philosophy |
| `web-motion` | playbook for smooth web animation on the compositor, including the agent-swarm build effect; used by `coder-front` | never | keep for frontend motion work; plugin only, see below |

`downshift-effort` is an advisory skill: it tells the orchestrator how much
reasoning and verification a task's stakes deserve (mechanical, ordinary, high) and
never sets an effort value itself. Setup installs it next to `downshift-orchestrator`. It also installs `ponytail` and `downshift-squad` when the plugin lacks them.
Inspired by an MIT-licensed skill, github.com/mmc7676/skills.

`web-motion` ships through the Claude Code plugin only. It is not in the `go:embed` list of `embed.go`, so `downshift setup` never writes a web-animation skill into a harness that has no use for it. Other harnesses can copy the folder by hand. It credits two MIT-licensed skills as inspiration (`emilkowalski/skills`, `mblode/agent-skills`); the text is original.

## What they spend in tokens

A skill is not free: its `description` is loaded into every session, and its
body when it triggers. Measured on 2026-10-17 as bytes / 4 (an estimate, not a
tokenizer):

| File | Description, always loaded | Body, loaded on use |
|---|---|---|
| `ponytail/SKILL.md` | ~45 tokens | ~620 tokens |
| `downshift-tune/SKILL.md` | ~86 tokens | ~750 tokens, plus the one reference file it opens (~400 each) |
| `downshift-audit/SKILL.md` | ~70 tokens | ~890 tokens |
| `downshift-setup/SKILL.md` | ~66 tokens | ~730 tokens |
| `harness-engineer/SKILL.md` (2026-10-21) | ~50 tokens | ~170 tokens |
| `web-motion/SKILL.md` (2026-11-18) | ~40 tokens | ~2,000 tokens |
| `agents/mechanical-worker.md` | ~55 tokens | ~270 tokens |
| `commands/status.md` | ~16 tokens | ~90 tokens |

About 430 tokens per session for the whole set when installed through the
plugin. A skill reduces nothing just by recommending a reduction; it pays off
only when its advice is applied. Descriptions therefore trigger only when the
user names Downshift or asks about its hooks and token use (the one exception is
`web-motion`, which triggers on web animation work), not on any mention
of `CLAUDE.md`.

## Delegation is not free either

`agents/mechanical-worker.md` is an example of a bounded, unpinned subagent
that Downshift can right-size. It is not a recommendation to delegate every
small task: a spawn pays for its own context, the brief, the return and the
parent's coordination, and a one-line change is usually cheaper done by the
main agent. Delegate when the work is self-contained and its output is short;
then let the router pick the tier. See [docs/guides/subagents.md](../docs/guides/subagents.md).

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift-ai
