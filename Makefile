# IRon Makefile
# Cross-platform builds: macOS (arm64/amd64), Linux (amd64/arm64), Windows (amd64)

APP     := iron
CMD     := ./cmd/iron
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X main.version=$(VERSION) -s -w"

.PHONY: all build build-all run test lint clean proto install docker build-ui

## ── Web UI ──────────────────────────────────────────────────────────────────
build-ui:
	cd ui && npm install && npm run build

## ── Local build ─────────────────────────────────────────────────────────────
build: build-ui
	go build $(LDFLAGS) -o bin/$(APP) $(CMD)

run: build
	./bin/$(APP)

install: build
	cp bin/$(APP) $(GOPATH)/bin/$(APP)
	@echo "✅ Installed to $(GOPATH)/bin/$(APP)"

## ── Cross-platform builds ───────────────────────────────────────────────────
build-all: build-ui \
	dist/$(APP)-darwin-arm64 \
	dist/$(APP)-darwin-amd64 \
	dist/$(APP)-linux-amd64 \
	dist/$(APP)-linux-arm64 \
	dist/$(APP)-windows-amd64.exe

dist/$(APP)-darwin-arm64:
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $@ $(CMD)

dist/$(APP)-darwin-amd64:
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $@ $(CMD)

dist/$(APP)-linux-amd64:
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $@ $(CMD)

dist/$(APP)-linux-arm64:
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $@ $(CMD)

dist/$(APP)-windows-amd64.exe:
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $@ $(CMD)

## ── Quality ──────────────────────────────────────────────────────────────────
test:
	go test ./... -v -race -cover

lint:
	golangci-lint run ./...

## ── Cleanup ──────────────────────────────────────────────────────────────────
clean:
	rm -rf bin/ dist/
