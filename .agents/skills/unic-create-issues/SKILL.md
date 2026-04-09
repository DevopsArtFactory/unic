---
name: unic-create-issues
description: Use when the user asks to create GitHub issues from planned but unimplemented unic work in PLAN.md. Do not use for editing existing issues or for implementation.
---

Create GitHub issues for planned `unic` work that does not already have issue
coverage.

## Workflow

1. Read `PLAN.md` to identify planned milestones and unimplemented work.
2. Fetch existing issues with `gh issue list --state all --limit 100`.
3. For borderline matches, inspect candidate issues with `gh issue view <number>`
before deciding a PLAN item is uncovered.
4. Identify PLAN items without corresponding issues.
5. Create missing issues with `gh issue create`.
- Title format: `feat: <description> (M<X>.<Y>)`
- Label: `enhancement`
- Body sections: Summary, Details, Checklist
- Keep checklist items grounded in the repo's real files and directories

6. Skip anything that already has an open or closed issue.

## Output

Report the created issues with URLs and note any PLAN items you intentionally
left alone because matching issues already existed.
