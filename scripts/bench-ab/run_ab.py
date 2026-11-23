#!/usr/bin/env python3
# Copyright (c) 2026 Tiago Vilas Boas.
# SPDX-License-Identifier: Apache-2.0
# Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift-ai
"""Reproducible paired A/B harness (Off vs Downshift) on refactor/debug tasks.

Supports `--dry-run` (zero API spend: prepares temp workspaces, confirms initial
baseline fails hidden checks, and prints exact commands) and live runs against
`claude` or `codex` CLIs in isolated temporary state directories.
"""

from __future__ import annotations

import argparse
import json
import os
import shutil
import subprocess
import sys
import tempfile

REPO = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
TASKS_FILE = os.path.join(os.path.dirname(os.path.abspath(__file__)), "tasks.json")
SCRUBBED_ENV_PREFIXES = ("DOWNSHIFT_", "CLAUDECODE", "CLAUDE_CODE_", "CODEX_")


def load_tasks(task_filter: str) -> list[dict]:
    with open(TASKS_FILE, encoding="utf-8") as fh:
        tasks = json.load(fh)["tasks"]
    if task_filter == "all":
        return tasks
    selected = [t for t in tasks if t["id"] == task_filter]
    if not selected:
        raise ValueError(f"unknown task {task_filter!r}; available: {[t['id'] for t in tasks]}")
    return selected


def prepare_workspace(task: dict, root: str, include_hidden: bool = True) -> str:
    ws = os.path.join(root, "ws")
    os.makedirs(ws, exist_ok=True)
    src_dir = os.path.join(REPO, task["source_dir"])
    files = list(task["visible_files"])
    if include_hidden:
        files += [f for f in task["hidden_test_files"] if f not in files]
    for rel in files:
        shutil.copy2(os.path.join(src_dir, rel), os.path.join(ws, rel))
    if task.get("need_go_mod") and not os.path.exists(os.path.join(ws, "go.mod")):
        with open(os.path.join(ws, "go.mod"), "w", encoding="utf-8") as fh:
            fh.write(task["need_go_mod"])
    return ws


def write_claude_hooks_settings(path: str, bin_path: str) -> None:
    cfg = {
        "hooks": {
            "PreToolUse": [
                {"matcher": "Agent|Task", "hooks": [{"type": "command", "command": f"{bin_path} claude-code"}]},
                {"matcher": "Bash", "hooks": [{"type": "command", "command": f"{bin_path} claude-code"}]},
            ],
            "PostToolUse": [
                {"matcher": "Bash", "hooks": [{"type": "command", "command": f"{bin_path} claude-code-post-tool-use"}]},
                {"matcher": "Agent|Task", "hooks": [{"type": "command", "command": f"{bin_path} claude-code-post-tool-use"}]},
            ],
            "SubagentStop": [
                {"hooks": [{"type": "command", "command": f"{bin_path} claude-code-subagent-stop"}]}
            ],
        }
    }
    with open(path, "w", encoding="utf-8") as fh:
        json.dump(cfg, fh, indent=2)


def build_cli_command(harness: str, arm: str, prompt: str, settings_path: str, model: str) -> list[str]:
    if harness == "claude":
        cmd = ["claude", "-p", prompt, "--output-format", "json", "--permission-mode", "bypassPermissions"]
        if model:
            cmd += ["--model", model]
        if arm == "off":
            cmd += ["--disable-all-hooks"]
        else:
            cmd += ["--settings", settings_path]
        return cmd
    if harness == "codex":
        cmd = ["codex", "exec", "--json"]
        if model:
            cmd += ["--model", model]
        if arm == "off":
            cmd += ["-c", "features.hooks=false"]
        cmd.append(prompt)
        return cmd
    raise ValueError(f"unsupported harness {harness!r}")


def extract_usage(raw_stdout: str) -> dict:
    """Extracts usage token counters from Claude Code or Codex JSON output."""
    usage = {"input": 0, "output": 0, "cache_read": 0, "cache_write": 0}
    for line in raw_stdout.splitlines():
        line = line.strip()
        if not line.startswith("{"):
            continue
        try:
            obj = json.loads(line)
        except json.JSONDecodeError:
            continue
        candidates = []
        if isinstance(obj.get("usage"), dict):
            candidates.append(obj["usage"])
        if isinstance(obj.get("modelUsage"), dict):
            candidates.extend(v for v in obj["modelUsage"].values() if isinstance(v, dict))
        for u in candidates:
            usage["input"] += int(u.get("input_tokens") or u.get("inputTokens") or 0)
            usage["output"] += int(u.get("output_tokens") or u.get("outputTokens") or 0)
            usage["cache_read"] += int(u.get("cache_read_input_tokens") or u.get("cacheReadInputTokens") or 0)
            usage["cache_write"] += int(u.get("cache_creation_input_tokens") or u.get("cacheCreationInputTokens") or 0)
    return usage


def run_judge(ws: str, task: dict) -> bool:
    src_dir = os.path.join(REPO, task["source_dir"])
    for rel in task["hidden_test_files"]:
        shutil.copy2(os.path.join(src_dir, rel), os.path.join(ws, rel))
    p = subprocess.run(["go", "test", "./..."], cwd=ws, capture_output=True, text=True, timeout=60)
    return p.returncode == 0


def run_pair(
    tasks: list[dict],
    harness: str,
    reps: int,
    bin_path: str,
    model: str,
    dry_run: bool,
) -> list[dict]:
    records = []
    for rep in range(1, reps + 1):
        for task in tasks:
            for arm in ("off", "on"):
                tmp = tempfile.mkdtemp(prefix=f"ds-ab-{task['id']}-{arm}-")
                try:
                    ws = prepare_workspace(task, tmp, include_hidden=False)
                    baseline_passed = run_judge(prepare_workspace(task, os.path.join(tmp, "base"), include_hidden=True), task)
                    if baseline_passed:
                        raise RuntimeError(f"task {task['id']} unexpectedly passes before edits")
                    state_dir = os.path.join(tmp, "state")
                    os.makedirs(state_dir, exist_ok=True)
                    settings_path = os.path.join(tmp, "claude-hooks.json")
                    write_claude_hooks_settings(settings_path, bin_path)
                    cmd = build_cli_command(harness, arm, task["prompt"], settings_path, model)

                    if dry_run:
                        records.append(
                            {
                                "task": task["id"],
                                "arm": arm,
                                "rep": rep,
                                "harness": harness,
                                "dry_run": True,
                                "baseline_fails": not baseline_passed,
                                "command": cmd,
                                "usage": {"input": 0, "output": 0, "cache_read": 0, "cache_write": 0},
                                "accepted": None,
                                "compactor_cut_bytes": 0,
                                "retrieved_bytes": 0,
                            }
                        )
                        continue

                    env = {k: v for k, v in os.environ.items() if not k.startswith(SCRUBBED_ENV_PREFIXES)}
                    env["DOWNSHIFT_STATE_DIR"] = state_dir
                    p = subprocess.run(cmd, cwd=ws, capture_output=True, text=True, env=env, timeout=300)
                    accepted = run_judge(ws, task)
                    usage = extract_usage(p.stdout)
                    cut_bytes, retrieved_bytes = 0, 0
                    if arm == "on" and os.path.isfile(bin_path):
                        rp = subprocess.run(
                            [bin_path, "report", "--days=0", "--json"],
                            capture_output=True,
                            text=True,
                            env=env,
                            timeout=30,
                        )
                        if rp.returncode == 0 and rp.stdout.strip():
                            comp = (json.loads(rp.stdout).get("numbers") or {}).get("compactor") or {}
                            cut_bytes = int(comp.get("saved_bytes", 0))
                            retrieved_bytes = int(comp.get("retrieved_bytes", 0))

                    records.append(
                        {
                            "task": task["id"],
                            "arm": arm,
                            "rep": rep,
                            "harness": harness,
                            "model": model or "default",
                            "dry_run": False,
                            "accepted": accepted,
                            "usage": usage,
                            "compactor_cut_bytes": cut_bytes,
                            "retrieved_bytes": retrieved_bytes,
                        }
                    )
                finally:
                    shutil.rmtree(tmp, ignore_errors=True)
    return records


def main(argv: list[str] | None = None) -> int:
    ap = argparse.ArgumentParser(description="Paired A/B harness (Off vs Downshift) for coding agents.")
    ap.add_argument("--harness", required=True, choices=("claude", "codex"))
    ap.add_argument("--task", default="all", help="all, refactor-clamp, or debug-reverse")
    ap.add_argument("--reps", type=int, default=1)
    ap.add_argument("--model", default="", help="Optional model override")
    ap.add_argument("--bin", default=os.path.join(REPO, "downshift"))
    ap.add_argument("--dry-run", action="store_true", help="Prepare workspaces and verify baseline fails without calling CLI")
    ap.add_argument("--out", default="", help="Write JSON records to file")
    args = ap.parse_args(argv)

    records = run_pair(
        load_tasks(args.task),
        harness=args.harness,
        reps=max(1, args.reps),
        bin_path=os.path.abspath(args.bin),
        model=args.model,
        dry_run=args.dry_run,
    )
    payload = json.dumps(records, indent=2)
    if args.out:
        with open(args.out, "w", encoding="utf-8") as fh:
            fh.write(payload + "\n")
    print(payload)
    return 0


if __name__ == "__main__":
    sys.exit(main())
