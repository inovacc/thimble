#!/usr/bin/env bash
# Marketplace launcher for thimble.
# Auto-downloads the binary on first use, then proxies all args.
# Used by: /plugin marketplace add inovacc/thimble
set -euo pipefail

REPO="inovacc/thimble"
INSTALL_DIR="${THIMBLE_INSTALL_DIR:-${HOME}/.local/bin}"

detect_os() {
  case "$(uname -s)" in
    Linux*)  echo "Linux" ;;
    Darwin*) echo "Darwin" ;;
    *)       echo "unsupported" ;;
  esac
}

detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64) echo "x86_64" ;;
    aarch64|arm64) echo "arm64" ;;
    *)             echo "unsupported" ;;
  esac
}

main() {
  local bin="${INSTALL_DIR}/thimble"

  # If binary exists, just exec it.
  if [ -x "$bin" ]; then
    exec "$bin" "$@"
  fi

  local os_name arch asset_name version
  os_name="$(detect_os)"
  arch="$(detect_arch)"

  if [ "$os_name" = "unsupported" ] || [ "$arch" = "unsupported" ]; then
    echo "thimble: unsupported platform $(uname -s)/$(uname -m)" >&2
    exit 1
  fi

  asset_name="thimble_${os_name}_${arch}.tar.gz"

  # Resolve latest version.
  if [ -n "${THIMBLE_VERSION:-}" ]; then
    version="${THIMBLE_VERSION#v}"
  else
    version="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"v?([^"]+)".*/\1/')"
  fi

  echo "thimble: downloading v${version} for ${os_name}/${arch}..." >&2

  local tmpdir
  tmpdir="$(mktemp -d)"
  trap 'rm -rf "$tmpdir"' EXIT

  # Download archive + checksums.
  curl -fsSL "https://github.com/${REPO}/releases/download/v${version}/${asset_name}" -o "${tmpdir}/${asset_name}"
  curl -fsSL "https://github.com/${REPO}/releases/download/v${version}/checksums.txt" -o "${tmpdir}/checksums.txt"

  # Verify checksum.
  local expected actual
  expected="$(grep "${asset_name}" "${tmpdir}/checksums.txt" | awk '{print $1}')"
  actual="$(sha256sum "${tmpdir}/${asset_name}" | awk '{print $1}')"

  if [ "$expected" != "$actual" ]; then
    echo "thimble: checksum mismatch (expected ${expected}, got ${actual})" >&2
    exit 1
  fi

  # Extract and install.
  mkdir -p "$INSTALL_DIR"
  tar xzf "${tmpdir}/${asset_name}" -C "${tmpdir}" thimble
  mv "${tmpdir}/thimble" "$bin"
  chmod +x "$bin"

  echo "thimble: installed to ${bin}" >&2

  # Run setup for Claude Code.
  "$bin" setup --client claude --plugin 2>/dev/null || true

  # Now exec with original args.
  exec "$bin" "$@"
}

main "$@"
