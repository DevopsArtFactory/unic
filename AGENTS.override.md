# unic — Codex Override

Codex is the primary coding agent for this repository. Own implementation,
testing, documentation updates, and PR preparation unless the user explicitly
asks for planning or advisory-only output.

## Workflow

- Do not commit directly to `main`.
- New work should map to a GitHub issue. If the user asks to ship changes
  without an issue reference, call that out before opening a PR.
- The expected delivery flow is: issue -> implementation -> feature branch ->
  PR that references the issue.
- Read `PLAN.md` when planning new work so feature scope stays aligned with the
  active milestone and prior design decisions.
- Follow recent commit history for commit title style (`feat:`, `fix:`,
  `docs:`, and similar prefixes).

## Implementation Patterns

- Follow existing service patterns before inventing new ones. Use the RDS
  implementation (`internal/services/aws/rds.go`, `rds_model.go`,
  `rds_test.go`) as the first reference for new AWS service work.
- Keep Bubbletea and Lipgloss output consistent with the current UI patterns:
  column-aligned tables, dimmed labels, and existing style helpers.
- For scrolling detail views, preserve the windowing pattern
  `visibleLines := max(m.height-N, 5)`.
- Destructive or high-risk actions must use the existing type-to-confirm
  workflow before execution.
- Tests should use mock client interfaces following the existing AWS service
  test style.

## Feature Implementation Order

- For new service work, prefer this order:
  1. `internal/domain/model.go` and `internal/domain/catalog.go`
  2. `internal/services/aws/<service>.go` and `<service>_model.go`
  3. `internal/services/aws/repository.go`
  4. `internal/app/` integration
  5. Tests
- Match surrounding naming and file layout unless there is a clear reason not
  to.

## Validation

- After behavior changes, run `make test`.
- After behavior changes, run `make build`.
- If either validation step cannot be run, say so explicitly.

## Documentation

- When features are added, changed, or removed, update `README.md` to match the
  shipped behavior.
- Keep `Currently Implemented Features`, `TUI Key Bindings`, `Usage`, and
  `Configuration` aligned with the code.
- When milestone status changes, update `PLAN.md` before the PR is opened.

## Repo Skills

- Use the repo-local skills in `.agents/skills/` for the migrated Claude Code
  workflows: feature implementation, docs refresh, PR shipping, senior review,
  review-driven refactors, and issue maintenance.
