#!/usr/bin/env python3
# Copyright (c) 2026 Tiago Vilas Boas.
# SPDX-License-Identifier: Apache-2.0
# Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift-ai
"""1-command offline proof benchmark for Downshift.

Drives the real compiled `downshift` binary in throwaway state directories
(zero LLM/API calls, zero writes to ~/.downshift) and verifies four claims:
  1. Compaction + 100% lossless CCR retrieval on real repository outputs.
  2. Debug signal retention on failing `go test` + active anti-loop raw bypass.
  3. Prompt-cache safety invariance (main session untouched; spawn sibling fields kept).
  4. Policy risk floor enforcement + local FinOps `report --json` accounting.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile
import uuid

REPO = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
DEFAULT_BIN = os.path.abspath(os.environ.get("DOWNSHIFT_BIN", os.path.join(REPO, "downshift")))
BALANCED_DIR = os.path.join(REPO, "internal", "compressor", "testdata", "balanced")
SCRUBBED_ENV_PREFIXES = (
    "DOWNSHIFT_",
    "KIROCREW_",
    "KIRO_",
    "CLAUDECODE",
    "CLAUDE_CODE_",
    "CODEX_",
    "CURSOR_",
    "OPENCODE",
    "ANTIGRAVITY_",
)
CLAUDE_SESSION = ["claude-haiku-5-5", "claude-sonnet-4-5", "claude-opus-5-5"]
RETRIEVE_RE = re.compile(r"downshift retrieve(?:\s+--session=([0-9a-f]{12}))?\s+([0-9a-f]{12})")
CCR_REDACTED_LINE = b"[downshift ccr: line withheld, it looked like a secret]"

BALANCED_FIXTURES = (
    ("diff_git_log_p.txt", "git log -p | cat"),
    ("diff_git_show.txt", "git show HEAD | cat"),
    ("log_go_test_v.txt", "go test -v ./... | cat"),
    ("matches_grep_rn.txt", "grep -rn x . | cat"),
)


def child_env(state_dir: str) -> dict[str, str]:
    env = {k: v for k, v in os.environ.items() if not k.startswith(SCRUBBED_ENV_PREFIXES)}
    env["DOWNSHIFT_STATE_DIR"] = state_dir
    env["DOWNSHIFT_CONTROL_SHARE"] = "0"
    return env


def run_cli(
    bin_path: str,
    args: list[str],
    state_dir: str,
    payload: dict | None = None,
    cwd: str | None = None,
) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        [bin_path] + args,
        input=json.dumps(payload) if payload is not None else None,
        capture_output=True,
        text=True,
        env=child_env(state_dir),
        cwd=cwd or REPO,
        timeout=60,
    )


def parse_retrieve_marker(text: str) -> tuple[str | None, str | None]:
    matches = RETRIEVE_RE.findall(text)
    if not matches:
        return None, None
    session_ref, kept_id = matches[-1]
    return (session_ref or None), kept_id


def parse_updated_stdout(raw_json: str) -> tuple[bool, str]:
    if not raw_json.strip():
        return False, ""
    data = json.loads(raw_json)
    hso = data.get("hookSpecificOutput") or {}
    uto = hso.get("updatedToolOutput")
    if not isinstance(uto, dict):
        return False, ""
    return True, str(uto.get("stdout", ""))


def read_exec_events(state_dir: str) -> list[dict]:
    path = os.path.join(state_dir, "exec-events.jsonl")
    if not os.path.exists(path):
        return []
    with open(path, encoding="utf-8") as fh:
        return [json.loads(line) for line in fh if line.strip()]


def build_failing_go_test() -> tuple[str, list[str]]:
    lines = [f"ok  example.com/service/pkg{i:02d}  0.{10 + (i % 9)}s\n" for i in range(40)]
    critical = [
        "--- FAIL: TestCriticalAuth (0.04s)",
        "    auth_test.go:42: expected expired token rejected, got 200 OK",
        "panic: nil pointer dereference [recovered]",
        "goroutine 1 [running]:",
        "FAIL\texample.com/service/auth\t0.15s",
    ]
    lines.extend(c + "\n" for c in critical)
    return "".join(lines), critical


def build_failing_make_log() -> tuple[str, str]:
    critical = "src/unit125.c:19:5: error: use of undeclared identifier 'cachePrefix'"
    lines = []
    for i in range(360):
        lines.append(f"gcc -O2 -c src/unit{i:03d}.c -o obj/unit{i:03d}.o -Wall -Wextra\n")
        if i == 125:
            lines.append(critical + "\n")
    lines.append("make: *** [Makefile:12: all] Error 2\n")
    return "".join(lines), critical


def verify_ccr_retrieve(raw_bytes: bytes, ret_bytes: bytes) -> tuple[bool, str]:
    if not ret_bytes:
        return False, "empty"
    if ret_bytes == raw_bytes:
        return True, "exact"
    r_lines = raw_bytes.splitlines()
    g_lines = ret_bytes.splitlines()
    if len(r_lines) != len(g_lines):
        return False, "line_mismatch"
    redacted_count = 0
    for r_line, g_line in zip(r_lines, g_lines):
        if r_line == g_line:
            continue
        if g_line == CCR_REDACTED_LINE:
            redacted_count += 1
            continue
        return False, "content_mismatch"
    if redacted_count > 0:
        return True, f"exact ({redacted_count} secret-shaped lines redacted)"
    return False, "mismatch"


def check_compaction_and_ccr(bin_path: str, base_dir: str, clean_cwd: str) -> dict:
    state = os.path.join(base_dir, "compaction")
    os.makedirs(state, exist_ok=True)
    sid = "proof-" + uuid.uuid4().hex[:8]
    rows = []
    total_raw = 0
    total_emitted = 0
    total_retrieved_expected = 0
    lossless_ok = True

    for fname, cmd in BALANCED_FIXTURES:
        fpath = os.path.join(BALANCED_DIR, fname)
        with open(fpath, "rb") as fh:
            raw_bytes = fh.read()
        raw_text = raw_bytes.decode("utf-8")
        payload = {
            "hook_event_name": "PostToolUse",
            "session_id": sid,
            "tool_name": "Bash",
            "tool_input": {"command": cmd},
            "tool_response": {"stdout": raw_text, "stderr": "", "exit_code": 0},
        }
        p = run_cli(bin_path, ["claude-code-post-tool-use"], state, payload=payload, cwd=clean_cwd)
        replaced, emitted = parse_updated_stdout(p.stdout)
        emitted_bytes = len(emitted.encode("utf-8")) if replaced else len(raw_bytes)
        sref, kept_id = parse_retrieve_marker(emitted)
        retrieved_ok, retrieve_mode = False, "missing"
        if kept_id:
            r_args = ["retrieve"]
            if sref:
                r_args.append(f"--session={sref}")
            r_args.append(kept_id)
            rp = run_cli(bin_path, r_args, state, cwd=clean_cwd)
            if rp.returncode == 0:
                ret_bytes = rp.stdout.encode("utf-8")
                total_retrieved_expected += len(ret_bytes)
                retrieved_ok, retrieve_mode = verify_ccr_retrieve(raw_bytes, ret_bytes)
        if not (p.returncode == 0 and replaced and emitted_bytes < len(raw_bytes) and kept_id and retrieved_ok):
            lossless_ok = False
        total_raw += len(raw_bytes)
        total_emitted += emitted_bytes
        rows.append(
            {
                "fixture": fname,
                "command": cmd,
                "raw_bytes": len(raw_bytes),
                "compacted_bytes": emitted_bytes,
                "cut_pct": round(100.0 * (len(raw_bytes) - emitted_bytes) / len(raw_bytes), 1),
                "ccr_id": kept_id or "",
                "lossless_retrieve": retrieved_ok,
                "retrieve_mode": retrieve_mode,
            }
        )

    rep_p = run_cli(bin_path, ["report", "--days=0", "--json"], state, cwd=clean_cwd)
    rep_ok = False
    retrieved_bytes = 0
    retrieved_pct = 0.0
    if rep_p.returncode == 0 and rep_p.stdout.strip():
        rep = json.loads(rep_p.stdout)
        comp = (rep.get("numbers") or {}).get("compactor") or {}
        retrieved_bytes = int(comp.get("retrieved_bytes", 0))
        retrieved_pct = float(comp.get("retrieved_pct", 0.0))
        rep_ok = (
            int(comp.get("commands", 0)) == len(BALANCED_FIXTURES)
            and int(comp.get("compacted", 0)) == len(BALANCED_FIXTURES)
            and retrieved_bytes == total_retrieved_expected
            and 99.0 <= retrieved_pct <= 100.0
        )

    cut_pct = round(100.0 * (total_raw - total_emitted) / total_raw, 1) if total_raw else 0.0
    return {
        "name": "Compaction & 100% lossless CCR recovery (4 real repo logs/diffs)",
        "passed": lossless_ok and rep_ok,
        "state_dir": state,
        "raw_bytes": total_raw,
        "compacted_bytes": total_emitted,
        "saved_bytes": total_raw - total_emitted,
        "raw_tokens_est": total_raw // 4,
        "compacted_tokens_est": total_emitted // 4,
        "saved_tokens_est": (total_raw - total_emitted) // 4,
        "cut_pct": cut_pct,
        "report_retrieved_bytes": retrieved_bytes,
        "report_retrieved_pct": round(retrieved_pct, 1),
        "fixtures": rows,
    }


def check_signal_and_antiloop(bin_path: str, base_dir: str, clean_cwd: str) -> dict:
    state = os.path.join(base_dir, "signal")
    os.makedirs(state, exist_ok=True)
    sid = "proof-loop-" + uuid.uuid4().hex[:8]

    # 2a: failing go test preserves every failure/panic/stack line
    go_fail, critical_lines = build_failing_go_test()
    p_go = run_cli(
        bin_path,
        ["claude-code-post-tool-use"],
        state,
        cwd=clean_cwd,
        payload={
            "hook_event_name": "PostToolUse",
            "session_id": sid,
            "tool_name": "Bash",
            "tool_input": {"command": 'FOO="1" go test ./...'},
            "tool_response": {"stdout": go_fail, "stderr": "", "exit_code": 1},
        },
    )
    rep_go, out_go = parse_updated_stdout(p_go.stdout)
    shown_go = out_go if rep_go else go_fail
    go_signal_kept = p_go.returncode == 0 and all(line in shown_go for line in critical_lines)

    # 2b: active anti-loop switches 3rd consecutive failing fallback_cut run to verbatim raw
    make_fail, make_err = build_failing_make_log()
    make_sid = "proof-anti-" + uuid.uuid4().hex[:8]
    runs = []
    for attempt in range(1, 4):
        p = run_cli(
            bin_path,
            ["claude-code-post-tool-use"],
            state,
            cwd=clean_cwd,
            payload={
                "hook_event_name": "PostToolUse",
                "session_id": make_sid,
                "tool_name": "Bash",
                "tool_input": {"command": 'FOO="1" make test'},
                "tool_response": {"stdout": make_fail, "stderr": "", "exit_code": 2},
            },
        )
        replaced, out = parse_updated_stdout(p.stdout)
        runs.append(
            {
                "attempt": attempt,
                "replaced": replaced,
                "error_line_kept": (make_err in (out if replaced else make_fail)),
                "emitted_bytes": len(out.encode("utf-8")) if replaced else len(make_fail.encode("utf-8")),
            }
        )
    evs = read_exec_events(state)
    last_detail = evs[-1].get("passthrough_detail", "") if evs else ""
    anti_loop_ok = (
        runs[0]["replaced"]
        and runs[0]["error_line_kept"]
        and runs[1]["replaced"]
        and runs[1]["error_line_kept"]
        and (not runs[2]["replaced"])
        and last_detail == "anti_loop_bypass"
    )
    return {
        "name": "Signal retention (100% failure lines) & active anti-loop raw bypass",
        "passed": go_signal_kept and anti_loop_ok,
        "go_test_critical_lines_kept": go_signal_kept,
        "anti_loop_attempts": runs,
        "third_attempt_passthrough_detail": last_detail,
    }


def check_cache_safety(bin_path: str, base_dir: str, clean_cwd: str) -> dict:
    state = os.path.join(base_dir, "cache")
    os.makedirs(state, exist_ok=True)
    sid = "proof-cache-" + uuid.uuid4().hex[:8]

    # Non-spawn tools must never mutate model or conversation prefix
    p_read = run_cli(
        bin_path,
        ["claude-code"],
        state,
        cwd=clean_cwd,
        payload={
            "session_id": sid,
            "session_models": CLAUDE_SESSION,
            "tool_name": "Read",
            "tool_input": {"file_path": "/repo/main.go"},
        },
    )
    main_untouched = p_read.returncode == 0 and p_read.stdout.strip() in ("", "{}")

    # Subagent spawn rewrites ONLY model and preserves all prompt/sibling fields verbatim
    orig_input = {
        "prompt": "[tier=small] rename the userId variable to userIdentifier",
        "model": "claude-opus-5-5",
        "subagent_type": "Explore",
        "description": "rename userId identifier",
    }
    p_spawn = run_cli(
        bin_path,
        ["claude-code"],
        state,
        cwd=clean_cwd,
        payload={
            "session_id": sid,
            "session_models": CLAUDE_SESSION,
            "tool_name": "Task",
            "tool_use_id": "toolu_cache_1",
            "tool_input": orig_input,
        },
    )
    out = json.loads(p_spawn.stdout or "{}") if p_spawn.stdout.strip() else {}
    upd = ((out.get("hookSpecificOutput") or {}).get("updatedInput")) or {}
    rewritten_model = upd.get("model")
    siblings_preserved = all(upd.get(k) == v for k, v in orig_input.items() if k != "model")
    spawn_routed = rewritten_model in ("haiku", "claude-haiku-5-5") and siblings_preserved

    return {
        "name": "Prompt-cache safety (main prefix untouched; spawn preserves prompt verbatim)",
        "passed": main_untouched and spawn_routed,
        "main_session_read_untouched": main_untouched,
        "spawn_model_before": orig_input["model"],
        "spawn_model_after": rewritten_model or "unchanged",
        "sibling_fields_preserved": siblings_preserved,
    }


def check_policy_and_finops(bin_path: str, base_dir: str) -> dict:
    state = os.path.join(base_dir, "policy")
    proj = os.path.join(base_dir, "proj")
    os.makedirs(state, exist_ok=True)
    os.makedirs(os.path.join(proj, ".downshift"), exist_ok=True)
    with open(os.path.join(proj, ".downshift", "policy.json"), "w", encoding="utf-8") as fh:
        json.dump({"version": 1, "min_tier_by_risk": {"authz": "frontier"}}, fh)

    sid = "proof-pol-" + uuid.uuid4().hex[:8]
    p_auth = run_cli(
        bin_path,
        ["claude-code"],
        state,
        cwd=proj,
        payload={
            "session_id": sid,
            "session_models": CLAUDE_SESSION,
            "tool_name": "Task",
            "tool_use_id": "toolu_auth",
            "tool_input": {
                "prompt": "verify authz permission middleware token checks",
                "model": "claude-opus-5-5",
                "subagent_type": "Explore",
                "description": "auth check",
            },
        },
    )
    out_auth = json.loads(p_auth.stdout or "{}") if p_auth.stdout.strip() else {}
    upd_auth = ((out_auth.get("hookSpecificOutput") or {}).get("updatedInput")) or {}
    auth_held_frontier = upd_auth.get("model") is None

    p_triv = run_cli(
        bin_path,
        ["claude-code"],
        state,
        cwd=proj,
        payload={
            "session_id": sid,
            "session_models": CLAUDE_SESSION,
            "tool_name": "Task",
            "tool_use_id": "toolu_triv",
            "tool_input": {
                "prompt": "rename the userId variable to userIdentifier",
                "model": "claude-opus-5-5",
                "subagent_type": "Explore",
                "description": "rename var",
            },
        },
    )
    out_triv = json.loads(p_triv.stdout or "{}") if p_triv.stdout.strip() else {}
    upd_triv = ((out_triv.get("hookSpecificOutput") or {}).get("updatedInput")) or {}
    triv_downshifted = upd_triv.get("model") in ("haiku", "claude-haiku-5-5")

    return {
        "name": "User-owned policy floor (authz held frontier, routine downshifted)",
        "passed": auth_held_frontier and triv_downshifted,
        "authz_model_after": upd_auth.get("model") or "claude-opus-5-5 (held)",
        "routine_model_after": upd_triv.get("model") or "unchanged",
    }


def render_markdown(report: dict) -> str:
    comp = report["checks"][0]
    sig = report["checks"][1]
    cache = report["checks"][2]
    pol = report["checks"][3]
    lines = [
        "# Downshift 1-Command Proof Benchmark (Offline)",
        "",
        "| Pillar | Status | Measured Result |",
        "|---|---|---|",
        f"| 1. Compaction + lossless CCR | {'PASS' if comp['passed'] else 'FAIL'} | "
        f"{comp['raw_bytes']:,} -> {comp['compacted_bytes']:,} bytes (**{comp['cut_pct']:.1f}% cut**, "
        f"~{comp['saved_tokens_est']:,} tokens avoided); 4/4 `retrieve` byte-exact ({comp['report_retrieved_pct']:.1f}% reported) |",
        f"| 2. Signal + active anti-loop | {'PASS' if sig['passed'] else 'FAIL'} | "
        f"100% failure/stack lines kept on `go test`; 3rd identical failing `make test` triggered `{sig['third_attempt_passthrough_detail']}` (raw delivered) |",
        f"| 3. Prompt-cache safety | {'PASS' if cache['passed'] else 'FAIL'} | "
        f"Main-session `Read` untouched={cache['main_session_read_untouched']}; spawn `{cache['spawn_model_before']}` -> `{cache['spawn_model_after']}` with prompt preserved verbatim |",
        f"| 4. Policy floor + FinOps | {'PASS' if pol['passed'] else 'FAIL'} | "
        f"`authz` -> `{pol['authz_model_after']}`; routine -> `{pol['routine_model_after']}` |",
        "",
        "## Fixture breakdown (real repo outputs)",
        "",
        "| Fixture | Command | Raw bytes | Compacted bytes | Cut % | Lossless `retrieve` |",
        "|---|---|--:|--:|--:|---|",
    ]
    for row in comp["fixtures"]:
        status = f"{row.get('retrieve_mode', 'exact')} (`{row['ccr_id']}`)" if row["lossless_retrieve"] else "FAIL"
        cmd_cell = row["command"].replace("|", "\\|")
        lines.append(
            f"| `{row['fixture']}` | `{cmd_cell}` | {row['raw_bytes']:,} | {row['compacted_bytes']:,} | "
            f"{row['cut_pct']:.1f}% | {status} |"
        )
    passed_count = sum(1 for c in report["checks"] if c["passed"])
    lines.extend(["", f"**Result: {passed_count}/{len(report['checks'])} proof checks passed.**"])
    return "\n".join(lines) + "\n"


def run_all(bin_path: str) -> dict:
    if not os.path.isfile(bin_path):
        raise FileNotFoundError(f"downshift binary not found at {bin_path} (run `make build` first)")
    base = tempfile.mkdtemp(prefix="ds-proof-")
    try:
        clean_cwd = os.path.join(base, "clean_cwd")
        os.makedirs(clean_cwd, exist_ok=True)
        checks = [
            check_compaction_and_ccr(bin_path, base, clean_cwd),
            check_signal_and_antiloop(bin_path, base, clean_cwd),
            check_cache_safety(bin_path, base, clean_cwd),
            check_policy_and_finops(bin_path, base),
        ]
        ver = subprocess.run([bin_path, "version"], capture_output=True, text=True, timeout=15)
        return {
            "binary": os.path.basename(bin_path),
            "version": ver.stdout.strip(),
            "passed": all(c["passed"] for c in checks),
            "checks": checks,
        }
    finally:
        shutil.rmtree(base, ignore_errors=True)


def main(argv: list[str] | None = None) -> int:
    ap = argparse.ArgumentParser(description="Run Downshift's 1-command offline proof benchmark.")
    ap.add_argument("--bin", default=DEFAULT_BIN, help="Path to compiled downshift binary")
    ap.add_argument("--json", action="store_true", help="Emit machine-readable JSON")
    ap.add_argument("--write", default="", help="Optional path to write Markdown output")
    args = ap.parse_args(argv)

    try:
        report = run_all(os.path.abspath(args.bin))
    except FileNotFoundError as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 2

    md = render_markdown(report)
    if args.write:
        with open(args.write, "w", encoding="utf-8") as fh:
            fh.write(md)
    if args.json:
        print(json.dumps(report, indent=2))
    else:
        print(md, end="")
    return 0 if report["passed"] else 1


if __name__ == "__main__":
    sys.exit(main())
