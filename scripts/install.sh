#!/usr/bin/env bash
# Thimble installer for Linux and macOS.
# Usage: curl -fsSL https://raw.githubusercontent.com/inovacc/thimble/main/scripts/install.sh | bash
set -euo pipefail

REPO="inovacc/thimble"
PLUGIN_DIR="${THIMBLE_PLUGIN_DIR:-${HOME}/.thimble/plugin}"
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

# Download plugin archive (binary + .claude-plugin/ + skills/ + hooks/ + agents/ + .mcp.json).
ASSET="thimble-plugin_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/${TAG}/${ASSET}"

TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TMPDIR"' EXIT

echo "Downloading ${URL}..."
curl -fsSL "$URL" -o "${TMPDIR}/${ASSET}"

echo "Extracting to ${PLUGIN_DIR}..."
mkdir -p "$PLUGIN_DIR"
tar xzf "${TMPDIR}/${ASSET}" -C "$PLUGIN_DIR"

# Symlink binary for CLI access.
if [ -w "$BIN_DIR" ]; then
  ln -sf "${PLUGIN_DIR}/thimble" "${BIN_DIR}/thimble"
else
  echo "Linking to ${BIN_DIR} (requires sudo)..."
  sudo ln -sf "${PLUGIN_DIR}/thimble" "${BIN_DIR}/thimble"
fi

echo ""
echo "✓ Installed thimble ${TAG} to ${PLUGIN_DIR}"
echo ""
echo "Activate in Claude Code:"
echo ""
echo "  claude --plugin-dir ${PLUGIN_DIR}"
echo ""
echo "Other commands:"
echo "  thimble doctor    # Run diagnostic checks"
echo "  thimble --help    # Show all commands"
