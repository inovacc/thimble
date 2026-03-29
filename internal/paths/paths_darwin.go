//go:build darwin

package paths

import (
	"os"
	"path/filepath"
)

// dataDir returns the macOS data directory: ~/Library/Application Support/thimble.
func dataDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "Application Support", "thimble")
}

// configDir returns the macOS config directory: ~/Library/Application Support/thimble.
func configDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "Application Support", "thimble")
}
