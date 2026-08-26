---
name: unic-implement-feature
description: Use when the user asks to implement a feature request or GitHub issue in the unic repository, especially new AWS service work. Do not use for pure refactors, docs-only updates, PR shipping, or issue triage.
---

Implement a feature for `unic` using the repository's existing service and TUI
patterns.

## Input

Treat the prompt as one of:
- a GitHub issue number such as `#29`
- a feature description
- an explicitly scoped feature request

## Workflow

1. Start from an isolated main-based worktree.
- Before editing anything, fetch or verify the intended `main` base.
- Create a fresh git worktree with a new task branch from `main` or
  `origin/main`.
- Do not implement in the primary checkout or on a pre-existing feature branch.
- Use one worktree per issue or feature.
- If the feature appears to depend on another unmerged branch, still start from
  `main` and document the dependency before applying any stacked changes.

2. Resolve scope first.
- Before implementing anything, inspect open pull requests with
  `gh pr list --state open --limit 50`.
- If the target issue or feature already has an open PR in progress, stop and
  tell the user instead of implementing duplicate scope. Only continue if the
  user explicitly asks to work on that existing PR or to intentionally create a
  follow-up.
- If the user gave an issue number, read it with `gh issue view <number>`.
- When an issue number is known, also search open PRs for that issue first
  (title, body, or branch naming) before starting implementation.
- Once the issue to implement is known and you have confirmed there is no open
  PR already covering it, claim the issue before coding by commenting
  `@unic-bot: assign me` on the issue.
- If the user gave a description, search open issues first with
  `gh issue list --state open --search '<terms>' --limit 20`.
- For description-based requests, also search open PRs with the same terms
  before choosing the feature to implement.
- If the user did not name a concrete feature, inspect the current backlog with
  `gh issue list --state open --limit 50` and prefer an open issue over
  inventing new scope.
- Use GitHub issues and pull requests, including relevant comments, as the
  source of truth for planned work and status. Use repository docs and code to
  verify implementation context, not to infer speculative roadmap work.
- If multiple open issues fit equally well, ask only the minimum clarifying
  question needed to pick the right one.

3. Gather repo context before editing.
- Read `internal/domain/model.go` and `internal/domain/catalog.go`.
- Read `internal/services/aws/repository.go`.
- Use the RDS implementation in `internal/services/aws/rds.go`,
  `internal/services/aws/rds_model.go`, and `internal/services/aws/rds_test.go`
  as the first reference pattern.
- Read the owning TUI flow in `internal/app/app.go` and any extracted helper
  files nearby.

4. Plan the change before writing code.
- For new service work, prefer this order:
  1. domain model and catalog
  2. AWS service layer and models
  3. repository interface wiring
  4. TUI integration
  5. tests
- Keep behavior aligned with existing Bubbletea and Lipgloss conventions.
- Use `visibleLines := max(m.height-N, 5)` style windowing where applicable.
- Destructive actions must use the existing type-to-confirm pattern.

5. Implement with minimal divergence.
- Follow existing naming, layout, and error-handling patterns.
- Use mock client interfaces in tests.
- Avoid introducing new abstractions unless the surrounding code already points
  in that direction.

6. Validate and sync docs.
- Run `make test`.
- Run `make build`.
- Update `README.md` when shipped behavior changes.
- Update the related GitHub issue or pull request when implementation status or
  scope changes.

## Output

Report what changed, how it was validated, and any remaining follow-up or PR
preparation steps.
