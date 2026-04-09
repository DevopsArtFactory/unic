---
name: unic-ship-pr
description: Use when the user explicitly asks to stage, commit, push, and open a pull request for unic. Do not use for general coding work or docs-only sync unless the user is asking to ship the current branch.
---

Ship the current `unic` changes through the repository's expected branch,
commit, and PR flow.

## Workflow

1. Validate first.
- Run `make test` and stop if it fails.
- Run `make build` and stop if it fails.

2. Inspect scope.
- Check `git status` and `git diff --stat`.
- Confirm the changes are ready to ship and map to the intended issue.
- If the issue number is known, read it with `gh issue view <number>`.
- If the issue is implied but not named, search with
  `gh issue list --state open --search '<terms>' --limit 20` before opening the
  PR.
- Do not open a PR without an issue reference unless the user explicitly
  approves that exception.

3. Create the branch.
- Branch from `main`.
- Use a conventional prefix such as `feat/`, `fix/`, or `docs/`.
- Use the user hint when provided; otherwise infer a concise branch name from
  the change.

4. Stage and commit carefully.
- Stage only the relevant files by name.
- Never use `git add .` or `git add -A`.
- Use a conventional commit title consistent with recent history.
- Include a short body with bullet points for key changes.
- Do not add `Co-Authored-By` lines.

5. Push and open the PR.
- Push with `-u origin <branch>`.
- Use `gh pr create` to open the PR.
- If a PR already exists for the branch, inspect it with `gh pr view` and
  update that PR instead of opening a duplicate.
- If `.github/pull_request_template.md` exists, follow it.
- Otherwise structure the PR body with: Summary, Related Issues, Validation,
  and Checklist.

## Output

Return the branch name, commit message, and PR URL.
