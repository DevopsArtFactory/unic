---
name: unic-ship-pr
description: Use when the user explicitly asks to stage, commit, push, and open a pull request for unic. Do not use for general coding work or docs-only sync unless the user is asking to ship the current branch.
---

Ship the current `unic` changes through the repository's expected branch,
commit, and PR flow.

## Workflow

1. Confirm main-based worktree isolation.
- Ship only from a task branch in a fresh git worktree created from `main` or
  `origin/main`.
- If the current changes are in the primary checkout or on a branch that was not
  created from `main`, create a new main-based worktree and port only the
  intended changes before validating or opening a PR.
- Keep one worktree per PR-sized change.

2. Validate first.
- Run `make test` and stop if it fails.
- Run `make build` and stop if it fails.

3. Inspect scope.
- Check `git status` and `git diff --stat`.
- Confirm the changes are ready to ship and map to the intended issue.
- If the issue number is known, read it with `gh issue view <number>`.
- If the issue is implied but not named, search with
  `gh issue list --state open --search '<terms>' --limit 20` before opening the
  PR.
- Do not open a PR without an issue reference unless the user explicitly
  approves that exception.

4. Create the branch.
- Branch from `main` in a fresh worktree.
- Use a conventional prefix such as `feat/`, `fix/`, or `docs/`.
- Use the user hint when provided; otherwise infer a concise branch name from
  the change.

5. Stage and commit carefully.
- Stage only the relevant files by name.
- Never use `git add .` or `git add -A`.
- Use a conventional commit title consistent with recent history.
- Include a short body with bullet points for key changes.
- Do not add `Co-Authored-By` lines.

6. Push and open the PR.
- Push with `-u origin <branch>`.
- Use `gh pr create` to open the PR.
- If a PR already exists for the branch, inspect it with `gh pr view` and
  update that PR instead of opening a duplicate.
- If `.github/pull_request_template.md` exists, follow it.
- Otherwise structure the PR body with: Summary, Related Issues, Validation,
  and Checklist.

7. Wait for automated review.
- After the PR is open, wait for required checks and Amazon Q Developer review
  to finish before reporting the PR as ready.
- Read the latest Amazon Q top-level review/comments for the current head with
  `gh pr view <number> --json reviews,comments,statusCheckRollup,headRefOid`.
- Also read thread-aware inline review state before deciding the PR is clean.
  Use the GitHub connector `list_pull_request_review_threads` when available,
  or `gh api graphql` to fetch `reviewThreads { isResolved isOutdated path line
  comments { author { login } body createdAt } }`.
- Treat any unresolved, non-outdated Amazon Q inline thread as actionable unless
  it is clearly informational. Do not rely only on top-level review summaries;
  they can miss inline findings.
- Ignore outdated Amazon Q threads only after confirming the current head no
  longer contains the flagged code.
- If Amazon Q reports actionable issues in either top-level reviews or current
  inline threads, patch them locally, rerun `make test` and `make build`, commit
  the fix, push, and comment `/q review` on the PR.
- Repeat the wait/read-threads/read-summary/patch/review loop until the latest
  Amazon Q review for the current head has no blocking or actionable issues and
  there are no unresolved, non-outdated actionable Amazon Q threads.
- If Amazon Q is unavailable, still report the PR URL and the completed local
  validation, but explicitly note that automated review could not be confirmed.
- Do not resolve or dismiss review threads unless the user explicitly asks.

## Output

Return the branch name, commit message, PR URL, local validation, and latest
Amazon Q result.
