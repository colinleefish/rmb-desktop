# rmb app (Tauri v1)

Desktop app for managing `rmbd`, opening the dashboard, and future native UI.

## Dev

From repo root:

```bash
make build          # build rmbd to bin/
cd app && npm install
RMBD_PATH=../bin/rmbd npm run dev
```

You should see an **RMB** icon in the macOS menu bar (top-right). Click for:

- **🟢 RMB is running** — opens the dashboard (`http://127.0.0.1:19019/ui/`)
- **Quit RMB** — stops the menu bar app and its background service

While the menu bar icon is visible, the local `rmbd` service runs with the app. Quitting stops both.

The app is **menu bar only** (`LSUIElement`) — no Dock icon. Quit only from the tray menu (**Quit RMB**), which stops both the app and `rmbd`.

## Build

```bash
cd app && npm run build
```

Output: `app/src-tauri/target/release/bundle/macos/RMB.app`

On first launch, the app copies bundled `rmb` and `rmbd` sidecars into
`~/.rmb/bin/` (`rmb` and `rmbd-desktop`). Agent setup writes hook commands
against that stable path — no manual CLI install step after drag-and-drop.

Sidecar staging (before `tauri build`):

```bash
make prepare-sidecars   # bin/rmb + bin/rmbd → app/src-tauri/binaries/*-<target-triple>
```

## Environment

| Variable | Purpose |
|----------|---------|
| `RMBD_PATH` | Path to `rmbd` binary (dev default: search `../../../bin/rmbd`) |
| `RMB_ADDR` | Daemon address (default `127.0.0.1:19019`) |

## Login item (optional)

**Recommended:** enable **Launch at login** in the web UI (Settings → General). The app manages the agent itself — it writes `~/Library/LaunchAgents/me.remember.rmb.login.plist` and cleans up the legacy `me.remember.rmb` agent on toggle.

For headless setups (no menu bar app), the repo ships a template at
[`install/macos/com.remember.rmbd.plist`](../install/macos/com.remember.rmbd.plist)
(label `me.remember.rmbd`). Do not run it alongside the menu bar app — the app
disables that agent while it runs. [`install/macos/com.remember.rmb.plist`](../install/macos/com.remember.rmb.plist)
is the template the login feature is based on; prefer the in-app toggle over
installing either plist by hand.
