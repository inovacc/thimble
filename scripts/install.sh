#!/usr/bin/env bash
# Thimble installer for Linux and macOS.
# Usage: curl -fsSL https://raw.githubusercontent.com/inovacc/thimble/main/scripts/install.sh | bash
set -euo pipefail

REPO="inovacc/thimble"
BINARY="thimble"
INSTALL_DIR="${THIMBLE_INSTALL_DIR:-/usr/local/bin}"

# Detect OS and architecture.
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "$ARCH" in
  x86_64|amd64) ARCH="x86_64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "Unsupported architecture: $ARCH" >&2; exit 1 ;;
esac

case "$OS" in
  linux)  OS="Linux" ;;
  darwin) OS="Darwin" ;;
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

# Build download URL.
ASSET="${BINARY}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/${TAG}/${ASSET}"

# Download and extract.
TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TMPDIR"' EXIT

echo "Downloading ${URL}..."
curl -fsSL "$URL" -o "${TMPDIR}/${ASSET}"

echo "Extracting..."
tar xzf "${TMPDIR}/${ASSET}" -C "$TMPDIR"

# Install binary.
if [ -w "$INSTALL_DIR" ]; then
  mv "${TMPDIR}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
else
  echo "Installing to ${INSTALL_DIR} (requires sudo)..."
  sudo mv "${TMPDIR}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
fi

chmod +x "${INSTALL_DIR}/${BINARY}"

echo ""
echo "Installed ${BINARY} ${TAG} to ${INSTALL_DIR}/${BINARY}"
echo ""

# Configure npm for GitHub Packages if npm is available.
if command -v npm &>/dev/null; then
  NPMRC="${HOME}/.npmrc"
  if ! grep -q '@inovacc:registry' "$NPMRC" 2>/dev/null; then
    echo "@inovacc:registry=https://npm.pkg.github.com" >> "$NPMRC"
    echo "Configured npm for @inovacc GitHub Packages."
  fi
fi

echo "Next steps:"
echo "  claude plugin install thimble@npm:@inovacc/thimble   # Register as Claude Code plugin"
echo "  thimble setup --client claude                        # Or configure hooks manually"
echo "  thimble doctor                                       # Run diagnostic checks"
echo "  thimble --help                                       # Show all commands"
