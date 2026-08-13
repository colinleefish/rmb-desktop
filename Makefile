.PHONY: test build build-all run-rmbd run-hook tidy app-dev app-build app-build-windows app-install app-icons webui-dev webui-build webui-embed-check icons-sync prepare-sidecars build-windows-sidecars notarize

GO_TAGS := sqlite_fts5
EMBED_INDEX := internal/http/static/web/index.html
VERSION ?= 0.1.15
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
GO_LDFLAGS := -X github.com/colinleefish/rmb-desktop/internal/version.Version=$(VERSION) -X github.com/colinleefish/rmb-desktop/internal/version.Commit=$(COMMIT)

ICON_SRC := icons/pyramid-dark-accent.svg
TRAY_ICON_SRC := icons/pyramid-tray.svg

test: webui-embed-check
	CGO_ENABLED=1 go test -tags "$(GO_TAGS)" ./...

build: webui-embed-check
	CGO_ENABLED=1 go build -tags "$(GO_TAGS)" -ldflags "$(GO_LDFLAGS)" -o bin/rmbd ./cmd/rmbd
	CGO_ENABLED=1 go build -tags "$(GO_TAGS)" -ldflags "$(GO_LDFLAGS)" -o bin/rmb ./cmd/rmb

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

prepare-sidecars: build
	bash scripts/prepare-sidecars.sh

app-dev: prepare-sidecars
	cd app && RMBD_PATH=$(CURDIR)/bin/rmbd npm run dev

DMG_BUNDLE := app/src-tauri/target/release/bundle/dmg/RMB Desktop_$(VERSION)_aarch64.dmg
WINDOWS_INSTALLER := app/src-tauri/target/x86_64-pc-windows-msvc/release/bundle/nsis/RMB Desktop_$(VERSION)_x64-setup.exe
APP_BUNDLE := app/src-tauri/target/release/bundle/macos/RMB Desktop.app
PROXY_URL ?= socks5://127.0.0.1:1080
SIGN_IDENTITY ?= Developer ID Application: GUANGHUI LI (N4YPJBRBN4)
SIGN_KEYCHAIN ?= $(HOME)/Library/Keychains/rmb-sign.keychain-db
SIGN_KEYCHAIN_PASS ?=
NOTARY_PROFILE ?= rmb-notary
GH := ALL_PROXY=$(PROXY_URL) HTTPS_PROXY=$(PROXY_URL) HTTP_PROXY=$(PROXY_URL) gh

# Upload/publish GitHub release (uses proxy). VERSION=0.1.x NOTES=file.md optional.
release-publish:
	@test -n "$(VERSION)" || (echo "usage: make release-publish VERSION=0.1.x" >&2; exit 1)
	@test -f "$(DMG_BUNDLE)" || (echo "missing $(DMG_BUNDLE); run make app-build" >&2; exit 1)
	cp "$(DMG_BUNDLE)" "/tmp/RMB.Desktop_$(VERSION)_aarch64.dmg"
	$(GH) release create "v$(VERSION)" "/tmp/RMB.Desktop_$(VERSION)_aarch64.dmg" \
		--repo colinleefish/rmb-desktop \
		--title "RMB Desktop $(VERSION)" \
		$(if $(NOTES),--notes-file $(NOTES),)

release-upload-windows:
	@test -n "$(VERSION)" || (echo "usage: make release-upload-windows VERSION=0.1.x" >&2; exit 1)
	@test -f "$(WINDOWS_INSTALLER)" || (echo "missing $(WINDOWS_INSTALLER); run make app-build-windows" >&2; exit 1)
	cp "$(WINDOWS_INSTALLER)" "/tmp/RMB.Desktop_$(VERSION)_x64-setup.exe"
	$(GH) release upload "v$(VERSION)" "/tmp/RMB.Desktop_$(VERSION)_x64-setup.exe" --repo colinleefish/rmb-desktop --clobber

build-windows-sidecars: webui-embed-check
	VERSION=$(VERSION) COMMIT=$(COMMIT) bash scripts/build-windows-sidecars.sh

app-build-windows: webui-build app-icons build-windows-sidecars
	export PATH="$$HOME/.cargo/bin:/opt/homebrew/opt/llvm/bin:/opt/homebrew/bin:$$PATH"; \
	cd app && npm run tauri build -- --runner cargo-xwin --target x86_64-pc-windows-msvc

app-build: webui-build app-icons prepare-sidecars
	@# Unlock dedicated signing keychain (pass via SIGN_KEYCHAIN_PASS=... ; do not commit the password).
	@if [ -z "$(SIGN_KEYCHAIN_PASS)" ]; then echo "warning: SIGN_KEYCHAIN_PASS empty — codesign may prompt for rmb-sign.keychain password" >&2; fi
	-security unlock-keychain -p "$(SIGN_KEYCHAIN_PASS)" "$(SIGN_KEYCHAIN)"
	cd app && RMB_APP_VERSION="$(VERSION)" RMB_APP_COMMIT="$(COMMIT)" APPLE_SIGNING_IDENTITY="$(SIGN_IDENTITY)" npm run build
	bash scripts/finish-dmg.sh "$(DMG_BUNDLE)"

notarize:
	xcrun notarytool submit "$(DMG_BUNDLE)" --keychain-profile "$(NOTARY_PROFILE)" --wait
	xcrun stapler staple "$(DMG_BUNDLE)"

app-install: app-build
	rm -rf "/Applications/RMB Desktop.app"
	cp -R "$(APP_BUNDLE)" "/Applications/RMB Desktop.app"
	cp "$(APP_BUNDLE)/Contents/MacOS/RMB Desktop" "$(HOME)/.rmb/bin/rmb-app"
	chmod +x "$(HOME)/.rmb/bin/rmb-app"
	cp "$(APP_BUNDLE)/Contents/MacOS/rmbd" "$(HOME)/.rmb/bin/rmbd-desktop"
	chmod +x "$(HOME)/.rmb/bin/rmbd-desktop"
	rm -rf "$(APP_BUNDLE)"
	open "/Applications/RMB Desktop.app"

app-icons:
	cd app && npx @resvg/resvg-js-cli ../$(ICON_SRC) /tmp/rmb-app-icon.png --fit-width 1024 --fit-height 1024
	cd app && npx tauri icon /tmp/rmb-app-icon.png -o src-tauri/icons
	cd app && npx @resvg/resvg-js-cli ../$(TRAY_ICON_SRC) src-tauri/icons/tray-icon.png --fit-width 52 --fit-height 52
