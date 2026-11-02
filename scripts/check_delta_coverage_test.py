"""Pruebas de scripts/check_delta_coverage.py (nv-03 plan §3).

python3 -m unittest discover -s scripts -p "*_test.py"
"""
import unittest

import check_delta_coverage as dc

MODULE = "github.com/WhereverApp2026/wherever-backend"

DIFF = """\
diff --git a/internal/shared/x/x.go b/internal/shared/x/x.go
--- a/internal/shared/x/x.go
+++ b/internal/shared/x/x.go
@@ -0,0 +10,4 @@
+a
+b
+c
+d
@@ -20 +30 @@
+e
diff --git a/internal/shared/x/x_test.go b/internal/shared/x/x_test.go
--- a/internal/shared/x/x_test.go
+++ b/internal/shared/x/x_test.go
@@ -0,0 +1,2 @@
+t
+t
diff --git a/internal/shared/x/gone.go b/internal/shared/x/gone.go
--- a/internal/shared/x/gone.go
+++ /dev/null
@@ -1,3 +0,0 @@
-x
"""

PROFILE = f"""\
mode: atomic
{MODULE}/internal/shared/x/x.go:10.1,11.5 2 3
{MODULE}/internal/shared/x/x.go:12.1,13.5 2 0
{MODULE}/internal/shared/x/x.go:30.1,30.9 1 0
{MODULE}/internal/shared/x/x.go:30.1,31.2 1 1
"""


class ParseDiffTest(unittest.TestCase):
    def test_takes_new_side_lines_and_skips_tests_and_deletions(self):
        self.assertEqual({"internal/shared/x/x.go": {10, 11, 12, 13, 30}}, dc.parse_diff(DIFF))

    def test_pure_deletion_hunk_adds_nothing(self):
        diff = "+++ b/internal/a/a.go\n@@ -5,2 +4,0 @@\n"
        self.assertEqual({}, dc.parse_diff(diff))


class HygieneTest(unittest.TestCase):
    def test_hygiene_is_out_of_the_delta(self):
        for path in (
            "cmd/api/main.go",
            "internal/testutil/tokens.go",
            "internal/usuarios/mocks/repo.go",
            "internal/usuarios/repository_mock.go",
            "internal/usuarios/dto.go",
        ):
            with self.subTest(path=path):
                self.assertFalse(dc.in_scope(path))
        self.assertTrue(dc.in_scope("internal/usuarios/service.go"))

    def test_delta_with_only_hygiene_is_empty(self):
        diff = "+++ b/internal/testutil/tokens.go\n@@ -0,0 +1,3 @@\n+++ b/internal/usuarios/dto.go\n@@ -0,0 +1,9 @@\n"
        code, report = dc.evaluate(dc.measure(dc.parse_diff(diff), {}), 80, advisory=False)
        self.assertEqual(0, code)
        self.assertIn("delta vacío", report[-1])


class EvaluateTest(unittest.TestCase):
    def setUp(self):
        blocks = dc.parse_profile(PROFILE, MODULE)
        self.deltas = dc.measure(dc.parse_diff(DIFF), blocks)

    def test_line_union_counts_a_line_covered_by_any_block(self):
        delta = self.deltas["internal/shared/x/x.go"]
        self.assertEqual({10, 11, 12, 13, 30}, delta.coverable)
        self.assertEqual({10, 11, 30}, delta.covered)

    def test_empty_delta_passes(self):
        code, report = dc.evaluate({}, 80, advisory=False)
        self.assertEqual(0, code)
        self.assertIn("delta vacío", report[-1])

    def test_below_minimum_is_advisory_exit_zero(self):
        code, report = dc.evaluate(self.deltas, 80, advisory=True)
        self.assertEqual(0, code)
        self.assertIn("ADVISORY", report[-1])
        self.assertIn("60.0%", report[-1])

    def test_below_minimum_without_advisory_fails(self):
        code, report = dc.evaluate(self.deltas, 80, advisory=False)
        self.assertEqual(1, code)
        self.assertIn("FAIL", report[-1])

    def test_at_or_above_minimum_passes(self):
        code, report = dc.evaluate(self.deltas, 60, advisory=False)
        self.assertEqual(0, code)
        self.assertIn("OK", report[-1])


if __name__ == "__main__":
    unittest.main()
