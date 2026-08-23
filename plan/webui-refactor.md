# WebUI Refactor Plan

**Date**: 2026-08-24 · **Status**: proposed, scheduled for the week of 2026-08-31
**Scope**: `webui/` — information architecture first, code structure second.

---

## 0. Why

The current UI is "a bit unintuitive". Root cause hypothesis: **the nav exposes the system's internal data tiers instead of user jobs.** The sidebar groups things as Per-session / Across-sessions — that's the distillation pyramid (sessions → atoms → scenes → memories), which is our architecture, not what a user wants to *do*.

## 1. Current state (audited 2026-08-24)

- Stack: React + Vite + Tailwind + react-router, ~15 pages, i18n, per-agent integration panels. Fine — no stack change needed.
- `SettingsPage.tsx` is the giant: **604 lines** with its own URL-section parser (`lib/settingsRoutes.ts` → `parseSettingsPath`).
- Route churn artifacts: `RedirectSettingsIntegrations` shim, `/pipeline` → `/` redirect.
- `lib/` carries ~10 `*Mock.ts` files alongside real API clients (`api.ts`, `setupApi.ts`, `onboardingApi.ts`) — legacy scaffolding, dead weight.
- Sidebar (`components/Sidebar.tsx`, 306 lines) organized by data tier:
  - Current: `Home / Per-session / Across-sessions / Integrations` (system tiers)
  - Candidate: job-oriented groups like `Overview / Agents / Memories / Activity`

## 2. Principles

### 2.1 Two separate refactors — do not merge them

"Unintuitive" is a UX/IA problem, not a code problem. Doing both at once is how rewrites die.

- **Refactor A — Information architecture**: nav structure, page purposes, flows.
- **Refactor B — Code structure**: split SettingsPage, kill mocks, extract shared components.

**A first.** Cheaper, higher impact, and it reshapes what B needs to look like.

### 2.2 Audit before designing

Same pattern as the backend: `docs/audit/2026-08-22-memory-retrieval-audit/` → MASTER-REPORT → remediation plan. Lightweight version for the UI:

1. **Task inventory** — the 5 top user jobs:
   - verify an agent is set up correctly
   - review what a session distilled
   - find/edit a memory
   - fix a broken model config
   - see if recall is working
2. **Friction map** — for each job: click count, dead-ends, places where the actual user (Colin) hesitates.
3. Output: `docs/audit/webui-ux-audit.md`. A solo screencast walkthrough + notes is enough — future-us diffs against it.

### 2.3 Strangler fig, not rewrite

No new SPA bootstrap. Refactor **one route at a time** inside the existing router:

- Rebuild the *worst* page first as the reference implementation (page structure, data fetching, loading/error states).
- Once one page proves the pattern, roll it across the rest.
- Delete dead weight as you go (mocks, redirect shims) — each merged page should leave `src/` smaller.

### 2.4 Reshape the shell cheaply

`Layout` + `Sidebar` is the spine; nav changes are low-risk (no logic). Sketch the new IA on paper first, then **walk the task inventory through the new nav before writing code** to validate it.

## 3. Execution order

```
week 1 (2026-08-31):  task inventory + friction map (no code)
                      paper sketch of new IA; walk tasks through it
week 2:               sidebar/shell restructure + route cleanup (delete redirects)
                      delete unused mocks
week 3+:              page-by-page strangler, starting with SettingsPage
```

## 4. First targets when work starts

1. `docs/audit/webui-ux-audit.md` — 30-minute exercise; makes every later decision obvious.
2. `SettingsPage.tsx` — split per section; retire `parseSettingsPath` in favor of plain routes.
3. Route cleanup — remove `RedirectSettingsIntegrations`, `/pipeline` redirect.
4. Mock purge — delete unused `*Mock.ts` from `lib/`.

If it grows into a visual redesign, load the `redesign-existing-projects` skill at that point — not before. IA-first.
