---
name: unic-update-issues
description: Use when the user asks to review, reconcile, or update open GitHub issues for the unic repository against the current repo state. Do not use for creating new issues or implementing the work itself.
---

Review open `unic` GitHub issues against the current repo state and propose or
apply updates.

## Workflow

1. Gather state.
- Fetch open issues with `gh issue list --state open --limit 50`.
- Read issue details with `gh issue view <number>` before proposing edits.
- Read relevant repository docs, code, and issue comments.
- Check recent history with `git log --oneline -20`.
- Inspect merged PRs or current code when an issue appears complete.
- Use `gh pr view <number>` or `gh pr list --state merged --limit 50` when an
  issue may already be shipped through a PR.

2. Assess each issue or filtered subset.
- already done
- partially done
- stale or superseded
- missing details or missing links
- labels or tracking references that should change

3. Present recommendations before mutating anything.
- Summarize each issue and the suggested action.
- Show the planned body diff before editing issue text.
- Do not close or edit an issue without explicit user confirmation.

4. Execute approved actions only.
- Use `gh issue close`, `gh issue edit`, or `gh issue comment` as needed.
- If an issue references a PR, verify whether the PR was merged.

## Output

Report what changed, what was left unchanged, and any issues that still need a
human product decision.
