# Branch Naming Harness

This file defines the branch naming convention for `unic`.

It is based on the guideline:

- [Git branch naming convention](https://github.com/ribaguifi/development-guidelines/blob/main/git-branch-naming-convention.md)

## Goal

Make branches easy to understand from the name alone.

The branch name should tell us:

- what kind of work it is
- which issue it relates to
- what the change is about

## Canonical Format

```text
<work-type>/<issue-number>-<short-description>
```

Examples:

- `feature/69-context-setup-env`
- `bugfix/76-s3-region-error-handling`
- `refactor/27-iam-rotation-flow`
- `docs/79-documentation-harness`

## Naming Rules

1. Use lowercase ASCII only.
2. Use dashes `-` to separate words.
3. Keep the description short and specific.
4. Prefer two to four meaningful words in the description.
5. Include the issue number when the work is tracked by a GitHub issue.
6. Avoid spaces, underscores, `#`, Korean text, or other non-URL-friendly characters.

## Allowed Work Types

Use one of these prefixes unless there is a strong reason not to:

- `feature`
- `bugfix`
- `refactor`
- `docs`
- `chore`
- `test`
- `hotfix`

## How To Choose a Name

### Feature work

```text
feature/58-s3-browser
feature/73-list-sorting
```

### Bug fixes

```text
bugfix/76-s3-region-error-handling
bugfix/27-iam-rotation-verification
```

### Refactors

```text
refactor/27-iam-service-split
refactor/69-context-doc-structure
```

### Docs-only work

```text
docs/79-documentation-harness
docs/80-architecture-refresh
```

## No-Issue Case

If there is no GitHub issue yet, use:

```text
<work-type>/<short-description>
```

Examples:

- `docs/readme-cleanup`
- `chore/release-prep`

Create or link an issue when the work is large enough to benefit from tracking.

## Pull Request Check

Before opening a PR, confirm:

- the branch name follows this convention
- the issue number is included when applicable
- the short description still matches the actual change

## Repository Rule

For contributor-facing work in this repository, prefer this harness over ad-hoc prefixes such as `feat/...` or personal naming styles.

If an automation or external tool creates a temporary branch, keep it readable and align it to this format whenever practical.
