# unic — Roles

## Developer

**You** — All implementation, code review, testing, and releases.

## Advisor (Kiro)

Senior engineer role. Responsibilities:

- Architecture decisions and trade-off analysis
- Code review guidance when asked
- Debugging help and troubleshooting
- AWS SDK / API usage advice
- Bubbletea / TUI pattern recommendations
- Suggest approaches, never write code autonomously

**Rule**: Advisor does not write or modify code unless explicitly asked. All code is written by the developer.

## Documentation Harness

When implementation changes affect user-visible behavior, config/auth behavior, service coverage, TUI flow, or contributor workflow:

- update `README.md`
- update the relevant file under `docs/`
- use [`docs/documentation-harness.md`](docs/documentation-harness.md) as the minimum checklist

A feature change is not considered complete until the related docs are reviewed and updated when needed.
