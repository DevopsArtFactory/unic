#!/bin/sh
set -e

REPO="DevopsArtFactory/unic"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

# Detect OS
OS="$(uname -s)"
case "$OS" in
  Darwin)  OS="darwin" ;;
  Linux)   OS="linux" ;;
  *)       echo "Unsupported OS: $OS" >&2; exit 1 ;;
esac

# Detect architecture
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64)  ARCH="amd64" ;;
  aarch64) ARCH="arm64" ;;
  arm64)   ARCH="arm64" ;;
  *)       echo "Unsupported architecture: $ARCH" >&2; exit 1 ;;
esac

# Get latest release tag
echo "Fetching latest release..."
TAG=$(curl -sSf "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')
if [ -z "$TAG" ]; then
  echo "Failed to fetch latest release tag" >&2
  exit 1
fi
echo "Latest version: $TAG"

# Download
ARCHIVE="unic-${OS}-${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/${TAG}/${ARCHIVE}"
echo "Downloading ${URL}..."

TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

curl -sSfL "$URL" -o "${TMPDIR}/${ARCHIVE}"

# Extract
tar -xzf "${TMPDIR}/${ARCHIVE}" -C "$TMPDIR"

# Install both the TUI and MCP server.
if [ -w "$INSTALL_DIR" ]; then
  install -m 0755 "${TMPDIR}/unic" "${INSTALL_DIR}/unic"
  install -m 0755 "${TMPDIR}/unic-mcp" "${INSTALL_DIR}/unic-mcp"
else
  echo "Installing to ${INSTALL_DIR} (requires sudo)..."
  sudo install -m 0755 "${TMPDIR}/unic" "${INSTALL_DIR}/unic"
  sudo install -m 0755 "${TMPDIR}/unic-mcp" "${INSTALL_DIR}/unic-mcp"
fi

echo "unic ${TAG} installed to ${INSTALL_DIR}/unic"
echo "unic-mcp ${TAG} installed to ${INSTALL_DIR}/unic-mcp"
echo ""
echo "Run 'unic' to get started."
