"""Render scanner report metadata without publishing finding descriptions."""

import html
import json
import sys


def render_summary(report):
    def cell(value):
        return html.escape(str(value), quote=True)

    lines = ["## HOL Plugin Scanner", "", f"Score: {report['score']}/100", ""]
    lines.append("Analyzer status:")
    for integration in report["summary"]["integrations"]:
        lines.append(f"- {cell(integration['name'])}: {cell(integration['status'])}")
    lines.extend(["", "<table><tr><th>Severity</th><th>Rule</th><th>Location</th></tr>"])
    for finding in report["findings"]:
        location = finding.get("filePath") or "Repository"
        if finding.get("lineNumber") is not None:
            location += f":{finding['lineNumber']}"
        values = (finding["severity"], finding["ruleId"], location)
        lines.append("<tr>" + "".join(f"<td>{cell(value)}</td>" for value in values) + "</tr>")
    lines.extend(["</table>", "", "See the JSON artifact for full finding details.", ""])
    return "\n".join(lines)


if __name__ == "__main__":
    with open(sys.argv[1], encoding="utf-8") as report_file:
        print(render_summary(json.load(report_file)))
