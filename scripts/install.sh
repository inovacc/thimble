#!/usr/bin/env bash
# Thimble installer for Linux and macOS.
# Usage: curl -fsSL https://raw.githubusercontent.com/inovacc/thimble/main/scripts/install.sh | bash
set -euo pipefail

REPO="inovacc/thimble"
APP_DIR="${THIMBLE_INSTALL_DIR:-${HOME}/.thimble}"
PLUGIN_DIR="${APP_DIR}/plugins"
BIN_DIR="${THIMBLE_BIN_DIR:-/usr/local/bin}"

# Detect OS and architecture.
OS="$(uname -s)"
ARCH="$(uname -m)"

case "$ARCH" in
  x86_64|amd64) ARCH="x86_64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "Unsupported architecture: $ARCH" >&2; exit 1 ;;
esac

case "$OS" in
  Linux)  OS="Linux" ;;
  Darwin) OS="Darwin" ;;
  *) echo "Unsupported OS: $OS" >&2; exit 1 ;;
esac

# Get latest release tag.
echo "Fetching latest release..."
TAG="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')"
if [ -z "$TAG" ]; then
  echo "Failed to determine latest release." >&2
  exit 1
fi
echo "Latest release: $TAG"

# Download plugin archive (binary + plugin assets).
ASSET="thimble-plugin_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/${TAG}/${ASSET}"

TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TMPDIR"' EXIT

echo "Downloading ${URL}..."
curl -fsSL "$URL" -o "${TMPDIR}/${ASSET}"

echo "Extracting..."
tar xzf "${TMPDIR}/${ASSET}" -C "$TMPDIR"

# Install binary to app dir.
mkdir -p "$APP_DIR"
cp "${TMPDIR}/thimble" "${APP_DIR}/thimble"
chmod +x "${APP_DIR}/thimble"

# Install plugin assets to plugins dir.
mkdir -p "$PLUGIN_DIR"
for item in .claude-plugin .mcp.json hooks skills agents scripts LICENSE; do
  if [ -e "${TMPDIR}/${item}" ]; then
    cp -r "${TMPDIR}/${item}" "${PLUGIN_DIR}/${item}"
  fi
done
# Copy binary into plugins dir too (needed for MCP server).
cp "${APP_DIR}/thimble" "${PLUGIN_DIR}/thimble"

# Symlink binary for CLI access.
if [ -w "$BIN_DIR" ]; then
  ln -sf "${APP_DIR}/thimble" "${BIN_DIR}/thimble"
else
  echo "Linking to ${BIN_DIR} (requires sudo)..."
  sudo ln -sf "${APP_DIR}/thimble" "${BIN_DIR}/thimble"
fi

echo ""
echo "✓ Installed thimble ${TAG}"
echo "  Binary:  ${APP_DIR}/thimble"
echo "  Plugins: ${PLUGIN_DIR}"
echo ""
echo "Run 'thimble setup' to configure your AI coding assistant."
