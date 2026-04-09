---
name: unic-senior-review
description: Use when the user wants a deep maintainability or refactoring review of the unic codebase, including file-size analysis, Go idioms, and concrete refactoring priorities. Do not use for normal bug reviews or feature implementation.
---

Conduct a senior-engineer style review of `unic` focused on code quality,
maintainability, and refactoring opportunities.

## Workflow

1. Start with a line-count inventory.
- Run `find internal/ cmd/ -name '*.go' | xargs wc -l | sort -rn`.
- Run `find internal/ cmd/ -name '*_test.go' | xargs wc -l | sort -rn`.
- When GitHub access is available, run `gh issue list --state open --limit 50`
  so you can distinguish already-tracked work from net-new findings.
- Produce a table with File, Lines, and Status.
- Flag files over 300 lines as `[LONG]` and over 500 lines as `[CRITICAL]`.

2. Review the code against these categories.
- file size and function length
- naming and readability
- SOLID, abstraction, and DRY pressure
- error handling and edge cases
- Go-specific idioms such as context flow, error wrapping, goroutine cleanup,
  and `defer` usage

3. Keep the review actionable.
- Lead with concrete findings.
- Include file references.
- Give a specific refactoring suggestion for every finding.
- Call out when a finding already appears to be tracked by an open GitHub
  issue.
- Avoid style-only feedback unless it hides a real maintenance risk.
- Do not nitpick test-file length unless there is a real quality issue.

4. End with a report.
- Executive summary
- LoC breakdown table
- Findings ordered by severity
- Top 5 refactoring priorities
- Positive patterns worth preserving

## Optional persistence

If the user wants the report saved, prefer `.agents/reports/senior-review-YYYY-MM-DD.md`.
You may read legacy reports from `.claude/reports/` for context when helpful.
