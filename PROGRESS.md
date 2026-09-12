# PROGRESS.md

## Current State

Last commit: `feat/webui-ia-shell` (549adc1 + follow-up) — integrates **chore/pnpm** (3110269) + **F04 webui shell restructure** on top of main @ 706d07f (F10 merged). `make check` and `make webui-build`: green locally (2026-09-12). F04 `active`; verify via `make verify-feature F=F04` before merge.

- **pnpm**: Makefile/CI use `pnpm install --frozen-lockfile`; `webui/pnpm-lock.yaml` replaces `package-lock.json`.
- **F04**: Job-oriented shell (Overview · Sessions · Memories · Agents · Settings), Topbar recall bar, Agents hub (integrations + skills), memory category tabs, daemon heartbeat, corrections `target_uris` in API + UI. Contract: `.harness/contracts/F04.md`, design notes: `docs/design/webui-design.md`.
- **Ship**: push `feat/webui-ia-shell` → PR → merge → bump VERSION (0.2.10) → `make release`.

## In Progress

- PR for F04 + pnpm awaiting CI (`pr-check`).

## Next Steps

1. Merge PR; release 0.2.10
2. B04 phase-3 fix (`fix/B04-flaky-daemon-log-poll`)
3. B05 deterministic regression + fix
4. F05 settings strangler per `plan/webui-refactor.md`

## Blockers

None.
