# PROGRESS.md

## Current State

Last commit on **main**: `d83070f` — VERSION **0.2.10** shipped (F04 webui IA shell + pnpm). `make check` green on main when `webui-build` artifact present (2026-09-12).

**Open PRs (merge order suggestion: #78 → #77 → #79, or parallel after CI green):**

| PR | Branch | Closes |
|----|--------|--------|
| [#78](https://github.com/colinleefish/rmb-desktop/pull/78) | `chore/webui-verify-gate` | #60 (F11 webui-verify in `make check`) |
| [#77](https://github.com/colinleefish/rmb-desktop/pull/77) | `fix/B03-tray-update-baseline` | #15 (B03 sidecar version baseline) |
| [#79](https://github.com/colinleefish/rmb-desktop/pull/79) | `fix/B05-incumbent-materiality-race` | #72 (B05 L3 rollup URI lock) |

## In Progress

- `chore/bug-track-hygiene` — backfill B01/B02 `passing` + `// B01`/`// B02` regression tags (fixes merged pre-F08).
- `fix/B04-flaky-daemon-log-poll` — B04 (#70), existing worktree.

## Next Steps

1. Merge PRs #78–#79; close #60/#15/#72 on merge.
2. Merge `chore/bug-track-hygiene` after bug_list backfill review.
3. Comment/close #17 (Sessions logos — `AgentChip` on main since F04); optional #40 (release `ci` workflow superseded by `pr-check` + local `make release`).
4. F05 settings strangler per `plan/webui-refactor.md`.

## Blockers

None.
