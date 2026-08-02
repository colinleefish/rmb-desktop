.PHONY: test build run-rmbd run-hook tidy

GO_TAGS := sqlite_fts5

test:
	CGO_ENABLED=1 go test -tags "$(GO_TAGS)" ./...

build:
	CGO_ENABLED=1 go build -tags "$(GO_TAGS)" -o bin/rmbd ./cmd/rmbd
	CGO_ENABLED=1 go build -tags "$(GO_TAGS)" -o bin/rmb ./cmd/rmb

run-rmbd:
	CGO_ENABLED=1 go run -tags "$(GO_TAGS)" ./cmd/rmbd serve

tidy:
	go mod tidy
