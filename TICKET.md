# Current Ticket: UNIC-2

## TUI Shell

**Status**: Not Started
**Milestone**: M1.2
**Priority**: Highest

---

### Summary

Implement a Ratatui + Crossterm TUI shell that serves as the interactive foundation for all AWS service modules.

### Tasks

- [ ] Implement screen router with back navigation stack (`src/tui/router.rs`)
- [ ] Build main menu view with service list navigation (`src/tui/views/`)
- [ ] Create shared layout: header (profile/region/account), body, footer (keybindings)
- [ ] Build reusable widgets: filterable list, confirmation dialog, notification bar, loading spinner (`src/tui/widgets/`)

### Acceptance Criteria

- Main menu displays a navigable list of services
- Screen router supports push/pop navigation with back key
- Header shows current profile, region, and account info
- Footer displays context-sensitive keybindings
- Reusable widgets work across different views

### Tech Notes

- Prerequisite: M1.1 (Config & Profile Management) is complete
- All auth (M2) and service modules (M3) will be built on top of this TUI shell
- Dependencies: `ratatui`, `crossterm` (already in Cargo.toml)

### Files

- `src/tui/router.rs` — screen navigation
- `src/tui/views/` — per-service TUI screens
- `src/tui/widgets/` — reusable components

### Related Issue

Closes #4
