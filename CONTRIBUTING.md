# Contributing to unic

Thanks for your interest in contributing.

## Before You Start

- Search existing issues before opening a new one.
- For larger changes, open an issue first to discuss scope.

## Development Workflow

1. Fork or branch from `main`.
2. Create a focused branch following [`docs/branch-naming-harness.md`](docs/branch-naming-harness.md).
3. Keep commits small and descriptive.
4. Run `go test ./...` before pushing.
5. Run the documentation harness in [`docs/documentation-harness.md`](docs/documentation-harness.md) for user-visible or architecture-relevant changes.
6. Open a Pull Request with context and testing notes.

## Pull Request Checklist

- [ ] The change is scoped and documented.
- [ ] New user-facing AWS service capabilities are TUI-first: they ship with a TUI entry point, and any CLI command is a secondary surface for scripting or automation.
- [ ] Branch name follows `docs/branch-naming-harness.md`.
- [ ] `README.md` and relevant `docs/` pages were reviewed and updated if needed.
- [ ] Existing behavior is not broken.
- [ ] Tests or validation steps are included.
- [ ] Related issue is linked (if any).

## Commit Message Style

Use clear commit messages. Conventional Commits are recommended:

- `feat: add ...`
- `fix: resolve ...`
- `docs: update ...`

## Code of Conduct

By participating, you agree to follow the [Code of Conduct](CODE_OF_CONDUCT.md).
