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


if __name__ == "__main__":
    unittest.main()
