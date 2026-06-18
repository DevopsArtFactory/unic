---
name: unic-update-dependencies
description: Use when the user asks to update, bump, refresh, audit, or modernize dependencies, libraries, Go modules, the Go language version, or Go toolchain support in the unic repository. Use for dependency maintenance PRs, safe patch/minor Go module updates, Go version bumps, vulnerability-driven dependency updates, or CI/doc updates tied to dependency or Go version changes. Do not use for unrelated feature work, docs-only edits, or CI refactors unless they are required by the dependency/toolchain update.
---

Update `unic` dependencies and Go toolchain support with conservative scope,
clear validation, and repository workflow alignment.

## Workflow

1. Resolve intent and risk.
- Treat "safe", "routine", or unspecified dependency updates as patch/minor
  updates only.
- Treat major version upgrades as opt-in. Do not apply them unless the user
  explicitly asks for major upgrades or a specific module version.
- Separate Go language/toolchain version bumps from dependency refreshes unless
  the user asks for both.
- If the user asks for "latest Go", verify the current stable Go release from
  official Go sources before editing.

2. Inspect repository state before editing.
- Check `git status --short --branch`.
- Read `go.mod` and identify the current `go` and `toolchain` directives.
- Search for Go version pins in `.github/`, `Makefile`, `README.md`,
  `Dockerfile*`, `.go-version`, `.tool-versions`, and scripts.
- Inspect available module updates with `go list -m -u all`.
- If network access blocks Go module or release checks, request escalation
  instead of guessing.

3. Plan the update.
- For patch-only refreshes, prefer `go get -u=patch ./...`.
- For safe minor refreshes, prefer explicit updates from `go list -m -u all`
  or `go get -u ./...`, then review the diff.
- For targeted updates, use `go get <module>@<version>` and keep unrelated
  modules unchanged where practical.
- For vulnerability work, update the minimum affected modules needed to resolve
  the advisory, then run vulnerability checks if the toolchain is available.
- For Go version bumps, update every repo-controlled Go version pin needed for
  local build, CI, and docs consistency.

4. Apply changes carefully.
- Run the chosen `go get` command.
- Run `go mod tidy`.
- Review `git diff -- go.mod go.sum` before broadening scope.
- Avoid unrelated formatting, generated churn, or feature refactors.
- Update `README.md` or CI/docs only when supported Go version or setup
  instructions changed.

5. Validate.
- Run `make test`.
- Run `make build`.
- If validation fails because dependencies changed, fix the failure or revert
  the offending dependency update while preserving unrelated user changes.
- If validation cannot be run, report the exact blocker.

6. Ship only when requested.
- Follow the repo rule: issue -> implementation -> feature branch -> PR.
- If the user asks to open a PR and no issue is referenced, search open issues
  first. If none matches, call out that a new issue is needed before shipping
  unless the user explicitly approves an exception.
- Use the `unic-ship-pr` workflow for staging, committing, pushing, and opening
  the PR.

## Output

Report:
- Go version changes, if any.
- Direct and notable transitive dependency updates.
- Major updates intentionally skipped.
- Files changed.
- Validation results for `make test` and `make build`.
- Any follow-up needed for major upgrades, vulnerability checks, or PR shipping.
