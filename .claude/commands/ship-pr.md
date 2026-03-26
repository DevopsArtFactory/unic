Stage, commit, push, and create a PR for the current changes.

## Input
$ARGUMENTS — optional branch name and PR title hint

## Workflow

1. Run `make test` to verify all tests pass. Stop if tests fail.
2. Run `make build` to verify compilation. Stop if build fails.
3. Check `git status` and `git diff --stat` to understand what changed.
4. Create a feature branch from main using `feat/` prefix (e.g., `feat/rds-aurora-support`). Use the argument as a hint for the branch name, or infer from the changes.
5. Stage only the relevant modified files by name. Never use `git add -A` or `git add .`.
6. Commit with a conventional commit message (`feat:`, `fix:`, `docs:`, etc.) following the style in `git log --oneline -10`. Include a body with bullet points summarizing key changes. Do NOT add Co-Authored-By lines.
7. Push with `-u origin <branch>`.
8. Create a PR using `gh pr create` following the template in `.github/pull_request_template.md`:
   - Summary: what changed and why
   - Related Issues: reference any GitHub issues if applicable
   - Validation: describe how it was tested
   - Checklist: scope, docs, tests, breaking changes

Return the PR URL when done.
