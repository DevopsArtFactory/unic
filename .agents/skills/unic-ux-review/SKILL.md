---
name: unic-ux-review
description: Use when the user asks to run unic locally, capture TUI screenshots, review the Bubble Tea UX, suggest UX improvements, or optionally implement scoped UX polish in the unic repository. This skill is for terminal UI review and safe UX iteration, not for unrelated feature work, dependency updates, PR shipping, or non-UX code review.
---

# unic UX Review

Review `unic` by running the real TUI, capturing reproducible screenshots, and
turning the evidence into prioritized UX improvements. Implement only when the
user asks for changes or clearly invites follow-through.

## Workflow

1. Start with repo safety.
- Run `git status --short --branch`.
- Note existing untracked or modified files and do not touch unrelated work.
- If implementation will become PR work and no GitHub issue is referenced,
  call out the repo rule that new work should map to an issue before shipping.

2. Capture the current UI.
- Prefer the bundled harness:
  `python3 .agents/skills/unic-ux-review/scripts/capture_unic_tui.py`
- The harness builds `./unic`, creates an isolated XDG config with fake
  contexts, disables EC2 metadata, seeds the update cache, launches the real TUI
  in `tmux`, captures the context picker and service picker, renders PNGs when
  Pillow is available, and kills the `tmux` session.
- If the command fails with a `tmux` socket or sandbox permission error, rerun
  the same command with escalated permissions. Do not work around this by using
  the user's real config.
- Use `--out-dir /private/tmp/unic-ux-review` when you want predictable output.
- Use `--skip-build` only after confirming a fresh `./unic` binary exists.

3. Inspect the artifacts.
- Open the generated PNGs with `view_image`.
- Also inspect the raw `.txt` captures when alignment, wrapping, or truncation
  needs exact terminal text.
- Keep screenshot paths in the final answer so the user can open them.

4. Review UX with a TUI-specific rubric.
- Workflow clarity: does the first screen explain where the user is and what
  action is expected?
- Layout responsiveness: do panels use the available terminal width without
  creating awkward empty regions?
- Text fit: do status bars, help bars, table cells, and labels wrap or truncate
  in avoidable ways at common sizes?
- Visual hierarchy: can the user scan title, current context, selected row,
  metadata, and available actions in that order?
- Semantic consistency: do symbols such as `*`, cursor markers, colors, and
  key names mean the same thing across screens?
- Keyboard discoverability: are frequent actions visible without making the
  help bar noisy or wrapped?
- Existing style fit: preserve `unic`'s current Bubble Tea/Lip Gloss patterns,
  column-aligned tables, dim labels, and compact terminal-first density.

5. Suggest before changing when scope is broad.
- Lead with concrete findings tied to screenshot evidence.
- Prefer small, testable improvements such as table width allocation, help-bar
  shortening, consistent markers, responsive panel widths, and clearer labels.
- If the user asked to "maybe implement", choose only low-risk improvements
  that are directly visible in the captured screens. Ask before larger redesigns
  or behavior changes.

6. Implement scoped improvements when appropriate.
- Read the owning files before editing, usually `internal/app/styles.go`,
  `internal/app/app.go`, `internal/app/screen_context.go`,
  `internal/app/context_table.go`, and nearby tests.
- Preserve existing Bubble Tea navigation and `visibleLines := max(m.height-N,
  5)` style windowing where applicable.
- Add or update tests for formatting helpers, view rendering, and key UX
  invariants when possible.
- Run `make test` and `make build` after behavior or rendering changes.
- Rerun the capture harness after edits and compare screenshots.
- Update `README.md` only if visible behavior, key bindings, or configuration
  semantics changed.

## Blocked Capture Fallback

If the real TUI cannot be launched in the current environment:

- Capture the exact failure and say why it blocks screenshot generation.
- Devise a plan using the same isolated-config approach on the user's machine.
- If useful, add or propose a test-only render harness that instantiates
  `app.Model` with fixture config and writes `View()` output at fixed sizes.
- Do not claim UX findings from imagined screenshots; separate hypotheses from
  observed evidence.

## Output Shape

For review-only work, report:

- screenshot paths
- 3-7 prioritized findings
- suggested implementation order
- any capture limitations

For implemented work, report:

- what changed
- before/after screenshot paths or a concise visual comparison
- tests and build commands run
- remaining UX follow-ups
