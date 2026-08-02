# rmb-desktop

Desktop companion for [rmb](https://github.com/colinleefish/rmb) — local-first memory for AI coding agents.

Syncthing-style architecture: a background daemon runs `rmb serve` on your machine; this repo provides the menubar app, installers, and first-run setup so colleagues can use rmb without provisioning a server.

## Architecture

```
menubar app (Tauri)  →  opens localhost:8080/ui
        ↓ manages
rmb serve (daemon)   →  capture, distill, recall
        ↓
local storage        →  PostgreSQL + pgvector (bundled or local)
```

## Status

Early bootstrap. See the [rmb](https://github.com/colinleefish/rmb) repo for the core engine.

## Planned layout

```
rmb-desktop/
├── menubar/     # Tauri tray app
├── install/     # per-OS service installers (launchd, systemd, …)
└── scripts/     # download rmb binary, first-run setup
```

## License

TBD — engine is in [rmb](https://github.com/colinleefish/rmb).
