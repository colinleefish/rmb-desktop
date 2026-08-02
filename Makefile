.PHONY: test build build-all run-rmbd run-hook tidy menubar-dev menubar-build ui-dev ui-build

GO_TAGS := sqlite_fts5

test:
	CGO_ENABLED=1 go test -tags "$(GO_TAGS)" ./...

build:
	CGO_ENABLED=1 go build -tags "$(GO_TAGS)" -o bin/rmbd ./cmd/rmbd
	CGO_ENABLED=1 go build -tags "$(GO_TAGS)" -o bin/rmb ./cmd/rmb

build-all: ui-build build

ui-dev:
	cd ui && npm run dev

ui-build:
	cd ui && npm run build
	rm -rf internal/http/static/web/assets internal/http/static/web/index.html internal/http/static/web/vite.svg
	cp -R ui/dist/. internal/http/static/web/

run-rmbd:
	CGO_ENABLED=1 go run -tags "$(GO_TAGS)" ./cmd/rmbd serve

tidy:
	go mod tidy

menubar-dev:
	cd menubar && RMBD_PATH=$(CURDIR)/bin/rmbd npm run dev

menubar-build:
	cd menubar && npm run build
