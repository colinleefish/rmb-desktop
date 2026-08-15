# Replace the Tauri shell with a pure-Go tray app + self-updater

> Migration plan. Status: **Phase 1–4 complete (merged to main 2026-08-15); Phase 5 (0.2.0 release & verification) next**.
> D1–D4 resolved 2026-08-15: zip · ed25519 now · v0.2.0 · split status/Open Dashboard.

## 0. Context & goal

The desktop app is a **tray-only shell**: it never opens a window. All UI lives in
the webui embedded in `rmbd` (served at `127.0.0.1:19019`). The Tauri/Rust shell
(836 lines across 4 files) does exactly five things:

1. System tray: status item (click → open dashboard) + Quit, health-poll every 5s
2. Spawn / supervise the `rmbd` sidecar; recycle it when version/commit mismatches
3. Bootstrap: copy `rmb` + `rmbd` into `~/.rmb/bin`, refresh on version change
4. Single-instance lock (flock / Windows mutex)
5. Open the dashboard URL in the default browser

Shipping a full webview runtime (Tauri) — or worse, Electron — to render zero
pixels is pure overhead. The Go ecosystem covers 100% of the shell's needs:

- **`fyne.io/systray`** (standalone; NOT the Fyne toolkit — no OpenGL/GLFW)
- stdlib `os/exec`, `net/http`, plus `golang.org/x/sys`

**Goals**

- One language, one test suite; delete `app/` (Tauri, node_modules, 4.4GB `target/`)
- Windows builds without cargo-xwin / NSIS-for-Tauri machinery
- **Self-updating sidecars** with a China-reachable primary feed
  (`releases.re-mem-ber.me` on Cloudflare R2), GitHub as fallback, user mirrors

**Non-goals** (⏸ deferred)

- In-shell windows/webview (dashboard stays in the browser)
- Shell/bundle self-update — the `.app` rarely changes; sidecars carry the features
- Linux desktop support (systray there needs libappindicator; not requested)

## 1. Target architecture

```
cmd/rmb-app                  ← new Go tray shell (replaces src-tauri)
internal/appshell/           ← ported shell logic, unit-testable
  daemon.go                  ← DaemonManager: spawn/recycle/stop rmbd (port of daemon.rs)
  bootstrap.go               ← install/refresh ~/.rmb/bin sidecars (port of bootstrap.rs)
  lock.go                    ← single-instance: flock (unix) / mutex (windows)
  addr.go                    ← RMB_ADDR env → config.yaml addr → default (reuse internal/config)
internal/update/             ← NEW: feed client + verifier + swapper
internal/version/            ← existing; shell now uses it instead of RMB_APP_VERSION env
scripts/build-macos-app.sh   ← hand-rolled .app assembly (Info.plist, icns, sidecars)
scripts/build-dmg.sh         ← hdiutil UDZO + finish-dmg.sh icon step
rmb-website/scripts/publish-release.sh  ← extended: sidecars + sha256 + signed manifest
```

Reuse as-is: `internal/version` (ldflags), `internal/config` (Addr semantics
already match daemon.rs), `internal/platform`, `internal/launchatlogin`,
`internal/httpserver` (dashboard + `/api/v1/version` + health endpoints),
`scripts/build-windows-sidecars.sh` (mingw CGO cross-build, unchanged core),
`scripts/finish-dmg.sh` (DMG icon, default icns path updated).

## 2. Phase 1 — Go shell parity (est. 1 day)

Port the four Rust modules; **behavior-identical**, no new features.

| Rust source | Go destination | Notes |
|---|---|---|
| `main.rs` tray + 5s poller | `cmd/rmb-app/main.go` + `appshell/tray.go` | `systray.Run` on main thread (`runtime.LockOSThread`); poller goroutine calls `systray.SetTitle`. Quit from tray handler; keep "status click → open dashboard" behavior |
| `daemon.rs` DaemonManager | `appshell/daemon.go` | Port exactly: `ensure_running`, `restart_after_update`, `shutdown`, version+commit match via `GET /api/v1/version`, launchd detach (`launchctl bootout` `gui/<uid>` `me.remember.rmbd`), lsof/netstat port-kill, 15s health wait |
| `bootstrap.rs` | `appshell/bootstrap.go` | Same paths: `~/.rmb/bin/{rmb, rmbd-desktop, rmb-app}`; sidecars resolve relative to the running exe (bundle layout: `Contents/MacOS/{rmb-app, rmb, rmbd}`) |
| `instance.rs` | `appshell/lock.go` | Same lock file `~/.rmb/rmb-app.lock` + same Windows mutex name `Global\me.remember.rmb.app` — so a new-version launch still refuses to double-run alongside an old one |

- `main.go` order: acquire lock → bootstrap → `restart_after_update` → tray
- `go.mod`: add `fyne.io/systray`, `golang.org/x/sys`; nothing else
- Unit tests for `addr.go`, version-match predicate, bootstrap refresh decision
  (`needs_refresh` logic), lock (unix)
- Makefile: `app-dev` becomes `go run ./cmd/rmb-app` against `bin/rmbd` — no
  sidecar staging step at all

**Acceptance:** tray shows 🟢 status, daemon spawns, dashboard opens, Quit kills
daemon + launchd, second launch exits silently, 5s poller recovers a killed rmbd.

## 3. Phase 2 — Self-updater, sidecars only (est. 1–1.5 days)

### Feed

- **Primary:** `https://releases.re-mem-ber.me/latest.json` (R2, China-reachable)
- **Fallback:** GitHub API `repos/colinleefish/rmb-desktop/releases/latest` (json proxy of
  the tag → assets; fine via api.github.com even when `objects.githubusercontent.com` is not)
- **User mirrors:** `update.mirrors: [url...]` in `config.yaml` (+ `rmb config`),
  tried before the defaults

### Manifest v2 (published by publish-release.sh)

```jsonc
{
  "product": "rmb-desktop",
  "version": "0.2.0",
  "released_at": "...",
  "signature": "<ed25519 over canonical manifest body, base64>",
  "platforms": {
    "macos":  { "aarch64": { "sidecars": "rmb-desktop_0.2.0_darwin_arm64.tar.gz", "sha256": "..." } },
    "windows":{ "amd64":   { "sidecars": "rmb-desktop_0.2.0_windows_amd64.zip",    "sha256": "..." } }
  }
}
```

One sidecar bundle per platform containing `rmb` + `rmbd` (+ checksums inside).
The shell binary is NOT in the feed (non-goal). ⏸ **Decision D2: ed25519
manifest signing now (pubkey constant in `internal/update`, private key in the
signing keychain next to the Apple credentials) vs. sha256-only for 0.2.0.**
Recommendation: sign now — mirrors then can serve bytes but never tamper.

### Flow

1. Background check: on launch + every 24h + tray item "Check for Updates…"
2. Newer version → menu gains **"🆕 v0.2.0 — Install Update"**
3. Install: download → verify sig+sha256 → `shutdown()` daemon → swap
   `~/.rmb/bin/{rmb, rmbd-desktop}` (temp file + `os.Rename`; on Windows move
   running exe aside to `*.old` first) → `restart_after_update()` → menu reflects
   new version
4. Failure at any step → rollback to previous files, keep old daemon running,
   status line shows the error

Version comparison: semver-ish (`internal/update`), dev builds (commit `dev`)
never auto-update.

## 4. Phase 3 — Packaging & pipeline ✅ (2026-08-15)

Implemented: `scripts/build-macos-app.sh` (hand-rolled .app; executable keeps
the name `RMB Desktop` so existing login items keep working; ad-hoc sign when
no identity given), `scripts/build-dmg.sh`, `scripts/app-template-Info.plist`
(LSUIElement, min 12.0), `icons/app.icns` committed, Windows zip flow
(`build-windows-zip.sh` + `install-windows.ps1`, decision D1), Makefile +
release.sh re-pointed to `dist/`. Verified by local install: orphan adoption,
bootstrap refresh, login-item continuity.

### macOS

- `scripts/build-macos-app.sh` assembles:
  `RMB Desktop.app/Contents/{MacOS/{rmb-app,rmb,rmbd}, Resources/icon.icns, Info.plist}`
- Info.plist: `CFBundleIdentifier me.remember.rmb` (unchanged), `LSUIElement=true`
  (agent app, no Dock), versions stamped from Makefile vars
- Icons without the tauri CLI: `resvg` → PNG set → `iconutil` → `icns` from the
  existing `icons/pyramid-dark-accent.svg`; tray PNG from `pyramid-tray.svg`
  (template, 52px) — replaces the `app-icons` Makefile target
- `scripts/build-dmg.sh`: `hdiutil create -format UDZO` + existing
  `finish-dmg.sh` (update its default icns path)
- Signing/notarization: unchanged identities/targets
  (`SIGN_IDENTITY`, `SIGN_KEYCHAIN`, `notarytool` + `stapler`)

### Windows

- `build-windows-sidecars.sh` keeps producing `bin/{rmb,rmbd}-windows-amd64.exe`
  (mingw CGO — drop only the `src-tauri/binaries` staging)
- Add `GOOS=windows` build of `rmb-app` (pure Go, trivial)
- ⏸ **Decision D1: 0.2.0 Windows distribution = zip** (`RMB-Desktop_0.2.0_x64.zip`
  with the 3 exes + `install.ps1` adding Start-menu shortcut + optional
  HKCU Run key) **vs. one NSIS script**. Recommendation: zip for 0.2.0 — the
  updater makes installer UX matter much less; NSIS only if users complain.

### rmb-website publishing (required every release now)

- Extend `publish-release.sh`: upload sidecar bundles + full `manifest.json`
  (with sha256), refresh `latest.json` + `versions.json`, sign with ed25519
- `make release` drops `PUBLISH_R2` gate → R2 publish becomes mandatory;
  GitHub upload stays as-is
- **Fix current rot:** `latest.json` still points at 0.1.0 — first 0.2.0 release
  must regenerate everything

## 5. Phase 4 — Deletion & docs ✅ (2026-08-15)

- `git rm -r app/` (src-tauri, node_modules, dist, tauri.conf.json)
- Makefile: remove `prepare-sidecars`, `app-icons` (tauri variant), tauri paths in
  `app-build*`; re-point `DMG_BUNDLE`, `app-install` at the new bundle path
- `scripts/prepare-sidecars.sh` deleted; `release.sh` DMG path updated
- Docs: this decision record + README install section; note in
  `plan/local-first-desktop.md`

## 6. Phase 5 — Verification & rollout

- `make test` + `go vet ./...`
- Manual matrix: fresh `~/.rmb` (bootstrap from zero), over-install from 0.1.21
  (old files refreshed, old daemon recycled), kill -9 rmbd recovery, Quit,
  double-launch lock, updater: 0.2.0 → 0.2.1 dry-run against a staging manifest
- Un-notarized local DMG smoke via `scripts/smoke.sh` where applicable
- **⏸ Decision D3: version = 0.2.0** (shell change + updater = minor bump)
- Tag the last Tauri release (`v0.1.21-tauri-final`) as rollback point
- One-time user action: existing 0.1.x users download the 0.2.0 DMG/installer
  manually (old shell has no updater). From 0.2.0 on, updates are in-app.

## 7. Risks

| Risk | Mitigation |
|---|---|
| `systray.Run` must own the main thread; UI calls from goroutines | All logic in goroutines; systray API is goroutine-safe; Quit from handler only (known-good pattern) |
| Replacing a running `rmbd` exe on Windows | Swap happens strictly after `shutdown()`; still move-aside fallback (`*.old`) |
| Mirror/tampering | ed25519 manifest signature verified before any swap; sha256 per bundle |
| Unsigned dev builds confuse the updater | Dev builds never self-update; signature failure = hard abort + rollback |
| R2 reachability regresses | mirror list in config is user-editable; GitHub fallback stays |
| Hand-rolled bundle misses a Tauri nicety (e.g. quarantine attrs, Info.plist key) | Diff `plist` output vs 0.1.21 bundle before release; keep 0.1.21 DMG for A/B |

## 8. Order of execution

Phase 1 → smoke → Phase 3 (macOS packaging only) → cut a local DMG →
Phase 2 (updater) + website publishing → Phase 4 deletion → Phase 5 full matrix
→ release 0.2.0.

Rationale: shell parity and packaging de-risk the bundle swap before the updater
(which needs real artifacts to test against) is built.

---

**Open decisions to confirm before starting**

- **D1 ✅ zip** — `RMB-Desktop_0.2.0_x64.zip` (3 exes + install.ps1); revisit NSIS only on user demand
- **D2 ✅ ed25519 now** — manifest signed, pubkey constant in `internal/update`
- **D3 ✅ 0.2.0**
- **D4 ✅ split** — status item (disabled) + "Open Dashboard" + separator + Quit ("Check for Updates…" added in Phase 2)
