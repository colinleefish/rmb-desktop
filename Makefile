.PHONY: test build build-all run-rmbd run-hook tidy app-dev app-build app-install app-icons webui-dev webui-build webui-embed-check icons-sync prepare-sidecars

GO_TAGS := sqlite_fts5
EMBED_INDEX := internal/http/static/web/index.html

ICON_SRC := icons/pyramid-dark-accent.svg
TRAY_ICON_SRC := icons/pyramid-tray.svg

test: webui-embed-check
	CGO_ENABLED=1 go test -tags "$(GO_TAGS)" ./...

build: webui-embed-check
	CGO_ENABLED=1 go build -tags "$(GO_TAGS)" -o bin/rmbd ./cmd/rmbd
	CGO_ENABLED=1 go build -tags "$(GO_TAGS)" -o bin/rmb ./cmd/rmb

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

DMG_BUNDLE := app/src-tauri/target/release/bundle/dmg/RMB Desktop_0.1.7_aarch64.dmg
APP_BUNDLE := app/src-tauri/target/release/bundle/macos/RMB Desktop.app

app-build: app-icons prepare-sidecars
	cd app && npm run build
	bash scripts/finish-dmg.sh "$(DMG_BUNDLE)"

app-install: app-build
	rm -rf "/Applications/RMB Desktop.app"
	cp -R "$(APP_BUNDLE)" "/Applications/RMB Desktop.app"
	cp "$(APP_BUNDLE)/Contents/MacOS/rmb" "$(HOME)/.rmb/bin/rmb-app"
	chmod +x "$(HOME)/.rmb/bin/rmb-app"
	cp "$(APP_BUNDLE)/Contents/MacOS/rmbd" "$(HOME)/.rmb/bin/rmbd-desktop"
	chmod +x "$(HOME)/.rmb/bin/rmbd-desktop"
	rm -rf "$(APP_BUNDLE)"
	open "/Applications/RMB Desktop.app"

app-icons:
	cd app && npx @resvg/resvg-js-cli ../$(ICON_SRC) /tmp/rmb-app-icon.png --fit-width 1024 --fit-height 1024
	cd app && npx tauri icon /tmp/rmb-app-icon.png -o src-tauri/icons
	cd app && npx @resvg/resvg-js-cli ../$(TRAY_ICON_SRC) src-tauri/icons/tray-icon.png --fit-width 52 --fit-height 52
