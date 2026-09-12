# WebUI design spec (visual pass on Refactor A)

**Date**: 2026-09-12 · **Branch**: `feat/webui-ia-shell` · **Status**: applied to `webui/src` (mock server)
**Scope**: visuals only. IA (flat nav, tabs, routes) is fixed by `plan/webui-refactor.md` and the IA wireframe.

Design read: redesign-preserve of a local-first dev-tool dashboard for one technical user, Linear-style
minimalist language, Tailwind v4 utilities + system font + hairlines. Dials: variance 3 / motion 2 / density 5.
Brand accent `#00adb5` stays (memory: `rmb://preferences/color-palette`); canvas goes white
(`rmb://preferences/rmb-webui-background`); flat, light, minimal borders (`rmb://preferences/flat-ui`).

## 1. Feature inventory

| Route | Job | Data shown | Primary action | Secondary | Noise flagged |
|---|---|---|---|---|---|
| `/` Overview | J5 (routes J1/J4) | counts, `memory_by_category`, pipeline health (funnel, stages, problems), latest sessions | open a "Needs attention" row | open Sessions / Memories | 4 KPI cards + hero strip + funnel + 3 tier cards all above the fold; T1/T2/T3 labels; "snapshot" timestamp |
| `/sessions` | J2 | date-grouped session rows | open a session | page / page size | table chrome (thead, 6 columns), T1/T2/T3 LEDs + a second status pill per row, UID column label |
| `/sessions/:key` | J2 | session, turns, atoms, scenes, pipeline state | read turns | switch atoms / scenes | T1/T2/T3 pills first, before turns |
| `/memories[/:cat]` | J3 | memories (title, abstract, version, recall stats, updated) | open a memory (modal) | tab, sort, page | 8-column table; three separate recall columns (search / cat / meta); version column |
| `/memories/profile` | J3 | single profile memory | read / correct | — | fine |
| `/agents` | J1 | 7 agents × setup state (`detected`, hook / recall status, `lastHookAt`) | open a broken agent | open connected agent | status as plain gray text; every card looks the same regardless of state |
| `/agents/:id` | J1 | setup checklist (hook, recall, verify) with diffs | apply a step | back to grid, pick another card | double frame (card in card), `/35` border + shadow |
| `/agents/skills` | J1 (playbooks) | skills (name, tags, description, version, recall) | open a skill | search, page | violet icon box + emerald / violet tag colors off-palette; "N revisions" |
| `/agents/skills/:slug` | J1 | SKILL.md tree + file viewer | read | pick file | fine |
| `/settings/*` | J4 | config (general / models / advanced) + connection test | save | restart, reset | card with `/35` border + shadow; tabs styled differently from PageTabs |

Redundancies removed: Sessions status shown twice (pill + LEDs); Overview counts shown three times (KPI cards,
funnel, stage totals). Internal tiers (T1/T2/T3, pending/running/idle/waiting) are moved behind a disclosure.

## 2. Design principles

- Clean and concise: one type scale, one accent, hairlines instead of shadows, no decorative gradients or dots.
- One primary action per screen; secondary actions are quiet text links, never a second filled button.
- Status before detail: the first thing on every list row / card is a status pill or the answer to "is it working".
- Pipeline internals (T1/T2/T3, worker states, funnel drop) live behind a collapsed "Pipeline details" disclosure.
- White surface everywhere; sidebar, topbar and canvas share white, separated only by 1px `rmb-line`.
- Consistent density: rows are 56px, cards `p-4`, sections `space-y-8`, page padding `px-8 py-6`.
- Never invent metrics the API lacks (no recall hit-rate, no p95): show counts that exist and link to detail.

## 3. Design system

### Tokens (`webui/src/index.css` `@theme`, additive)

| Token | Value | Role |
|---|---|---|
| `rmb-dark` | `#222831` | primary text, active nav, KPI numerals |
| `rmb-gray` | `#393e46` | (kept) strong secondary text |
| `rmb-accent` | `#00adb5` | the only accent: links, active tab underline, primary button, "ok" pill |
| `rmb-light` | `#eeeeee` | (kept for compat) no longer used as canvas |
| `rmb-muted` | `#646b76` | secondary text (labels, abstracts) |
| `rmb-faint` | `#9096a0` | tertiary text (timestamps, uris) |
| `rmb-line` | `#e7e9ec` | hairline borders, dividers |
| `rmb-line-strong` | `#d3d7dc` | input borders |
| `rmb-fill` | `#f5f6f7` | hover rows, kbd, neutral pill, disclosure body |
| `rmb-danger` | `#b42318` | failed / attention text; `rmb-danger-soft` `#fdecea` fill |
| `rmb-warn` | `#9a5b00` | setup incomplete text; `rmb-warn-soft` `#fbf3db` fill |

Type scale (system font, tabular numerals globally): page title 15/600 · section title 13/600 · body 13/400 ·
meta 12/400 · KPI 28/600 tight. Radius: cards / inputs / buttons `rounded-md` (6px), pills `rounded` (4px),
modal `rounded-lg`. Borders: 1px `rmb-line` only; shadow only on the modal (`0 12px 40px rgba(34,40,49,.14)`).
Motion: `transition-colors` 150ms on hover/focus only.

### Components (`webui/src/components/`)

- `StatusPill` — `tone: ok | neutral | attention | danger | info`, 11px medium, `rounded px-1.5 py-0.5`. Used for
  distilled / pending / failed, connected / setup incomplete / not set up, memory category, skill tags.
- `AgentChip` — agent logo (from `integrations/registry`) + label; resolves session `source` values.
- `EmptyState` / `ErrorNote` / `ListSkeleton` (`EmptyState.tsx`) — the three non-happy states, same width as a row.
- `PageHeader` — back link · title · description · meta row · actions. For detail routes only (topbar owns list titles).
- `SectionHeader` — 13px title + optional hint + optional right-side link ("View all").
- `Disclosure` — native `<details>` with a hairline summary row; hides pipeline internals.
- `SessionListRow` — the one session row (chip · key + abstract · pill · turns · time), shared by Sessions and Overview.
- `PageTabs` (kept) — underline tabs, badge as faint numeral. `Pagination` (kept) — hairline top, no card.
- `Modal` (kept) — hairline header, single soft shadow.

### States

Loading → `ListSkeleton` (3–5 rows of the final row shape). Empty → `EmptyState` with one next action.
Error → `ErrorNote` inline (red text on soft red fill), never `alert()`.

## 4. Per-screen layout

```
Shell
+---------------+------------------------------------------------------------+
| logo RMB      | Title  subtitle                       [ Find a memory… ⌘K ] |  56px
| Overview      +------------------------------------------------------------+
| Sessions  12  |  page canvas (white, px-8 py-6, max-w-7xl)                  |
| Memories  34  |                                                            |
| Agents        |                                                            |
|               |                                                            |
| Settings      |                                                            |
| ● rmbd · mock |                                                            |
+---------------+------------------------------------------------------------+
```

**Collapsible shell** — sidebar defaults to expanded (`w-56`). A focusable seam toggle sits on the
sidebar’s right hairline, vertically centered (`aria-expanded`, chevron), half on the rail and half
on the main column; it collapses to a `w-14` icon rail: nav labels and count numerals hide;
counts appear in hover tooltips. Settings and
the rmbd heartbeat compress to icons / dot with tooltips. Preference persists in
`localStorage` (`rmb.sidebarCollapsed`). Width animates `200ms ease`; main column stays
`flex-1 min-w-0` so the topbar and canvas fill the remainder without horizontal body scroll.

**Overview** — kept: three loop tiles (Capture: sessions + turns + tracked · Distillation: sessions fully distilled
(`funnel.t3_done`) + on/off pill + atoms / scenes / workers running · Memories: total + category counts + corrections + skills),
"Needs attention" (problems + a "distillation off" row linking to Settings › Models), "Latest sessions" (5 rows).
Moved behind `Pipeline details` (collapsed): funnel with drop %, three stage cards, snapshot time.
Removed: hero strip, 4 KPI cards + secondary stat line (duplicated the tiles), standalone pyramid.

**Sessions** — kept: date groups, pagination, page size. Row = `AgentChip · session_key / abstract · StatusPill ·
N turns · HH:MM`. Removed: table header, UID label, T1/T2/T3 LEDs (tooltip on the pill keeps the raw statuses).

**Session detail** — `PageHeader` (back, key, abstract, meta: chip · status pill · turns · time) → one-line value
funnel `turns → atoms → scenes` → tabs Turns / Atoms / Scenes. T1/T2/T3 pipeline state inside `Pipeline details`.

**Memories** — Topbar stays section-level (`shell.memoriesTitle` / `shell.memoriesSubtitle`). Under `PageTabs`, tab heading uses `getMemoriesSectionMeta` (category title + `t.memories.categories.*.subtitle`, or All). Tabs (All · Profile · Events · Preferences · Entities); tab and row category pills use `t.memories.categories.*.nav` (localized in zh). Toolbar: count + sort
(Updated / Most recalled / Version). Row = title + uri / abstract · category pill (All only) · recalls (sum, tooltip
breaks down search / cat / meta) · updated. Modal: title, uri, abstract, body, meta row, corrections (add / retract). Profile: plain
article. `/memories/corrections` redirects to `/memories` — corrections are not a memory category.

**Agents** — Integration tab: 3-col card grid, status-first: `logo · name · StatusPill` then one line
(`Last capture · time` or "no hook installed") then the single CTA (Details / Fix setup / Set up). Setup-incomplete
cards get a warm border. Agent detail: `PageHeader` back to **全部智能体** + full-width setup panel (no left agent
switcher — discovery is the grid only). Onboarding still uses the in-panel switcher. Skills tab: search + rows
(`name · tags / description · vN · recalls · updated`), no icon box.

**Settings** — kept everything (F05 split is out of scope). Tabs use PageTabs styling; card is a hairline frame;
footer bar is `rmb-fill`; inputs share `rmb-line-strong` border + accent focus ring.

## 5. Out of scope / later

- Refactor B code splits (`SettingsPage`, mock purge) — F05.
- Real recall-hit metrics (hit / miss, latency) — API has per-memory counters only; Overview shows none.
- Session filter by agent / status (`/sessions?source=`) — the paging API has no `source` param yet.
- Edit drawer instead of modal for memories; a ⌘K result palette instead of navigating to `/memories?q=`.
- Skeletons for the agent setup panel and onboarding flow (onboarding untouched in this pass).
- Sort by cat / meta recall counts individually was dropped from the UI (still supported by the API).
