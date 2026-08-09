# rmb app (Tauri v1)

Desktop app for managing `rmbd`, opening the dashboard, and future native UI.

## Dev

From repo root:

```bash
make build          # build rmbd to bin/
cd app && npm install
RMBD_PATH=../bin/rmbd npm run dev
```

You should see an **rmb** icon in the macOS menu bar (top-right). Right-click for:

- **Open Dashboard** → `http://127.0.0.1:19019/ui/`
- **Start rmbd** / **Stop rmbd**
- **Quit**

The app uses `ActivationPolicy::Accessory` — no Dock icon (for now).

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

| LaunchAgent | Purpose |
|-------------|---------|
| [`com.remember.rmbd.plist`](../install/macos/com.remember.rmbd.plist) | Start `rmbd` at login |
| [`com.remember.rmb.plist`](../install/macos/com.remember.rmb.plist) | Desktop app at login |

Copy binaries to `~/.rmb/bin/` (`rmbd-desktop`, `rmb-app`), edit paths in the plists, then:

```bash
launchctl bootstrap gui/$(id -u) ~/Library/LaunchAgents/me.remember.rmbd.plist
launchctl bootstrap gui/$(id -u) ~/Library/LaunchAgents/me.remember.rmb.plist
```
