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

Use **one** approach — not both:

| LaunchAgent | Purpose |
|-------------|---------|
| [`com.remember.rmb.plist`](../install/macos/com.remember.rmb.plist) | **Recommended:** menu bar app at login (starts `rmbd` with the app) |
| [`com.remember.rmbd.plist`](../install/macos/com.remember.rmbd.plist) | Headless `rmbd` only — do not use alongside the menu bar app |

The `rmbd` LaunchAgent uses `KeepAlive` and will restart the daemon if you quit it manually. The menu bar app disables that agent when it runs.

```bash
# Menu bar app at login (recommended)
launchctl bootstrap gui/$(id -u) ~/Library/LaunchAgents/me.remember.rmb.plist

# Stop the headless daemon agent if you no longer need it
launchctl bootout gui/$(id -u) ~/Library/LaunchAgents/me.remember.rmbd.plist
launchctl disable gui/$(id -u)/me.remember.rmbd
```
