from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

from scanner_summary import render_summary


class ScannerSummaryTest(unittest.TestCase):
    def test_report_metadata_is_visible_without_finding_content(self):
        report = {
            "score": 78,
            "summary": {"integrations": [{"name": "Cisco MCP", "status": "unavailable"}]},
            "findings": [
                {
                    "severity": "high",
                    "ruleId": "HARDCODED_SECRET",
                    "filePath": "src/<example>|file.py",
                    "lineNumber": 41,
                    "description": "sensitive finding text",
                    "remediation": "sensitive remediation text",
                },
                {"severity": "info", "ruleId": "METADATA_MISSING"},
            ],
        }

        summary = render_summary(report)

        self.assertIn("Score: 78/100", summary)
        self.assertIn("Cisco MCP: unavailable", summary)
        self.assertIn("<td>high</td><td>HARDCODED_SECRET</td>", summary)
        self.assertIn("src/&lt;example&gt;|file.py:41", summary)
        self.assertIn("<td>Repository</td>", summary)
        self.assertNotIn("<example>", summary)
        self.assertNotIn("sensitive", summary)
        self.assertIn("JSON artifact", summary)

    def test_invalid_report_keeps_failure_and_writes_summary(self):
        script = Path(__file__).with_name("scanner_summary.py")
        with tempfile.TemporaryDirectory() as temp:
            report = Path(temp) / "report.json"
            for content in (
                "malformed JSON", "{}", '{"score": 78, "summary": null}',
                '{"score": 78, "summary": {"integrations": []}, "findings": [null]}',
            ):
                with self.subTest(content=content):
                    report.write_text(content, encoding="utf-8")
                    result = subprocess.run(
                        [sys.executable, str(script), str(report)],
                        capture_output=True, text=True, check=False,
                    )
                    self.assertEqual(result.returncode, 1)
                    self.assertIn("Unable to render", result.stdout)
                    self.assertIn("Invalid or unavailable", result.stderr)


if __name__ == "__main__":
    unittest.main()
