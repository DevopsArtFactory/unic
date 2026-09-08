APP_NAME := unic
MCP_APP_NAME := unic-mcp
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null)
VERSION  := $(if $(VERSION),$(patsubst v%,%,$(VERSION)),dev)
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
	go build -ldflags="-X unic/internal/cli.Version=$(VERSION)" -o $(MCP_APP_NAME) ./cmd/unic-mcp

## Build for current platform (release, stripped)
release:
	go build -ldflags="-s -w -X unic/internal/cli.Version=$(VERSION)" -o $(APP_NAME) $(CMD_PATH)
	go build -ldflags="-s -w -X unic/internal/cli.Version=$(VERSION)" -o $(MCP_APP_NAME) ./cmd/unic-mcp

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
	GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w -X unic/internal/cli.Version=$(VERSION)" -o $(DIST_DIR)/$(MCP_APP_NAME)-darwin-amd64 ./cmd/unic-mcp

build-darwin-arm64:
	GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w -X unic/internal/cli.Version=$(VERSION)" -o $(DIST_DIR)/$(APP_NAME)-darwin-arm64 $(CMD_PATH)
	GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w -X unic/internal/cli.Version=$(VERSION)" -o $(DIST_DIR)/$(MCP_APP_NAME)-darwin-arm64 ./cmd/unic-mcp

## ── Linux ───────────────────────────────────────────────

build-linux: build-linux-amd64 build-linux-arm64

build-linux-amd64:
	GOOS=linux GOARCH=amd64 go build -ldflags="-s -w -X unic/internal/cli.Version=$(VERSION)" -o $(DIST_DIR)/$(APP_NAME)-linux-amd64 $(CMD_PATH)
	GOOS=linux GOARCH=amd64 go build -ldflags="-s -w -X unic/internal/cli.Version=$(VERSION)" -o $(DIST_DIR)/$(MCP_APP_NAME)-linux-amd64 ./cmd/unic-mcp

build-linux-arm64:
	GOOS=linux GOARCH=arm64 go build -ldflags="-s -w -X unic/internal/cli.Version=$(VERSION)" -o $(DIST_DIR)/$(APP_NAME)-linux-arm64 $(CMD_PATH)
	GOOS=linux GOARCH=arm64 go build -ldflags="-s -w -X unic/internal/cli.Version=$(VERSION)" -o $(DIST_DIR)/$(MCP_APP_NAME)-linux-arm64 ./cmd/unic-mcp

## ── Windows ─────────────────────────────────────────────

build-windows:
	GOOS=windows GOARCH=amd64 go build -ldflags="-s -w -X unic/internal/cli.Version=$(VERSION)" -o $(DIST_DIR)/$(APP_NAME)-windows-amd64.exe $(CMD_PATH)
	GOOS=windows GOARCH=amd64 go build -ldflags="-s -w -X unic/internal/cli.Version=$(VERSION)" -o $(DIST_DIR)/$(MCP_APP_NAME)-windows-amd64.exe ./cmd/unic-mcp

## ── All platforms ───────────────────────────────────────

build-all: build-darwin build-linux build-windows

## ── Archive (tar.gz / zip) ──────────────────────────────

archive: build-all
	@set -e; cd "$(DIST_DIR)"; \
	archive_dir=$$(mktemp -d); \
	trap 'rm -rf "$$archive_dir"' EXIT; \
	for platform in darwin-amd64 darwin-arm64 linux-amd64 linux-arm64; do \
		cp "$(APP_NAME)-$$platform" "$$archive_dir/$(APP_NAME)"; \
		cp "$(MCP_APP_NAME)-$$platform" "$$archive_dir/$(MCP_APP_NAME)"; \
		COPYFILE_DISABLE=1 tar czf "$(APP_NAME)-$$platform.tar.gz" -C "$$archive_dir" "$(APP_NAME)" "$(MCP_APP_NAME)"; \
		echo "Created $(APP_NAME)-$$platform.tar.gz"; \
	done; \
	cp "$(APP_NAME)-windows-amd64.exe" "$$archive_dir/$(APP_NAME).exe"; \
	cp "$(MCP_APP_NAME)-windows-amd64.exe" "$$archive_dir/$(MCP_APP_NAME).exe"; \
	rm -f "$(APP_NAME)-windows-amd64.zip"; \
	zip -j "$(APP_NAME)-windows-amd64.zip" "$$archive_dir/$(APP_NAME).exe" "$$archive_dir/$(MCP_APP_NAME).exe"; \
	echo "Created $(APP_NAME)-windows-amd64.zip"

## ── Clean ───────────────────────────────────────────────

clean:
	rm -f $(APP_NAME) $(MCP_APP_NAME)
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
