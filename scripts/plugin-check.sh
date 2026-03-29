#!/usr/bin/env bash
# Plugin binary verification hook for SessionStart.
# Checks that the thimble binary exists in the plugin root.
# If missing, prints installation instructions to stderr and exits 1.
set -euo pipefail

BINARY="${CLAUDE_PLUGIN_ROOT:-$(dirname "$0")/..}/thimble"

# On Windows, bash resolves thimble to thimble.exe automatically.
if [ -x "$BINARY" ] || [ -x "${BINARY}.exe" ]; then
  exit 0
fi

echo "thimble binary not found at ${BINARY}" >&2
echo "Install options:" >&2
echo "  1. Download from GitHub Releases: https://github.com/inovacc/thimble/releases" >&2
echo "  2. Build from source: go build -o \"${BINARY}\" ./cmd/thimble" >&2
echo "  3. Homebrew: brew install inovacc/tap/thimble" >&2
exit 1
