APP_NAME := unic
VERSION  := 0.1.3
DIST_DIR := dist
CMD_PATH := ./cmd/unic

.PHONY: all build release test clean \
        build-darwin build-darwin-amd64 build-darwin-arm64 \
        build-linux build-linux-amd64 build-linux-arm64 \
        build-windows build-all archive help

## Default: build for the current platform
all: build

## Build for current platform (debug)
build:
	go build -ldflags="-X unic/internal/cli.Version=$(VERSION)" -o $(APP_NAME) $(CMD_PATH)

## Build for current platform (release, stripped)
release:
	go build -ldflags="-s -w -X unic/internal/cli.Version=$(VERSION)" -o $(APP_NAME) $(CMD_PATH)

## Run tests
test:
	go test ./...

## Run tests with verbose output
test-v:
	go test -v ./...

## ── Darwin ──────────────────────────────────────────────

build-darwin: build-darwin-amd64 build-darwin-arm64

build-darwin-amd64:
	GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w -X unic/internal/cli.Version=$(VERSION)" -o $(DIST_DIR)/$(APP_NAME)-darwin-amd64 $(CMD_PATH)

build-darwin-arm64:
	GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w -X unic/internal/cli.Version=$(VERSION)" -o $(DIST_DIR)/$(APP_NAME)-darwin-arm64 $(CMD_PATH)

## ── Linux ───────────────────────────────────────────────

build-linux: build-linux-amd64 build-linux-arm64

build-linux-amd64:
	GOOS=linux GOARCH=amd64 go build -ldflags="-s -w -X unic/internal/cli.Version=$(VERSION)" -o $(DIST_DIR)/$(APP_NAME)-linux-amd64 $(CMD_PATH)

build-linux-arm64:
	GOOS=linux GOARCH=arm64 go build -ldflags="-s -w -X unic/internal/cli.Version=$(VERSION)" -o $(DIST_DIR)/$(APP_NAME)-linux-arm64 $(CMD_PATH)

## ── Windows ─────────────────────────────────────────────

build-windows:
	GOOS=windows GOARCH=amd64 go build -ldflags="-s -w -X unic/internal/cli.Version=$(VERSION)" -o $(DIST_DIR)/$(APP_NAME)-windows-amd64.exe $(CMD_PATH)

## ── All platforms ───────────────────────────────────────

build-all: build-darwin build-linux build-windows

## ── Archive (tar.gz / zip) ──────────────────────────────

archive: build-all
	@cd $(DIST_DIR) && \
	for f in *-darwin-* *-linux-*; do \
		[ -f "$$f" ] && tar czf "$$f.tar.gz" "$$f" && echo "Created $$f.tar.gz"; \
	done; \
	for f in *.exe; do \
		[ -f "$$f" ] && zip "$$f.zip" "$$f" && echo "Created $$f.zip"; \
	done

## ── Clean ───────────────────────────────────────────────

clean:
	rm -f $(APP_NAME)
	rm -rf $(DIST_DIR)
	go clean

## ── Help ────────────────────────────────────────────────

help:
	@echo "Usage: make <target>"
	@echo ""
	@echo "Targets:"
	@echo "  build              Build for current platform"
	@echo "  release            Build for current platform (stripped)"
	@echo "  test               Run tests"
	@echo "  test-v             Run tests (verbose)"
	@echo ""
	@echo "  build-darwin       Build for macOS (amd64 + arm64)"
	@echo "  build-linux        Build for Linux (amd64 + arm64)"
	@echo "  build-windows      Build for Windows x86_64"
	@echo "  build-all          Build for all platforms"
	@echo "  archive            Build all + create tar.gz/zip archives"
	@echo ""
	@echo "  clean              Remove build artifacts"
	@echo "  help               Show this help"
