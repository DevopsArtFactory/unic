---
name: unic-create-issues
description: Use when the user asks to create GitHub issues for confirmed but unimplemented unic work. Do not use for editing existing issues or for implementation.
---

Create GitHub issues for confirmed `unic` work that does not already have issue
or pull request coverage.

## Workflow

1. Resolve the requested scope from explicit maintainer input. If the request is
general, inspect current issues and pull requests, recent commits, and relevant
repository docs or code for a concrete unimplemented gap. Stop without creating
an issue when no gap is supported by repository evidence.
2. Fetch existing issues with `gh issue list --state all --limit 100` and open
pull requests with `gh pr list --state open --limit 100`.
3. For borderline matches, inspect candidates with `gh issue view <number>` or
`gh pr view <number>` before deciding the work is uncovered.
4. Create only confirmed missing issues with `gh issue create`.
- Use a conventional title prefix that matches the work, such as `feat:`,
  `fix:`, `docs:`, or `chore:`.
- Add a label only when an existing repository label clearly applies.
- Use body sections: Summary, Evidence, Acceptance criteria.
- Ground the evidence and acceptance criteria in explicit maintainer input,
  concrete repository paths, or observable current behavior.

5. Skip anything already covered by an open or closed issue or an open pull
request. Do not invent roadmap items, milestones, or implementation details.

## Output

Report created issues with URLs, duplicate matches that were skipped, and any
requested scope left alone because it lacked concrete evidence.
