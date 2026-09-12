# PROGRESS.md

## Current State

**main** @ `6d2eace` after merging #78 (F11), #77 (B03), #79 (B05), #80 (B01/B02 hygiene). VERSION is still `0.2.10` on main; next bump target `0.2.11-dev.1`.

This branch (`feat/drop-corrections-tab`): **F12 passing** (`f7e66a7`). Memories Corrections tab removed. Corrections stay distill-time pins (not auto-absorbed). Add/retract remain on the memory detail. `/memories/corrections` redirects to All. `make check` green (2026-09-12).

## In Progress

- F12 passing on `feat/drop-corrections-tab` (awaiting PR)
- `fix/B04-flaky-daemon-log-poll` — B04 (#70), existing worktree

## Next Steps

1. PR for F12 (`feat/drop-corrections-tab`)
2. Bump `Makefile` VERSION → `0.2.11-dev.1` when the next ship is ready
3. B04 (#70); optional close #17 (AgentChip already on main) / #40 (release `ci` superseded)
4. F05 settings strangler per `plan/webui-refactor.md`

## Blockers

None.
