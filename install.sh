#!/bin/sh
# Copyright (c) 2026 dyammarcano. All rights reserved.
#
# Redistribution and use in source and binary forms, with or without
# modification, are permitted provided that the following conditions are met:
#
# 1. Redistributions of source code must retain the above copyright notice,
#    this list of conditions and the following disclaimer.
#
# 2. Redistributions in binary form must reproduce the above copyright notice,
#    this list of conditions and the following disclaimer in the documentation
#    and/or other materials provided with the distribution.
#
# 3. Neither the name of the copyright holder nor the names of its
#    contributors may be used to endorse or promote products derived from
#    this software without specific prior written permission.
#
# THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
# AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
# IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE
# ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE
# LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR
# CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF
# SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS
# INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN
# CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE)
# ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE
# POSSIBILITY OF SUCH DAMAGE.
#
# Install script for thimble (Linux/macOS)
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/inovacc/thimble/main/install.sh | bash
#
# Environment variables:
#   THIMBLE_VERSION      - Install a specific version (e.g., "v1.0.0"). Default: latest.
#   THIMBLE_INSTALL_DIR  - Override install directory. Default: /usr/local/bin or ~/.local/bin.

set -e

REPO="inovacc/thimble"
GITHUB_API="https://api.github.com"

log() {
    printf '%s\n' "$1"
}

err() {
    printf 'Error: %s\n' "$1" >&2
    exit 1
}

# Detect OS
detect_os() {
    os="$(uname -s)"
    case "$os" in
        Linux)  echo "Linux" ;;
        Darwin) echo "Darwin" ;;
        *)      err "Unsupported operating system: $os" ;;
    esac
}

# Detect architecture
detect_arch() {
    arch="$(uname -m)"
    case "$arch" in
        x86_64|amd64)   echo "x86_64" ;;
        aarch64|arm64)  echo "arm64" ;;
        *)              err "Unsupported architecture: $arch" ;;
    esac
}

# Determine install directory
get_install_dir() {
    if [ -n "$THIMBLE_INSTALL_DIR" ]; then
        echo "$THIMBLE_INSTALL_DIR"
    elif [ "$(id -u)" -eq 0 ]; then
        echo "/usr/local/bin"
    else
        echo "$HOME/.local/bin"
    fi
}

# Resolve version tag (latest or user-specified)
resolve_version() {
    if [ -n "$THIMBLE_VERSION" ]; then
        # Ensure it starts with 'v'
        case "$THIMBLE_VERSION" in
            v*) echo "$THIMBLE_VERSION" ;;
            *)  echo "v$THIMBLE_VERSION" ;;
        esac
        return
    fi

    log "Fetching latest release version..."
    tag=$(curl -fsSL "$GITHUB_API/repos/$REPO/releases/latest" | \
          grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"//;s/".*//')

    if [ -z "$tag" ]; then
        err "Failed to determine latest version. Set THIMBLE_VERSION to install a specific version."
    fi

    echo "$tag"
}

# Find asset download URL from release
get_asset_url() {
    version="$1"
    filename="$2"

    url=$(curl -fsSL "$GITHUB_API/repos/$REPO/releases/tags/$version" | \
          grep "browser_download_url" | grep "$filename" | head -1 | \
          sed 's/.*"browser_download_url": *"//;s/".*//')

    if [ -z "$url" ]; then
        err "Could not find asset '$filename' in release $version"
    fi

    echo "$url"
}

# Download a file
download() {
    url="$1"
    dest="$2"
    log "Downloading $(basename "$dest")..."
    curl -fsSL -o "$dest" "$url"
}

# Verify SHA256 checksum
verify_checksum() {
    archive="$1"
    checksums="$2"
    filename="$(basename "$archive")"

    expected=$(grep "$filename" "$checksums" | awk '{print $1}')
    if [ -z "$expected" ]; then
        err "Checksum not found for $filename in checksums.txt"
    fi

    if command -v sha256sum >/dev/null 2>&1; then
        actual=$(sha256sum "$archive" | awk '{print $1}')
    elif command -v shasum >/dev/null 2>&1; then
        actual=$(shasum -a 256 "$archive" | awk '{print $1}')
    else
        err "No sha256sum or shasum found. Cannot verify checksum."
    fi

    if [ "$actual" != "$expected" ]; then
        err "Checksum mismatch for $filename\n  expected: $expected\n  actual:   $actual"
    fi

    log "Checksum verified."
}

main() {
    os=$(detect_os)
    arch=$(detect_arch)
    install_dir=$(get_install_dir)
    version=$(resolve_version)

    archive_name="thimble_${os}_${arch}.tar.gz"
    checksums_name="checksums.txt"

    log "Installing thimble $version ($os/$arch)..."

    # Create temp directory
    tmpdir=$(mktemp -d)
    trap 'rm -rf "$tmpdir"' EXIT

    # Get asset URLs and download
    archive_url=$(get_asset_url "$version" "$archive_name")
    checksums_url=$(get_asset_url "$version" "$checksums_name")

    download "$archive_url" "$tmpdir/$archive_name"
    download "$checksums_url" "$tmpdir/$checksums_name"

    # Verify checksum
    verify_checksum "$tmpdir/$archive_name" "$tmpdir/$checksums_name"

    # Extract binary
    log "Extracting thimble binary..."
    tar -xzf "$tmpdir/$archive_name" -C "$tmpdir" thimble

    if [ ! -f "$tmpdir/thimble" ]; then
        err "Binary 'thimble' not found in archive"
    fi

    # Install
    mkdir -p "$install_dir"
    mv "$tmpdir/thimble" "$install_dir/thimble"
    chmod +x "$install_dir/thimble"

    # Check if install_dir is in PATH
    case ":$PATH:" in
        *":$install_dir:"*) ;;
        *)
            log ""
            log "NOTE: $install_dir is not in your PATH."
            log "Add it by running:"
            log "  export PATH=\"$install_dir:\$PATH\""
            log "Or add that line to your shell profile (~/.bashrc, ~/.zshrc, etc.)."
            ;;
    esac

    log ""
    log "thimble $version installed successfully to $install_dir/thimble"
}

main
