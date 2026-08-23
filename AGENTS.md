# AGENTS.md — rmb-desktop

This is **rmb-desktop**: a local-first memory product for AI coding agents. Go daemon (`rmbd`) + CLI (`rmb`) + menu-bar shell (`rmb-app`) + embedded React webui. It captures agent conversation turns, distills them into structured memories (scenes → atoms → memories pyramid), and serves hybrid recall (FTS + vectors) across sessions. Read `README.md` for the product story, `plan/` for active plans.

**Verification: `make check` must exit 0 before every commit.** (vet + build + test + recall-eval gate.)

## Clock-in (before touching code)

1. Read this file, then `PROGRESS.md` (current state + next steps).
2. Run `make check` — confirm a green baseline before you start. If red, stop and report; do not build on red.
3. Declare your branch + worktree per [`plan/parallel-work-and-versioning.md`](plan/parallel-work-and-versioning.md) §2 (one task = one branch = one worktree; never work on the main checkout).

## Clock-out (before closing)

1. Update `PROGRESS.md`: Current State (last commit + check status), Next Steps, Blockers.
2. Run `make check`; if you touched docs/plans only, say so explicitly in the commit.
3. Commit — the repo must be in a consistent state after every commit (one logical operation per commit, no partial work).
4. Leave the worktree clean (commit or stash everything; no drifting artifacts).

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
| `plan/*.md` | Active plans; `plan/done/` = completed |
| `docs/audit/` | Audit reports feeding plans |

Chat history is not state. If you learned it and it matters, write it to a state file.

## Commits

Write commit messages that explain **why**, not just what. Example: `docs(plan): move completed tauri-to-go-shell plan to plan/done/; update remediation status` (the "why" = lifecycle rule, the "what" = file moves).

## Context anxiety

If you are running low on context: **do not rush to finish**. Stop, update `PROGRESS.md` (next steps specific enough for a fresh session to resume), commit a clean checkpoint. An honest checkpoint beats a botched finish.

## Feature List Rules (L07/L10/L11)

`feature_list.json` is the machine-readable feature tracker. State machine: `planned → active → passing` — **no skipping states, and NEVER set state to passing by hand**; only `make verify-feature F=<id>` may, by running the feature's layers.

- Activate: `make verify-feature F=<id> A=1` (enforces WIP=1 — refuses if another feature is active).
- Each feature carries `layers`: L1 static/syntax → L2 runtime behavior → L3 system confirmation. Do not proceed to layer N+1 if layer N fails. **Definition of Done = runtime evidence passes, not "the code is written" or "the agent is confident".**
- Granularity: each feature must be completable in one session. If it spans sessions, split it.
- New features get an entry (with verification layers) *before* implementation starts.

## Tools

- Build/test: `make` targets only (`check`, `test`, `eval`, `build`, `dev`, `setup`) — do not invoke raw go/npm commands that bypass the pipeline.
- Release pipeline: `make release VERSION=x.y.z` — see `.cursor/rules/release.mdc`; run it yourself when asked, credentials from `~/.rmb/release.env`.
- Tool permissions: `.claude/settings.json` scopes agent tooling (git/make/go allowed; destructive ops denied).
- No MCP servers are wired for this repo yet; add them to `.mcp.json` + document here first.

## Topic docs

- Branching / parallel agents / versioning: [`plan/parallel-work-and-versioning.md`](plan/parallel-work-and-versioning.md)
- WebUI refactor (IA-first): [`plan/webui-refactor.md`](plan/webui-refactor.md)
- Product roadmap: [`plan/local-first-desktop.md`](plan/local-first-desktop.md)
- Retrieval audit + remediation: [`docs/audit/2026-08-22-memory-retrieval-audit/`](docs/audit/2026-08-22-memory-retrieval-audit/MASTER-REPORT.md), [`plan/memory-retrieval-remediation.md`](plan/memory-retrieval-remediation.md)
