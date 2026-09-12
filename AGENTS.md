# AGENTS.md — rmb-desktop

This is **rmb-desktop**: a local-first memory product for AI coding agents. Go daemon (`rmbd`) + CLI (`rmb`) + menu-bar shell (`rmb-app`) + embedded React webui. It captures agent conversation turns, distills them into structured memories (scenes → atoms → memories pyramid), and serves hybrid recall (FTS + vectors) across sessions. Read `README.md` for the product story, `plan/` for active plans.

**Verification: `make check` must exit 0 before every commit.** (webui typecheck/lint/build via `webui-verify`, embed artifact check, vet + build + test + recall-eval gate.)

## Clock-in (before touching code)

1. Run `make session-start` (appends the session trace event).
2. Read this file, then `PROGRESS.md` (current state + next steps).
3. Run `make check` — confirm a green baseline before you start. If red, stop and report; do not build on red.
4. Declare your branch + worktree per [`plan/parallel-work-and-versioning.md`](plan/parallel-work-and-versioning.md) §2 (one task = one branch = one worktree; never work on the main checkout).
5. Fill `templates/sprint-contract.md` for the feature (scope in/out, DoD, timebox) before implementing.

## Clock-out (before closing)

1. Update `PROGRESS.md`: Current State (last commit + check status), Next Steps, Blockers.
2. Update `docs/quality-document.md` grades for the module you touched.
3. Run `make clean-check` (5 clean-state dimensions, idempotent) — all must pass.
4. Run `make check`; if you touched docs/plans only, say so explicitly in the commit.
5. `make session-end` (appends the session trace event).
6. Commit — the repo must be in a consistent state after every commit (one logical operation per commit, no partial work).
7. Leave the worktree clean (commit or stash everything; no drifting artifacts).

## Hard constraints

Each rule carries a `source:` — do not remove a rule without addressing its source.

- MUST NOT commit changes on `main` directly from an agent session (docs-only commits excepted). Branch + worktree always. `source: plan/parallel-work-and-versioning.md §1`
- MUST merge with merge commits (no squash/rebase of pushed branches) — every agent's work stays traceable. `source: plan/parallel-work-and-versioning.md §1`
- MUST NOT edit files under `internal/http/static/web/` by hand — that is the go:embed target of `webui/dist`, produced only by `make webui-build`. `source: Makefile webui-embed-check`
- MUST NOT resurrect Tauri-era files (`app/package.json`, `app/src-tauri/`). `source: plan/done/tauri-to-go-shell.md`
- MUST NOT hand-edit `VERSION` anywhere except `Makefile` `VERSION ?=` (single source of truth). `source: .cursor/rules/release.mdc`
- MUST update docs/plans in the same commit as the code change they describe — no stale documentation. Completed plans move to `plan/done/` in the same commit that completes them. `source: repo convention 2026-08-24`
- MUST NOT commit secrets/API keys; release credentials live in `~/.rmb/release.env` (see `scripts/release.env.example`). `source: Makefile release`
- WIP=1: only one feature may be `active` in `feature_list.json` at a time. `source: harness L07`

## State files (where truth lives)

| File | Purpose |
|---|---|
| `PROGRESS.md` | Current state, in-progress, next steps, blockers — update every clock-out |
| `DECISIONS.md` | Resolved decisions + rationale, one line each, pointer to the full plan |
| `feature_list.json` | Machine-readable feature states + verification + evidence (see #56) |
| `bug_list.json` | Machine-readable bug states (three-phase workflow, schema `bug_list.schema.json`, gates `make bug-state` / `make verify-bug`) |
| `plan/*.md` | Active plans; `plan/done/` = completed |
| `docs/audit/` | Audit reports feeding plans |

Chat history is not state. If you learned it and it matters, write it to a state file.

## Commits

Write commit messages that explain **why**, not just what. Example: `docs(plan): move completed tauri-to-go-shell plan to plan/done/; update remediation status` (the "why" = lifecycle rule, the "what" = file moves).

## Context anxiety

If you are running low on context: **do not rush to finish**. Stop, update `PROGRESS.md` (next steps specific enough for a fresh session to resume), commit a clean checkpoint. An honest checkpoint beats a botched finish.

## Observability (L12)

- **Before each feature**: sprint contract (`templates/sprint-contract.md`) — scope/DoD/exclusions, filled at clock-in.
- **During**: `make session-start` / `make session-end` append structured events to `.harness/traces/traces.jsonl` (gitignored runtime artifact).
- **After completion**: score the sprint against `templates/evaluator-rubric.md` (correctness / arch compliance / test coverage / verification evidence; every dimension ≥ B) and update `docs/quality-document.md` for touched modules.

**Dual-mode cleanup**: immediate cleanup at every session end (`make clean-check`) + a periodic (weekly-ish) full sweep for structural drift — re-run the harness audit (`temp/learn-harness-engineering/tools/audit-harness.sh .`), re-score the quality document, promote recurring review findings into `.harness/arch-rules.json`.

## Architecture Boundaries (L09)

Layer model: `cmd/` thin entrypoints (user-facing CLI output lives here) → `internal/<domain>` packages (structured logging only, no fmt prints) → `webui/` SPA (compiled to `internal/http/static/web` **only** via `make webui-build`). Boundary rules live in `.harness/arch-rules.json` and are enforced by `make check-arch` (part of `make check`); each rule prints WHAT/WHY/FIX on violation.

**Promotion principle:** every new error category caught in code review becomes a rule in `.harness/arch-rules.json` — with its `source:` — in the same commit that surfaced it.

## Feature List Rules (L07/L10/L11)

`feature_list.json` is the machine-readable feature tracker. State machine: `planned → active → passing` — **no skipping states, and NEVER set state to passing by hand**; only `make verify-feature F=<id>` may, by running the feature's layers.

- Activate: `make verify-feature F=<id> A=1` (enforces WIP=1 — refuses if another feature is active).
- Each feature carries `layers`: L1 static/syntax → L2 runtime behavior → L3 system confirmation. Do not proceed to layer N+1 if layer N fails. **Definition of Done = runtime evidence passes, not "the code is written" or "the agent is confident".**
- Granularity: each feature must be completable in one session. If it spans sessions, split it.
- New features get an entry (with verification layers) *before* implementation starts.

## Bug workflow (three-phase, issue #63)

Bugs live in `bug_list.json` (schema: `bug_list.schema.json`; gates: `scripts/verify-bug.sh`). Every bug = one GitHub issue + one entry. **One phase = one agent session**; phases 1 and 2 exist to hand the fixer enough context that phase 3 never re-investigates.

State machine: `reported → investigating → diagnosed → fixing → passing` — advance with `make bug-state B=<id> S=<state>` (gated, adjacent transitions only); `passing` only via `make verify-bug B=<id>`, **never by hand** (same rule as features). Run the gates inside the bug's worktree.

1. **Triage** (anyone): file the issue — title `bug(<scope>): <symptom>` (flaky tests: `flaky: <TestName> <behavior>`), labels `bug` + `area/<scope>` + `sev-high|sev-medium|sev-low` — then add the entry to `bug_list.json` (state `reported`). Fill `templates/bug-report.md` as the issue body; humans get the same form at `.github/ISSUE_TEMPLATE/bug.md`, and the triage agent normalizes title/labels/entry.
2. **Investigation** (dedicated session): claim (`→ investigating`), create the branch `fix/B<nn>-<slug>` + worktree, then produce `docs/bugs/B<nn>-investigation.md` from `templates/bug-investigation.md` **plus a regression test that fails on unfixed code** (tag it `// B<nn>`; record `regression {file, cmd}` on the entry). The `→ diagnosed` gate runs the regression and **requires it to fail** — a passing test means the bug was not reproduced. Investigation artifacts live on the bug branch so main never carries a red test.
3. **Fix** (new session, same branch): read the investigation doc, make the regression green, fill a sprint contract, then `make verify-bug B=<id>` (runs the entry's L1/L2/L3) → `passing`. Merge via PR with a green `pr-check` (F09). After merge: close the issue and append `issue closed` to the entry's evidence.

Rules:

- **Bug WIP**: at most one bug in `fixing` (enforced by the gate). Bugs do NOT count against feature WIP=1 — prod fixes may interleave with an active feature; path collisions are settled by the §2.5 ownership table in `plan/parallel-work-and-versioning.md`.
- Backfilled entries may point `investigation` at pre-convention docs (e.g. `docs/audit/...`); the field is authoritative.

Scope ↔ module map (extend as needed): `zcode`→`internal/hook`, `worker`→`internal/worker`, `recall`→`internal/recall`, `db`→`internal/db`, `cli`→`cmd/rmb`, `update`→`internal/update`, `webui`→`webui/`, `shell`→`cmd/rmb-app`.

## Tools

- Build/test: `make` targets only (`check`, `test`, `eval`, `build`, `dev`, `setup`, `verify-bug`, `bug-state`) — do not invoke raw go/pnpm commands that bypass the pipeline. Webui deps: `cd webui && pnpm install` (or `make setup`).
- GitHub access goes through the local proxy: `bash scripts/with-proxy.sh gh ...` / `bash scripts/with-proxy.sh git fetch` (issue/label for the bug workflow, pr/run/api for the CI gate).
- Release pipeline: `make release VERSION=x.y.z` — see `.cursor/rules/release.mdc`; run it yourself when asked, credentials from `~/.rmb/release.env`.
- Tool permissions: `.claude/settings.json` scopes agent tooling (git/make/go/gh allowed; destructive ops denied).
- No MCP servers are wired for this repo yet; add them to `.mcp.json` + document here first.

## Topic docs

- Branching / parallel agents / versioning: [`plan/parallel-work-and-versioning.md`](plan/parallel-work-and-versioning.md)
- WebUI refactor (IA-first): [`plan/webui-refactor.md`](plan/webui-refactor.md)
- Product roadmap: [`plan/local-first-desktop.md`](plan/local-first-desktop.md)
- Retrieval audit + remediation: [`docs/audit/2026-08-22-memory-retrieval-audit/`](docs/audit/2026-08-22-memory-retrieval-audit/MASTER-REPORT.md), [`plan/memory-retrieval-remediation.md`](plan/memory-retrieval-remediation.md)
