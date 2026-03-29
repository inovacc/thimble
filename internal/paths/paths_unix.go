//go:build !windows && !darwin

package paths

import (
	"os"
	"path/filepath"
)

// dataDir returns the Unix data directory: ~/.thimble.
func dataDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".thimble")
}

// configDir returns the Unix config directory: ~/.config/thimble.
func configDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "thimble")
}
