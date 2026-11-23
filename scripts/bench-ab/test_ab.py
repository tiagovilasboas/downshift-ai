#!/usr/bin/env python3
# Copyright (c) 2026 Tiago Vilas Boas.
# SPDX-License-Identifier: Apache-2.0
# Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift-ai

import os
import sys
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import run_ab
import score_ab


class BenchABUnitTest(unittest.TestCase):
    def test_dry_run_verifies_baseline_fails(self):
        tasks = run_ab.load_tasks("all")
        recs = run_ab.run_pair(tasks, harness="claude", reps=1, bin_path="/usr/local/bin/downshift", model="", dry_run=True)
        self.assertEqual(len(recs), 4)
        self.assertTrue(all(r["dry_run"] and r["baseline_fails"] for r in recs))
        off_cmds = [r["command"] for r in recs if r["arm"] == "off"]
        self.assertTrue(all("--disable-all-hooks" in c for c in off_cmds))

    def test_extract_usage_claude_and_codex_lines(self):
        raw = '{"type":"result","usage":{"input_tokens":1200,"output_tokens":300,"cache_read_input_tokens":48000,"cache_creation_input_tokens":500}}\n'
        u = run_ab.extract_usage(raw)
        self.assertEqual(u, {"input": 1200, "output": 300, "cache_read": 48000, "cache_write": 500})

    def test_score_calculations_and_markdown(self):
        u = {"input": 1000, "output": 500, "cache_read": 98000, "cache_write": 1000}
        self.assertAlmostEqual(score_ab.cache_hit_pct(u), 98.0, places=2)
        self.assertEqual(score_ab.billable_tokens(u), 2500)
        usd = score_ab.list_rate_usd(u, "sonnet")
        self.assertAlmostEqual(usd, 0.0193, places=4)
        md = score_ab.render_table(
            [
                {
                    "task": "debug-reverse",
                    "arm": "off",
                    "accepted": True,
                    "model": "sonnet",
                    "usage": u,
                    "compactor_cut_bytes": 0,
                    "retrieved_bytes": 0,
                },
                {
                    "task": "debug-reverse",
                    "arm": "on",
                    "accepted": True,
                    "model": "sonnet",
                    "usage": {"input": 600, "output": 400, "cache_read": 98000, "cache_write": 700},
                    "compactor_cut_bytes": 12400,
                    "retrieved_bytes": 0,
                },
            ]
        )
        self.assertIn("Cache Hit %", md)
        self.assertIn("98.0%", md)
        self.assertIn("12,400 B", md)


if __name__ == "__main__":
    unittest.main()
