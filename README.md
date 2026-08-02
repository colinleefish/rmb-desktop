# rmb-desktop

Local-first, cross-platform memory for AI coding agents.

Everything runs on your machine: capture, distillation, storage, and recall. No server to deploy. Data lives in a single SQLite database with vector search.

Syncthing-style product shape: background daemon + web GUI + menubar tray app.

## Architecture

```
menubar (Tauri)   →  tray · setup wizard · open dashboard
       ↓ manages
rmbd (daemon)     →  HTTP API · workers · embedded web UI
       ↓
SQLite + vectors  →  platform app-data dir (see plan)

rmb (CLI)         →  hook-submit · search · setup · cat · tree · meta
```

Stable IDs use a unified **`rmb://`** scheme (e.g. `rmb://profile`, `rmb://atoms/<uuid>`).

## Status

Early bootstrap. See [`plan/`](./plan/) — [product plan](./plan/local-first-desktop.md) and [implementation plan](./plan/implementation.md).

## Planned layout

```
rmb-desktop/
├── cmd/rmb/       # CLI entrypoint
├── cmd/rmbd/      # daemon entrypoint
├── internal/      # storage, workers, recall, hooks, API
├── ui/            # Vite + React dashboard (embedded in rmbd)
├── menubar/       # Tauri tray app
└── install/       # per-OS service installers
```

## Principles

- **Standalone** — all memory, distillation, and storage logic lives in this repo.
- **Local-first** — works offline; optional paid sync across devices (later).
- **SQLite + vectors** — single-file database, no external DB server.
- **Cross-platform** — macOS, Linux, Windows.

## License

TBD
