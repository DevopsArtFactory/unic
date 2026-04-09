---
name: unic-refactor-review
description: Use when the user asks to implement refactors from a prior unic senior review report. Do not use for new features or behavior changes.
---

Refactor `unic` based on an existing senior review report while preserving
behavior.

## Workflow

1. Load the latest review report.
- Prefer `.agents/reports/senior-review-*.md`.
- Fall back to `.claude/reports/senior-review-*.md`.
- If no report exists, tell the user to run `unic-senior-review` first.

2. Select scope.
- If the user named a priority or finding, scope the work to that item.
- If the user named a GitHub issue, read it with `gh issue view <number>`.
- When GitHub access is available, search open issues with
  `gh issue list --state open --search '<terms>'` to see whether the finding is
  already tracked before choosing scope.
- If not, summarize the report priorities and ask which one to tackle.

3. Plan before editing.
- Read every file referenced by the selected findings.
- Preserve behavior; this is refactor-only work.
- Do not change public APIs unless the finding specifically requires it.
- If the change would touch more than 10 files, split it into smaller steps.

4. Implement and validate incrementally.
- Make focused changes.
- Run `make test` after each logical refactor group when practical.
- Run `make test` and `make build` at the end.

5. Update the report when appropriate.
- Mark completed findings or note what changed.
- If the work maps to an issue, note the issue number in the report update.
- If a recommendation from the report turns out to be impractical, explain why
  instead of forcing the refactor.

## Constraints

- No new features.
- No intentional behavior changes.
- Note newly discovered issues, but leave them for a future review cycle.
