.PHONY: test build build-all run-rmbd run-hook tidy app-dev app-build app-icons webui-dev webui-build icons-sync

GO_TAGS := sqlite_fts5

ICON_SRC := icons/pyramid-dark-accent.svg
TRAY_ICON_SRC := icons/pyramid-tray.svg

test:
	CGO_ENABLED=1 go test -tags "$(GO_TAGS)" ./...

build:
	CGO_ENABLED=1 go build -tags "$(GO_TAGS)" -o bin/rmbd ./cmd/rmbd
	CGO_ENABLED=1 go build -tags "$(GO_TAGS)" -o bin/rmb ./cmd/rmb

build-all: webui-build build

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

app-dev:
	cd app && RMBD_PATH=$(CURDIR)/bin/rmbd npm run dev

app-build: app-icons
	cd app && npm run build

app-icons:
	qlmanage -t -s 1024 -o /tmp $(ICON_SRC)
	cd app && npx tauri icon /tmp/$$(basename $(ICON_SRC)).png -o src-tauri/icons
	cd app && npx @resvg/resvg-js-cli ../$(TRAY_ICON_SRC) src-tauri/icons/tray-icon.png --fit-width 52 --fit-height 52
