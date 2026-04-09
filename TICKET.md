# Ticket Tracking Note

This repository no longer uses `TICKET.md` as the active work tracker.

## Source of Truth

Current work is tracked in:

- GitHub Issues
- GitHub Pull Requests
- branch-specific implementation changes

## Why This File Exists

Older iterations of the project kept a single in-repo "current ticket" note here.
That model became stale as work moved to GitHub-native issue and PR tracking.

## Recommended Practice

If a change lands that affects user-facing behavior or architecture:

1. update the implementation
2. add or update tests
3. update `README.md` if the behavior is user-visible
4. update `.kiro/docs` and `.kiro/steering` docs if the architecture or project shape changed
