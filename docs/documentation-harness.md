# Documentation Harness

This file defines the minimum documentation work required when a feature or workflow changes.

## Goal

Prevent drift between the implementation and the docs.

If a change affects how `unic` behaves, this harness should be treated as part of the implementation checklist, not an optional follow-up.

## When This Harness Applies

Run this harness whenever a change modifies any of the following:

- a user-facing CLI command
- auth or context behavior
- config format or config resolution
- supported AWS services or feature catalog entries
- TUI navigation, keybindings, or screen flow
- operational behavior that users need to know to use safely
- development workflow that contributors are expected to follow

## Required Updates

### Always review

1. `README.md`
2. the relevant file in `docs/`

### Update `README.md` when the change affects

- installation or usage
- CLI commands
- config examples
- auth behavior
- supported feature list
- keybindings or common workflows

### Update `docs/project-overview.*` when the change affects

- product scope
- supported service areas
- repository/module ownership at a high level

### Update `docs/architecture.*` when the change affects

- module boundaries
- runtime flow
- auth flow
- repository responsibilities
- screen families or navigation architecture

### Update `docs/development.md` when the change affects

- contributor workflow
- implementation checklist
- testing expectations
- documentation responsibilities

### Update GitHub issues or PRs when the change affects

- near-term maintenance priorities
- the shape of ongoing product areas
- implementation status or milestone scope

## Pull Request Check

Before opening a PR, confirm:

- behavior is implemented
- tests or validation are updated
- docs were reviewed
- docs were changed if the behavior is user-visible or architecture-relevant

## Minimal Completion Rule

A feature change is not complete until:

1. code is updated
2. tests or validation are updated
3. `README.md` and the relevant `docs/` pages are updated when needed

## Quick Mapping

| Change Type | Update These Docs |
|---|---|
| New CLI command or flag | `README.md`, `docs/development.md`, optionally `docs/architecture.*` |
| New AWS service or feature | `README.md`, `docs/project-overview.*`, `docs/architecture.*` |
| Auth/config change | `README.md`, `docs/project-overview.*`, `docs/architecture.*`, `docs/development.md` |
| TUI flow / keybinding change | `README.md`, `docs/architecture.*` |
| Internal refactor with architectural impact | `docs/architecture.*`, optionally `docs/project-overview.*` |
