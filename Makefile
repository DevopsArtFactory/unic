APP_NAME := unic
VERSION  := $(shell grep '^version' Cargo.toml | head -1 | sed 's/.*"\(.*\)"/\1/')
DIST_DIR := dist

# Docker settings
DOCKER_IMAGE := unic-builder
DOCKER_TAG   := latest

# Build targets
TARGET_DARWIN_AMD64  := x86_64-apple-darwin
TARGET_DARWIN_ARM64  := aarch64-apple-darwin
TARGET_LINUX_AMD64   := x86_64-unknown-linux-gnu
TARGET_LINUX_ARM64   := aarch64-unknown-linux-gnu
TARGET_WINDOWS_AMD64 := x86_64-pc-windows-gnu

.PHONY: all clean setup build \
        build-darwin build-darwin-amd64 build-darwin-arm64 \
        build-linux build-linux-amd64 build-linux-arm64 \
        build-windows \
        archive \
        docker-builder docker-build docker-build-all

## Default: build for the current platform
all: build

## Build for current platform (debug)
build:
	cargo build

## Build for current platform (release)
release:
	cargo build --release

## Install cross-compilation targets via rustup
setup:
	curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y
	source $$HOME/.cargo/env
	rustup target add $(TARGET_DARWIN_AMD64)
	rustup target add $(TARGET_DARWIN_ARM64)
	rustup target add $(TARGET_LINUX_AMD64)
	rustup target add $(TARGET_LINUX_ARM64)
	rustup target add $(TARGET_WINDOWS_AMD64)
	@echo ""
	@echo "=== Additional dependencies ==="
	@echo "Linux cross-compile (from macOS):"
	@echo "  brew install filosottile/musl-cross/musl-cross  # for musl"
	@echo "  brew install messense/macos-cross-toolchains/x86_64-unknown-linux-gnu"
	@echo "  brew install messense/macos-cross-toolchains/aarch64-unknown-linux-gnu"
	@echo ""
	@echo "Windows cross-compile (from macOS):"
	@echo "  brew install mingw-w64"

## ── Darwin ──────────────────────────────────────────────

build-darwin: build-darwin-amd64 build-darwin-arm64

build-darwin-amd64:
	cargo build --release --target $(TARGET_DARWIN_AMD64)
	@mkdir -p $(DIST_DIR)
	cp target/$(TARGET_DARWIN_AMD64)/release/$(APP_NAME) \
	   $(DIST_DIR)/$(APP_NAME)-$(VERSION)-darwin-amd64

build-darwin-arm64:
	cargo build --release --target $(TARGET_DARWIN_ARM64)
	@mkdir -p $(DIST_DIR)
	cp target/$(TARGET_DARWIN_ARM64)/release/$(APP_NAME) \
	   $(DIST_DIR)/$(APP_NAME)-$(VERSION)-darwin-arm64

## ── Linux ───────────────────────────────────────────────

build-linux: build-linux-amd64 build-linux-arm64

build-linux-amd64:
	CC=x86_64-unknown-linux-gnu-gcc \
	cargo build --release --target $(TARGET_LINUX_AMD64)
	@mkdir -p $(DIST_DIR)
	cp target/$(TARGET_LINUX_AMD64)/release/$(APP_NAME) \
	   $(DIST_DIR)/$(APP_NAME)-$(VERSION)-linux-amd64

build-linux-arm64:
	CC=aarch64-unknown-linux-gnu-gcc \
	cargo build --release --target $(TARGET_LINUX_ARM64)
	@mkdir -p $(DIST_DIR)
	cp target/$(TARGET_LINUX_ARM64)/release/$(APP_NAME) \
	   $(DIST_DIR)/$(APP_NAME)-$(VERSION)-linux-arm64

## ── Windows ─────────────────────────────────────────────

build-windows:
	CC=x86_64-w64-mingw32-gcc \
	cargo build --release --target $(TARGET_WINDOWS_AMD64)
	@mkdir -p $(DIST_DIR)
	cp target/$(TARGET_WINDOWS_AMD64)/release/$(APP_NAME).exe \
	   $(DIST_DIR)/$(APP_NAME)-$(VERSION)-windows-amd64.exe

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
	cargo clean
	rm -rf $(DIST_DIR)

## ── Docker builds ─────────────────────────────────────

## Build the Docker builder image
docker-builder:
	docker build -f Dockerfile.build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .

## Build Linux + Windows via Docker (results → dist/)
docker-build: docker-builder
	@mkdir -p $(DIST_DIR)
	docker run --rm \
		-v $(CURDIR)/$(DIST_DIR):/output \
		$(DOCKER_IMAGE):$(DOCKER_TAG)

## Build all platforms: Docker (Linux + Windows) + native macOS
docker-build-all: docker-build build-darwin
	@echo "All platform builds complete. Check $(DIST_DIR)/"

## ── Help ────────────────────────────────────────────────

help:
	@echo "Usage: make <target>"
	@echo ""
	@echo "Targets:"
	@echo "  build              Build for current platform (debug)"
	@echo "  release            Build for current platform (release)"
	@echo "  setup              Install cross-compilation targets & show deps"
	@echo ""
	@echo "  build-darwin       Build for macOS (amd64 + arm64)"
	@echo "  build-darwin-amd64 Build for macOS Intel"
	@echo "  build-darwin-arm64 Build for macOS Apple Silicon"
	@echo ""
	@echo "  build-linux        Build for Linux (amd64 + arm64)"
	@echo "  build-linux-amd64  Build for Linux x86_64"
	@echo "  build-linux-arm64  Build for Linux aarch64"
	@echo ""
	@echo "  build-windows      Build for Windows x86_64"
	@echo ""
	@echo "  build-all          Build for all platforms"
	@echo "  archive            Build all + create tar.gz/zip archives"
	@echo ""
	@echo "  docker-builder     Build the Docker builder image"
	@echo "  docker-build       Build Linux + Windows via Docker (→ dist/)"
	@echo "  docker-build-all   Docker build + native macOS build"
	@echo ""
	@echo "  clean              Remove build artifacts"
	@echo "  help               Show this help"
