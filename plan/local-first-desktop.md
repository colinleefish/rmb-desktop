# rmb-desktop — local-first product plan

> Planning doc. Decisions marked ✅ are resolved; ⏸ are intentionally deferred.

## 1. Vision

**rmb-desktop** is a standalone, local-first memory product for AI coding agents.

- Capture conversation turns from Cursor, Claude Code, Codex, and similar tools.
- Distill them in the background into structured memories.
- Recall across sessions via hybrid search (full-text + vectors).
- Browse and manage everything in a web dashboard.

**Free (v1):** full single-device experience — install, capture, distill, recall, browse.

**Paid (later):** sync memories across devices.

### Product shape

Syncthing-style — not a heavy Electron shell:

```
┌─────────────────────────────────────────┐
│  menubar (Tauri)                        │
│  tray · setup wizard · open dashboard   │
└──────────────────┬──────────────────────┘
                   │ manages lifecycle
┌──────────────────▼──────────────────────┐
│  rmbd (daemon)                        │
│  hooks · workers · API · embedded UI  │
└──────────────────┬──────────────────────┘
                   │
┌──────────────────▼──────────────────────┐
│  SQLite + vector extension              │
│  ~/…/rmb-desktop/data/rmb.db            │
└─────────────────────────────────────────┘

CLI: `rmb` (hook-submit, search, setup, …) — agents and operators invoke this.
Daemon: `rmbd serve` — background process; menubar manages lifecycle.
```

### Principles

| Principle | Meaning |
|-----------|---------|
| **Standalone** | All memory, distillation, storage, hooks, API, and UI live in this repo. No runtime dependency on any other project. |
| **Local-first** | Data stays on disk; daemon runs at login; works without network (except LLM API calls for distillation). |
| **Self-hosted** | User owns the machine and the database file. No mandatory cloud. |
| **Cross-platform** | macOS, Linux, Windows — same product, per-OS installers. |
| **SQLite + vectors** | Single-file database with FTS5 and a vector extension. No external database server. |

### Goals

- Colleagues install without provisioning infrastructure.
- Agent hooks work while the browser UI is closed.
- Setup wizard configures hooks and API keys in one flow.

### Non-goals (v1)

- Multi-device sync (design for it; don't ship it).
- Bundled / subsidized LLM API keys.
- Competing with CC Switch (v1: no special integration; users manage agent config themselves).

---

## 2. Memory model

Greenfield design. Layered pyramid from raw chat to durable facts:

```text
                    ┌─────────────────────────────────┐
                    │  L3 — memories (cross-session)  │
                    │  profile · preferences · entities│
                    └───────────────┬─────────────────┘
                                    │
              ┌─────────────────────▼─────────────────────┐
              │  Session                                    │
              │  turns (L0) → atoms (L1) → scenes (L2)    │
              └─────────────────────────────────────────────┘
```

| Layer | What | Stable ID (`rmb://` scheme) |
|-------|------|-----------------------------|
| L0 | Raw user + assistant exchange | `rmb://turns/<uuid>` |
| L1 | Small extracted fact | `rmb://atoms/<uuid>` |
| L2 | Session-local narrative segment | `rmb://scenes/<uuid>` |
| L3 | Durable cross-session memory | `rmb://profile`, `rmb://entities/<slug>`, `rmb://preferences/<slug>`, … |
| — | Session container | `rmb://sessions/<uuid>` |
| — | Human override | `rmb://corrections/<uuid>` |
| — | Agent playbook | `rmb://skills/<slug>` |

All entities share a single **`rmb://`** prefix. Path segment identifies tier (`turns`, `atoms`, `scenes`, `memories`, …) — no separate schemes like `memory://` or `atom://`.

**Decision:** Unified `rmb://` URI scheme for every tier and entity type.

### ✅ D1 — URI scheme

Single prefix **`rmb://`** for all tiers and entities. Tier encoded in path segment — no separate schemes (`memory://`, `atom://`, etc.).

---

## 3. Storage — SQLite + vectors

### Why SQLite

- Industry standard for local-first desktop apps.
- Single file — easy backup, inspect, and (later) sync at the entity level.
- No separate database process to install or manage.
- Cross-platform with identical semantics.

### Schema approach

Design schema natively for SQLite from day one:

| Concern | Approach |
|---------|----------|
| Primary keys | `TEXT` UUIDs |
| Timestamps | `INTEGER` unix ms or `TEXT` ISO-8601 |
| Arrays (e.g. correction targets) | `JSON` text column |
| Memory versioning | `superseded_at` + partial unique index on active rows |
| Full-text search | **FTS5** virtual tables |
| Semantic search | **sqlite-vec** (or chosen extension) |
| Migrations | `goose`, `golang-migrate`, or `sqlc` + hand-rolled — pick one |

### ✅ D2 — Vector extension

| Option | Pros | Cons |
|--------|------|------|
| **[sqlite-vec](https://github.com/asg017/sqlite-vec)** | Purpose-built; cosine distance; small | CGO if using from Go; relatively new |
| **BLOB + brute-force** | No CGO; simple | O(n) — fine for personal scale (<100k vectors) |
| **In-memory ANN index** | Fast query; rebuild on start | RAM usage; persistence sync on shutdown |
| **FTS-only v1** | Ship faster | Weak semantic recall |

**Recommendation:** **sqlite-vec** for production path; spike early. Brute-force acceptable for alpha if CGO cross-compile is painful.

**Decision:** sqlite-vec

### ✅ D3 — Embedding model and dimensions

Cross-language recall (English query → Chinese memory, exotic scripts, mixed content) depends on choosing a **multilingual** embedding model — not an English-only one.

| Model (examples) | Dim | Notes |
|------------------|-----|-------|
| **BGE-M3** | 1024 | Strong multilingual; dense + sparse; good default candidate |
| **multilingual-e5-large** | 1024 | Solid general-purpose multilingual |
| **nomic-embed-text** | 768 | Lighter; weaker on low-resource languages |
| **OpenAI text-embedding-3-small** | 1536 | Decent cross-lingual via cloud API |
| **Cohere embed-multilingual-v3** | 1024 | Cloud; broad language coverage |

| Dimension strategy | Notes |
|--------------------|-------|
| **Configurable** | Store `dim` in config + DB metadata; re-embed all rows on model change |
| **Fixed per model** | Simpler schema; migration required to switch models |

**Recommendation:**

- Require a **multilingual** embed model (document supported defaults in setup wizard).
- **Configurable dimensions** — store `embed_dim` in config; default **1024** (BGE-M3 / multilingual-e5 fit).
- Re-embed job when user switches models.

**Decision:** Multilingual model required; configurable dimensions; default 1024.

### ✅ D4 — FTS tokenizer (multilingual)

FTS is a **lexical supplement** — exact keywords, hostnames, code tokens. Semantic recall across languages is handled by embeddings (D3), not FTS.

No single FTS tokenizer handles all languages well:

| Language type | FTS challenge |
|---------------|---------------|
| CJK (Chinese, Japanese) | No word boundaries; needs segmentation for good FTS |
| Thai, Khmer, Lao | No spaces between words |
| Arabic, Persian | Diacritics, morphological variants |
| Agglutinative (Turkish, Finnish) | Language-specific stemming |
| Low-resource / exotic | No stemmer exists |

| Option | All languages? | Notes |
|--------|----------------|-------|
| **Porter stemming** | No | English only — **reject** |
| **FTS5 `unicode61`** | Baseline | Safe for all Unicode scripts; no stemming; won't break exotic scripts |
| **FTS5 `trigram`** (SQLite 3.34+) | Partial | Substring matching; helps CJK and no-space languages |
| **Custom ICU tokenizer** | Best FTS | Per-locale word breaking; heavy dependency — defer |
| **Per-document language tag → tokenizer** | Best quality | Complex; defer |

**Recommendation:**

- **Primary:** `unicode61` on all FTS tables — safe default for every script.
- **Optional:** `trigram` virtual table or second FTS index for substring / CJK recall.
- **Reject:** Porter and any English-only stemmer.
- **Defer:** ICU custom tokenizers and per-language indexing until eval proves FTS gaps.
- Build a **multilingual eval set** (EN, ZH, plus at least one no-space language e.g. Thai, and one RTL e.g. Arabic) before beta.

**Decision:** `unicode61` default; optional `trigram` leg; multilingual recall via embed model (D3); no Porter.

### ✅ D5 — Database file location

| Path | Notes |
|------|-------|
| `~/.rmb-desktop/data/rmb.db` | Matches repo name; clear ownership |
| Platform app-data dir | `~/Library/Application Support/rmb-desktop/` etc. — more conventional on each OS |
| User-configurable | Override via config file |

**Recommendation:** Platform app-data dir with config override. Default:

- macOS: `~/Library/Application Support/rmb-desktop/data/rmb.db`
- Linux: `~/.local/share/rmb-desktop/data/rmb.db`
- Windows: `%APPDATA%\rmb-desktop\data\rmb.db`

**Decision:** Platform app-data dir with config override.

### ✅ D6 — Worker coordination (no DB server locks)

Background workers (L1 extract, L2 scene, L3 memory) need mutual exclusion.

| Option | Notes |
|--------|-------|
| **`BEGIN IMMEDIATE` transactions** | SQLite write lock per operation |
| **App-level mutex** | Single-process daemon — simple `sync.Mutex` |
| **Lease table in SQLite** | `worker_leases` row with expiry — survives crash |

**Recommendation:** App-level mutex for v1 (single daemon process). Lease table if we ever split workers into separate processes.

**Decision:** App-level mutex

---

## 4. Backend architecture

All engine code lives in this repo.

### ✅ D7 — Backend language

| Option | Pros | Cons |
|--------|------|------|
| **Go** | Fast compile; good HTTP/SQLite ecosystem; single binary | sqlite-vec needs CGO |
| **Rust** | Natural fit with Tauri; excellent SQLite crates | Steeper hook/agent adapter work |
| **TypeScript (Node)** | Same language as UI; `better-sqlite3` | Worker long-running process; less ideal for CLI hooks |

**Recommendation:** **Go** for daemon + CLI hooks. Tauri menubar in Rust (standard split). Communicate via localhost HTTP or IPC.

**Decision:** **Go** for daemon + CLI hooks. Tauri menubar in Rust (standard split).

### ✅ D8 — Binary names

| Binary | Name |
|--------|------|
| CLI | **`rmb`** |
| Daemon | **`rmbd`** (`rmbd serve`) |

**Decision:** CLI = **`rmb`**; daemon = **`rmbd`**.

### ✅ D9 — CLI vs daemon split

| Binary | Role | Commands |
|--------|------|----------|
| **`rmb`** | CLI for agents and operators | `hook-submit`, `search`, `cat`, `tree`, `meta`, `setup`, … |
| **`rmbd`** | Background daemon | `serve` (HTTP API, workers, embedded UI) |

Hooks invoke **`rmb hook-submit`**; daemon runs as **`rmbd`** via launchd/systemd.

**Decision:** Two binaries — **`rmb`** (CLI) + **`rmbd`** (daemon).

### ✅ D10 — HTTP API surface

Daemon exposes localhost API for UI, CLI recall, and hooks.

| Endpoint group | Purpose |
|----------------|---------|
| `POST /api/v1/sessions/:id/upload` | Hook ingestion |
| `GET /api/v1/search` | Hybrid recall |
| `GET /api/v1/browse/*` | Dashboard lists |
| `GET /api/v1/inspect/*` | `cat` / `tree` / `meta` for agents |
| `GET /healthz` | Liveness |
| `GET /ui/*` | Embedded web dashboard |

Auth: none on localhost (bind `127.0.0.1` only). Revisit if remote access is ever added.

**Decision:** Yes, I want it to be local only.

### ✅ D11 — Web UI tech

| Option | Pros | Cons |
|--------|------|------|
| **Next.js static export** | Rich ecosystem; embed in Go `embed.FS` | Build step; heavier |
| **Vite + React** | Lighter; easy static embed | Build from scratch |
| **Server-rendered templates** | No JS build | Poor UX for dashboard |

**Recommendation:** **Vite + React** — lighter than Next for a localhost-only dashboard. Embed static build in daemon binary.

**Decision:** **Vite + React**; ground-up web UI design in `ui/`.

---

## 5. Desktop shell

### ✅ D12 — Menubar tech

| Option | Pros | Cons |
|--------|------|------|
| **Tauri v2** | ~5 MB; native tray | Linux tray inconsistent |
| **Electron** | Familiar | Heavy for tray-only |
| **No menubar v1** | Fastest | Poor UX |

**Recommendation:** **Tauri v2**. Linux fallback: desktop shortcut + `systemctl --user` status.

**Decision:** tauri v1

### ✅ D13 — OS service registration

| Platform | Mechanism | v1 priority |
|----------|-----------|-------------|
| macOS | `launchd` LaunchAgent | **Yes** |
| Linux | `systemd` user unit | **Yes** |
| Windows | Task Scheduler | **Yes** (if hooks supported) |

**Decision:** All three platforms in scope for daemon; menubar tray quality varies by platform.

### ✅ D14 — Default port

| Option | Notes |
|--------|-------|
| `8080` | Common; may conflict |
| `9477` | Random high port |
| Dynamic / config | Written to `~/.rmb-desktop/config.yaml` on first run |

**Recommendation:** Default `9477`; config override. Write chosen port to config so menubar and hooks agree.

**Decision:** 19019

---

## 6. Agent integration

### Capture flow

```
Agent (Cursor / CC / Codex / …)
  → hook script: rmb hook-submit --source=<agent>
  → POST localhost:19019/api/v1/sessions/:id/upload  (rmbd)
  → daemon stores turn → queues L1 worker
```

### ✅ D15 — Hook install strategy

| Option | Notes |
|--------|-------|
| **Setup wizard writes agent config** | Merge into `hooks.json` / `settings.json`; preserve unrelated keys |
| **Manual docs only** | Fast to ship; bad colleague UX |

**Recommendation:** Setup page in web UI + idempotent merge. `rmb setup --agent=cursor` CLI for power users.

**Decision:** Dedicated setup page (always available in web UI).

### ✅ D16 — CC Switch coexistence

| Option | Notes |
|--------|-------|
| **Hook health check** | Menubar detects missing hook → notify + one-click repair |
| **Document ordering** | "Configure CC Switch first, then rmb-desktop setup" |

**Recommendation:** Health check on tray open.

**Decision:** Will not implement at the moment

### ✅ D17 — Supported agents (v1)

| Agent | macOS | Linux | Windows |
|-------|-------|-------|---------|
| Cursor | Yes | Yes | Yes |
| Claude Code | Yes | Yes | Later |
| Codex | Yes | Yes | Later |
| Pi / OpenCode | Optional | Optional | No |

**Decision:** Cursor, Claude Code, and Codex first (macOS + Linux); Windows hooks later.

---

## 7. Distillation (L1 → L2 → L3)

Background workers call an OpenAI-compatible chat API.

### Pipeline

```
upload turn
  → L1 extract (turns → atoms)
  → L2 scene (atoms → scenes + session abstract)
  → L3 memory (cross-session rollup → versioned memories)
  → embed worker (fill vector columns for search)
```

### ✅ D18 — LLM provider (v1)

| Option | Notes |
|--------|-------|
| **BYOK** | User sets API base + key + model in setup wizard |
| **Ollama (local)** | Documented optional backend |
| **Bundled / subsidized** | You pay inference — defer |

**Recommendation:** **BYOK** + Ollama docs. Ingest-only mode when no key configured.

**Decision:** Yes, I want the user to bring their own key and have a configuration page in the webui.

### ✅ D19 — Embedding provider

| Option | Notes |
|--------|-------|
| **Same provider as chat** | Simpler config |
| **Separate embed endpoint** | Flexibility (cheaper embed model; **must be multilingual**) |

**Recommendation:** Separate config keys; default embed model must be **multilingual** (see D3); document supported models in setup wizard.

**Decision:** Separate config key; multilingual model required.

### ✅ D20 — Workers when API key missing

| Option | Notes |
|--------|-------|
| **Ingest only** | Turns stored; pipeline paused; UI shows "add API key" |
| **Block setup** | Require key before finish |

**Recommendation:** Ingest only.

**Decision:** Ingest only

---

## 8. Recall

Hybrid search: multilingual vector cosine + FTS5 lexical, fused with reciprocal rank fusion (RRF).

### Multilingual recall strategy

```text
Query
  ├─ Vector search (multilingual embed model)  →  primary leg (~70% RRF weight)
  └─ FTS (unicode61 + optional trigram)        →  lexical leg (~30% RRF weight)
```

| Leg | Role | Languages |
|-----|------|-----------|
| **Vector** | Semantic similarity, cross-language recall | All — driven by multilingual embed model (D3) |
| **FTS `unicode61`** | Exact token / keyword hits | All scripts safely; no stemming |
| **FTS `trigram`** (optional) | Substring matching | Helps CJK, Thai, code identifiers |

When embed API key is missing, fall back to FTS-only (degraded). When query script is detected as non-Latin or CJK, optionally boost vector weight further.

### ✅ D21 — Search strategy and fallback

| Scenario | Behavior |
|----------|----------|
| **Normal (embed key set)** | Vector-primary RRF: ~70% vector + ~30% FTS (`unicode61` + optional `trigram`) |
| **No embed key** | FTS-only (`unicode61` + `trigram` if indexed) — degraded but usable |
| **Cross-language query** | Vector leg handles it; FTS may miss — acceptable |
| **Model switch** | Background re-embed all rows; search paused or FTS-only during migration |

**Recommendation:** Vector-primary RRF when embed key present; FTS-only fallback otherwise.

**Decision:** Vector-primary RRF (~70/30); FTS-only when no embed key.

### ✅ D22 — Agent recall interface

| Option | Notes |
|--------|-------|
| **CLI** — `rmb search "query"` | Agents shell out |
| **MCP server** | Native agent integration — later |
| **Both** | CLI v1; MCP v2 |

**Recommendation:** CLI v1; MCP in Phase 3.

**Decision:** `rmb search "query"` (CLI v1); MCP later.

---

## 9. Sync and monetization (design now, ship later)

### Free tier

- One device, full pipeline, local SQLite file.

### Paid tier (sync subscription)

Sync logical entities — never the raw DB file:

| Entity | Sync? | Conflict strategy |
|--------|-------|-------------------|
| L3 memories | Yes | Version by URI + `superseded_at` |
| Corrections | Yes | Append-only |
| Skills | Yes | Version by slug + content hash |
| Profile / agent guide | Yes | Merge UI or LWW |
| Sessions / turns | Optional | Large; backup-only vs full |
| L1 atoms / L2 scenes | Re-derive | Prefer workers over syncing derived data |
| Embeddings | Re-derive or sync | Same model → same vectors |

### ⏸ D23 — Sync architecture (future)

| Option | Notes |
|--------|-------|
| **Custom sync service** | E2E encrypted; sync logical rows |
| **CRDT** | Offline merge; heavy |
| **File sync (iCloud/Dropbox)** | Sync `rmb.db` — corruption risk |

**Recommendation:** Custom service over logical entities. Never sync the SQLite file directly.

**Decision:** _Defer_

### ⏸ D24 — Account / billing

Defer until sync MVP. Stripe subscription ~$5–8/mo when ready.

**Decision:** _Defer_

---

## 10. Release and distribution

### ✅ D25 — Packaging (per platform)

| Platform | Format | Signing |
|----------|--------|---------|
| macOS | `.dmg` | Apple Developer ($99/yr) — unsigned alpha OK |
| Linux | `.AppImage` or `.deb` | N/A |
| Windows | `.msi` or `.exe` | Authenticode — later |

**Decision:** lgtm

### ✅ D26 — What ships in the installer

| Component | Bundled? |
|-----------|----------|
| Daemon + CLI binary | Yes |
| Menubar app | Yes |
| Web UI (embedded in daemon) | Yes |
| External runtime (Node, etc.) | No — static embed only |

**Decision:** Single installer per platform. 

---

## 11. Testing strategy

| Area | Approach |
|------|----------|
| SQLite schema | In-memory `:memory:` in unit tests |
| Vector recall | Golden fixtures; ranking order assertions |
| FTS recall | Multilingual eval fixtures (EN, ZH, Thai or Arabic, mixed-script) |
| Hook merge | Table-driven tests per agent format |
| Workers | Mock LLM client; assert pipeline state transitions |
| Menubar | Smoke: start daemon, open dashboard, quit |
| E2E | Install → wizard → hook → capture → session visible in UI |

---

## 12. Phased delivery

### Phase 0 — Decisions + spike

- [x] Resolve D1–D22, D25–D26 (D23–D24 deferred)
- [ ] Spike: SQLite schema + sqlite-vec + FTS5 hybrid recall (unicode61 + trigram)
- [ ] Spike: multilingual embed + vector-primary RRF on sample EN/ZH/TH queries
- [ ] Spike: `rmb hook-submit` for Cursor → `rmbd`

### Phase 1 — Core engine

- [ ] SQLite schema + migrations
- [ ] `rmbd`: upload, health, config
- [ ] L1 / L2 / L3 workers + embed worker
- [ ] Hybrid recall (FTS + vector + RRF)
- [ ] `rmb` CLI: `search`, `cat`, `tree`, `meta`, `hook-submit`, `setup`

### Phase 2 — Web UI

- [ ] Dashboard: sessions, atoms, scenes, memories, pipeline
- [ ] Setup page: API keys, hook status
- [ ] Embedded static build in `rmbd`

### Phase 3 — Desktop shell

- [ ] Tauri menubar: tray, open dashboard, `rmbd` lifecycle
- [ ] Per-OS service install (launchd, systemd, Task Scheduler)
- [ ] First-run wizard

### Phase 4 — Cross-platform polish

- [ ] Linux + Windows installers
- [ ] Windows agent hooks
- [ ] Signed releases
- [ ] Colleague dogfood

### Phase 5 — Sync (paid)

- [ ] Account + auth
- [ ] Entity sync protocol
- [ ] Billing

---

## 13. Risk register

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| sqlite-vec CGO breaks cross-compile | Medium | High | Spike early; brute-force fallback for alpha |
| Multilingual recall quality | Medium | High | Multilingual embed model (D3); vector-primary RRF (D21); multilingual eval set |
| Linux tray missing | High | Low | Browser shortcut + systemd status |
| CC Switch overwrites agent config | Medium | Low | Out of scope v1 (D16); document manual re-run of `rmb setup` |
| Scope creep (Electron, cloud host) | Medium | Medium | Stick to local daemon + web UI |
| Greenfield rewrite takes longer than expected | High | Medium | Phase 1 spike validates core loop first |

---

## 14. Decision log

| ID | Decision | Choice | Date | Notes |
|----|----------|--------|------|-------|
| — | Standalone product | All logic in `rmb-desktop` | 2026-08-02 | No runtime dependency on other projects |
| — | Storage | SQLite + vectors | 2026-08-02 | No external DB server |
| — | Product shape | `rmb` + `rmbd` + web UI + menubar | 2026-08-02 | Syncthing-style |
| D1 | URI scheme | Unified `rmb://` for all tiers | 2026-08-02 | e.g. `rmb://atoms/<uuid>`, `rmb://profile` |
| D2 | Vector extension | sqlite-vec | 2026-08-02 | |
| D3 | Embedding model | Multilingual; configurable dim; default 1024 | 2026-08-02 | BGE-M3 / multilingual-e5 as documented defaults |
| D4 | FTS tokenizer | unicode61 + optional trigram; no Porter | 2026-08-02 | |
| D5 | DB location | Platform app-data dir + config override | 2026-08-02 | |
| D6 | Worker coordination | App-level mutex | 2026-08-02 | |
| D7 | Backend language | Go (`rmb`/`rmbd`) + Rust (Tauri menubar) | 2026-08-02 | |
| D8 | Binary names | `rmb` (CLI), `rmbd` (daemon) | 2026-08-02 | |
| D9 | CLI vs daemon | Two binaries | 2026-08-02 | |
| D10 | API auth | Localhost only (`127.0.0.1`) | 2026-08-02 | |
| D11 | Web UI | Vite + React; ground-up design | 2026-08-02 | |
| D12 | Menubar | Tauri v1 | 2026-08-02 | |
| D13 | OS services | launchd + systemd + Task Scheduler | 2026-08-02 | |
| D14 | Default port | 19019 | 2026-08-02 | |
| D15 | Hook install | Setup page in web UI + `rmb setup` | 2026-08-02 | |
| D16 | CC Switch | Not implemented v1 | 2026-08-02 | |
| D17 | Agents v1 | Cursor, CC, Codex (macOS + Linux) | 2026-08-02 | Windows hooks later |
| D18 | LLM provider | BYOK + config page in web UI | 2026-08-02 | |
| D19 | Embedding provider | Separate config key; multilingual | 2026-08-02 | |
| D20 | No API key | Ingest only | 2026-08-02 | |
| D21 | Search | Vector-primary RRF (~70/30); FTS fallback | 2026-08-02 | |
| D22 | Agent recall | `rmb search` CLI v1 | 2026-08-02 | MCP later |
| D23 | Sync | _Deferred_ | | |
| D24 | Billing | _Deferred_ | | |
| D25 | Packaging | .dmg / .AppImage or .deb / .msi per platform | 2026-08-02 | |
| D26 | Installer contents | Single installer: `rmb` + `rmbd` + menubar + embedded UI | 2026-08-02 | |

---

## 15. Open questions

1. Product name in the menubar: **rmb-desktop**, **rmb**, or a new consumer brand?
2. Fully offline (no network ever) vs localhost + optional cloud LLM API?
3. Team / shared memory namespace, or personal-only sync?
4. Optional import from other memory tools (generic JSON export) — v1 or never?
