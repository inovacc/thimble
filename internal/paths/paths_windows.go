//go:build windows

package paths

import (
	"os"
	"path/filepath"
)

// dataDir returns the Windows data directory: %LOCALAPPDATA%\Thimble.
func dataDir() string {
	if local := os.Getenv("LOCALAPPDATA"); local != "" {
		return filepath.Join(local, "Thimble")
	}

	home, _ := os.UserHomeDir()

	return filepath.Join(home, ".thimble")
}

// configDir returns the Windows config directory: %APPDATA%\Thimble.
func configDir() string {
	if appdata := os.Getenv("APPDATA"); appdata != "" {
		return filepath.Join(appdata, "Thimble")
	}

	home, _ := os.UserHomeDir()

	return filepath.Join(home, ".config", "thimble")
}
