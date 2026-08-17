.PHONY: test build build-all run-rmbd run-hook tidy app-dev app-build app-build-windows app-install webui-dev webui-build webui-embed-check icons-sync build-windows-sidecars notarize release release-upload release-publish

GO_TAGS := sqlite_fts5
EMBED_INDEX := internal/http/static/web/index.html
VERSION ?= 0.2.8-dev.3
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
GO_LDFLAGS := -X github.com/colinleefish/rmb-desktop/internal/version.Version=$(VERSION) -X github.com/colinleefish/rmb-desktop/internal/version.Commit=$(COMMIT)

ICON_SRC := icons/pyramid-dark-accent.svg
TRAY_ICON_SRC := icons/pyramid-tray.svg

test: webui-embed-check
	CGO_ENABLED=1 go test -tags "$(GO_TAGS)" ./...

build: webui-embed-check
	CGO_ENABLED=1 go build -tags "$(GO_TAGS)" -ldflags "$(GO_LDFLAGS)" -o bin/rmbd ./cmd/rmbd
	CGO_ENABLED=1 go build -tags "$(GO_TAGS)" -ldflags "$(GO_LDFLAGS)" -o bin/rmb ./cmd/rmb
	CGO_ENABLED=1 go build -tags "$(GO_TAGS)" -ldflags "$(GO_LDFLAGS)" -o bin/rmb-app ./cmd/rmb-app

build-all: webui-build build

webui-embed-check:
	@test -f $(EMBED_INDEX) || (echo "Missing $(EMBED_INDEX). Run: make webui-build  (or make build-all)" >&2; exit 1)

webui-dev:
	cd webui && npm run dev

webui-build: icons-sync
	cd webui && npm run build
	rm -rf internal/http/static/web/assets internal/http/static/web/index.html internal/http/static/web/vite.svg
	cp -R webui/dist/. internal/http/static/web/

icons-sync:
	cp $(ICON_SRC) webui/public/favicon.svg
	cp $(ICON_SRC) webui/public/logo.svg

run-rmbd:
	CGO_ENABLED=1 go run -tags "$(GO_TAGS)" ./cmd/rmbd serve

tidy:
	go mod tidy

app-dev: build
	RMBD_PATH=$(CURDIR)/bin/rmbd ./bin/rmb-app

DMG_BUNDLE := dist/RMB Desktop_$(VERSION)_aarch64.dmg
WINDOWS_ZIP := dist/RMB-Desktop_$(VERSION)_x64.zip
APP_BUNDLE := dist/RMB Desktop.app
PROXY_URL ?= socks5://127.0.0.1:1080
SIGN_IDENTITY ?= Developer ID Application: GUANGHUI LI (N4YPJBRBN4)
SIGN_KEYCHAIN ?= $(HOME)/Library/Keychains/rmb-sign.keychain-db
SIGN_KEYCHAIN_PASS ?=
NOTARY_PROFILE ?= rmb-notary
GH := ALL_PROXY=$(PROXY_URL) HTTPS_PROXY=$(PROXY_URL) HTTP_PROXY=$(PROXY_URL) gh

# Full pipeline: build → notarize → GitHub upload. SIGN_KEYCHAIN_PASS required unless UPLOAD_ONLY=1.
# Optional: PUBLISH_R2=1 SKIP_NOTARIZE=1 UPLOAD_ONLY=1
release:
	@test -n "$(VERSION)" || (echo "usage: make release VERSION=0.1.x [SIGN_KEYCHAIN_PASS=...]" >&2; exit 1)
	UPLOAD_ONLY=$(UPLOAD_ONLY) PUBLISH_R2=$(PUBLISH_R2) SKIP_NOTARIZE=$(SKIP_NOTARIZE) \
		SIGN_KEYCHAIN_PASS="$(SIGN_KEYCHAIN_PASS)" bash scripts/release.sh "$(VERSION)"

# Upload/publish GitHub release only (uses proxy). VERSION=0.1.x NOTES=file.md optional.
release-upload:
	@test -n "$(VERSION)" || (echo "usage: make release-upload VERSION=0.1.x" >&2; exit 1)
	UPLOAD_ONLY=1 bash scripts/release.sh "$(VERSION)"

# Deprecated alias — prefer make release or make release-upload.
release-publish: release-upload

release-upload-windows:
	@test -n "$(VERSION)" || (echo "usage: make release-upload-windows VERSION=0.1.x" >&2; exit 1)
	@test -f "$(WINDOWS_ZIP)" || (echo "missing $(WINDOWS_ZIP); run make app-build-windows" >&2; exit 1)
	$(GH) release upload "v$(VERSION)" "$(WINDOWS_ZIP)" --repo colinleefish/rmb-desktop --clobber

build-windows-sidecars: webui-embed-check
	VERSION=$(VERSION) COMMIT=$(COMMIT) bash scripts/build-windows-sidecars.sh

# sidecar-bundles produces the self-updater payloads + signed manifest.json.
# Requires the mingw cross toolchain for the Windows bundle (skip it for
# mac-only dry runs: it tolerates missing windows sidecars).
sidecar-bundles: build build-windows-sidecars
	bash scripts/build-sidecar-bundles.sh "$(VERSION)" "$(COMMIT)"

app-build-windows: webui-build build-windows-sidecars
	bash scripts/build-windows-zip.sh "$(VERSION)" "$(COMMIT)"

app-build: webui-build build
	@# Unlock dedicated signing keychain (pass via SIGN_KEYCHAIN_PASS=... ; do not commit the password).
	@if [ -z "$(SIGN_KEYCHAIN_PASS)" ]; then echo "warning: SIGN_KEYCHAIN_PASS empty — codesign may prompt for rmb-sign.keychain password" >&2; fi
	-security unlock-keychain -p "$(SIGN_KEYCHAIN_PASS)" "$(SIGN_KEYCHAIN)"
	bash scripts/build-macos-app.sh "$(VERSION)" "$(COMMIT)" "$(SIGN_IDENTITY)"
	bash scripts/build-dmg.sh "$(VERSION)"
notarize:
	xcrun notarytool submit "$(DMG_BUNDLE)" --keychain-profile "$(NOTARY_PROFILE)" --wait
	xcrun stapler staple "$(DMG_BUNDLE)"

app-install: app-build
	osascript -e 'quit app "RMB Desktop"' 2>/dev/null || true; sleep 2
	rm -rf "/Applications/RMB Desktop.app"
	cp -R "$(APP_BUNDLE)" "/Applications/RMB Desktop.app"
	cp "$(APP_BUNDLE)/Contents/MacOS/RMB Desktop" "$(HOME)/.rmb/bin/rmb-app"
	chmod +x "$(HOME)/.rmb/bin/rmb-app"
	cp "$(APP_BUNDLE)/Contents/MacOS/rmbd" "$(HOME)/.rmb/bin/rmbd-desktop"
	chmod +x "$(HOME)/.rmb/bin/rmbd-desktop"
	open "/Applications/RMB Desktop.app"

# icons/app.icns is committed. To regenerate: rasterize $(ICON_SRC)
# (resvg) at 16..1024px, then `iconutil -c icns`.
# Tray icon: internal/appshell/assets/tray-icon.png (from $(TRAY_ICON_SRC)).
