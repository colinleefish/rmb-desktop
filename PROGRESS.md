# PROGRESS.md

## Current State

**main** @ `6d2eace` after merging #78 (F11), #77 (B03), #79 (B05), #80 (B01/B02 hygiene). VERSION is still `0.2.10` on main; next bump target `0.2.11-dev.1`.

This branch (`feat/drop-corrections-tab`): **F12** — Memories Corrections tab removed. Corrections stay distill-time pins (not auto-absorbed). Add/retract remain on the memory detail. `/memories/corrections` redirects to All.

## In Progress

- `feat/drop-corrections-tab` / `../rmb-desktop-drop-corrections-tab` — F12 (planned → verify)
- `fix/B04-flaky-daemon-log-poll` — B04 (#70), existing worktree

## Next Steps

1. `make verify-feature F=F12 A=1` after commit; PR for F12
2. Bump `Makefile` VERSION → `0.2.11-dev.1` when the next ship is ready
3. B04 (#70); optional close #17 (AgentChip already on main) / #40 (release `ci` superseded)
4. F05 settings strangler per `plan/webui-refactor.md`

## Blockers

None.
