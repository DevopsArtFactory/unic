Review open GitHub issues against the current state of the repo and suggest updates.

## Input
$ARGUMENTS — optional scope filter (e.g., "Route53", "documentation", or issue number like "#13")

## Workflow

1. Fetch open issues: `gh issue list --state open --limit 50`.
2. Read relevant repository docs, code, issue comments, and recent git history
   (`git log --oneline -20`) to understand current repo state.
3. For each open issue (or filtered subset), assess:
   - **Already done?** — Check if the feature/fix has been merged (search PRs, git log, code).
   - **Partially done?** — Check if some checklist items are complete and the issue body needs updating.
   - **Stale or superseded?** — Check if the issue is no longer relevant given
     current code, merged work, or maintainer direction.
   - **Missing details?** — Check if the issue body is missing context that now exists (e.g., related PRs, design decisions).
   - **Labels/tracking wrong?** — Check if labels or references need updating.
4. Present findings to the user as a numbered list:
   ```
   1. #13 — "feat: Add Route53 ListHostedZones support"
      → Implemented in PR #38. Suggest: close issue.
   2. #40 — "feat: Route53 record mutations"
      → Still open, no work started. No changes needed.
   ```
5. Ask the user which actions to take (close, edit body, add comment, update labels, no change).
6. Execute only the approved actions using `gh issue close`, `gh issue edit`, or `gh issue comment`.
7. Report what was updated.

## Rules
- Never close or edit an issue without user confirmation.
- When editing issue bodies, show the diff of what will change before applying.
- If an issue references a PR, check if the PR is merged.
