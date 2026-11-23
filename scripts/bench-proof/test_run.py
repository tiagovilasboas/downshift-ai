#!/usr/bin/env python3
# Copyright (c) 2026 Tiago Vilas Boas.
# SPDX-License-Identifier: Apache-2.0
# Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift-ai

import os
import sys
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import run as bench_proof


class BenchProofUnitTest(unittest.TestCase):
    def test_parse_retrieve_marker_with_and_without_session(self):
        self.assertEqual(
            bench_proof.parse_retrieve_marker("[downshift compact: 100 to 20 bytes; downshift retrieve a1b2c3d4e5f6]"),
            (None, "a1b2c3d4e5f6"),
        )
        self.assertEqual(
            bench_proof.parse_retrieve_marker(
                "[downshift compact: 100 to 20 bytes; downshift retrieve --session=0123456789ab deadbeef0001]"
            ),
            ("0123456789ab", "deadbeef0001"),
        )
        self.assertEqual(bench_proof.parse_retrieve_marker("plain text"), (None, None))

    def test_render_markdown_includes_all_four_pillars(self):
        sample = {
            "passed": True,
            "checks": [
                {
                    "passed": True,
                    "raw_bytes": 1000,
                    "compacted_bytes": 200,
                    "saved_tokens_est": 200,
                    "cut_pct": 80.0,
                    "report_retrieved_pct": 100.0,
                    "fixtures": [
                        {
                            "fixture": "diff_git_log_p.txt",
                            "command": "git log -p",
                            "raw_bytes": 1000,
                            "compacted_bytes": 200,
                            "cut_pct": 80.0,
                            "ccr_id": "0123456789ab",
                            "lossless_retrieve": True,
                            "retrieve_mode": "exact",
                        }
                    ],
                },
                {"passed": True, "third_attempt_passthrough_detail": "anti_loop_bypass"},
                {
                    "passed": True,
                    "main_session_read_untouched": True,
                    "spawn_model_before": "claude-opus-5-5",
                    "spawn_model_after": "claude-haiku-5-5",
                },
                {"passed": True, "authz_model_after": "claude-opus-5-5 (held)", "routine_model_after": "claude-haiku-5-5"},
            ],
        }
        md = bench_proof.render_markdown(sample)
        for token in ("Compaction + lossless CCR", "anti_loop_bypass", "Prompt-cache safety", "4/4 proof checks passed"):
            self.assertIn(token, md)

    def test_end_to_end_when_binary_present(self):
        if not os.path.isfile(bench_proof.DEFAULT_BIN):
            self.skipTest("compiled ./downshift binary not present")
        rep = bench_proof.run_all(bench_proof.DEFAULT_BIN)
        self.assertTrue(rep["passed"], msg=str(rep))


if __name__ == "__main__":
    unittest.main()
