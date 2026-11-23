#!/usr/bin/env python3
# Copyright (c) 2026 Tiago Vilas Boas.
# SPDX-License-Identifier: Apache-2.0
# Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift-ai
"""Score paired Off vs Downshift A/B JSON results into Markdown."""

from __future__ import annotations

import argparse
import json
import sys

# List-rate reference (USD / 1M tokens, Anthropic API catalog rates in docs/guides/evidence.md).
# Used ONLY for reproducible reference comparison; never a subscription plan invoice.
LIST_RATES_PER_M = {
    "haiku": {"input": 1.00, "output": 5.00, "cache_read": 0.10, "cache_write": 1.25},
    "sonnet": {"input": 2.00, "output": 10.00, "cache_read": 0.10, "cache_write": 2.50},
    "opus": {"input": 4.00, "output": 20.00, "cache_read": 0.20, "cache_write": 5.00},
}


def tier_rates(model: str) -> dict[str, float]:
    m = (model or "").lower()
    if "haiku" in m:
        return LIST_RATES_PER_M["haiku"]
    if "opus" in m:
        return LIST_RATES_PER_M["opus"]
    return LIST_RATES_PER_M["sonnet"]


def cache_hit_pct(usage: dict) -> float:
    cr = int(usage.get("cache_read", 0))
    cw = int(usage.get("cache_write", 0))
    inp = int(usage.get("input", 0))
    denom = cr + cw + inp
    if denom <= 0:
        return 0.0
    return 100.0 * cr / denom


def billable_tokens(usage: dict) -> int:
    return int(usage.get("input", 0)) + int(usage.get("output", 0)) + int(usage.get("cache_write", 0))


def list_rate_usd(usage: dict, model: str = "sonnet") -> float:
    r = tier_rates(model)
    return (
        int(usage.get("input", 0)) * r["input"]
        + int(usage.get("output", 0)) * r["output"]
        + int(usage.get("cache_read", 0)) * r["cache_read"]
        + int(usage.get("cache_write", 0)) * r["cache_write"]
    ) / 1_000_000.0


def render_table(records: list[dict]) -> str:
    grouped: dict[tuple[str, str], list[dict]] = {}
    for rec in records:
        grouped.setdefault((rec["task"], rec["arm"]), []).append(rec)

    lines = [
        "| Task | Arm | Accepted | Billable Tokens | Cache Read | Cache Hit % | List-Rate Est. (USD) | Compactor Cut | CCR Retrieved |",
        "|---|---|--:|--:|--:|--:|--:|--:|--:|",
    ]
    for (task, arm), items in sorted(grouped.items()):
        evaluated = [x for x in items if x.get("accepted") is not None]
        acc_str = f"{sum(1 for x in evaluated if x['accepted'])}/{len(evaluated)}" if evaluated else "dry-run"
        bill = sum(billable_tokens(x.get("usage") or {}) for x in items)
        cr = sum(int((x.get("usage") or {}).get("cache_read", 0)) for x in items)
        cw = sum(int((x.get("usage") or {}).get("cache_write", 0)) for x in items)
        inp = sum(int((x.get("usage") or {}).get("input", 0)) for x in items)
        hit = cache_hit_pct({"input": inp, "cache_read": cr, "cache_write": cw})
        usd = sum(list_rate_usd(x.get("usage") or {}, x.get("model", "sonnet")) for x in items)
        cut = sum(int(x.get("compactor_cut_bytes", 0)) for x in items)
        ret = sum(int(x.get("retrieved_bytes", 0)) for x in items)
        lines.append(
            f"| {task} | {arm} | {acc_str} | {bill:,} | {cr:,} | {hit:.1f}% | ${usd:.4f} | {cut:,} B | {ret:,} B |"
        )
    lines.extend(
        [
            "",
            "_Note: List-Rate Est. (USD) applies reference API list rates from `docs/guides/evidence.md` for comparison; it is not a subscription plan invoice._",
        ]
    )
    return "\n".join(lines) + "\n"


def main(argv: list[str] | None = None) -> int:
    ap = argparse.ArgumentParser(description="Render Off vs Downshift A/B JSON results to Markdown.")
    ap.add_argument("results_json", help="Path to results JSON written by run_ab.py")
    args = ap.parse_args(argv)
    with open(args.results_json, encoding="utf-8") as fh:
        records = json.load(fh)
    print(render_table(records), end="")
    return 0


if __name__ == "__main__":
    sys.exit(main())
