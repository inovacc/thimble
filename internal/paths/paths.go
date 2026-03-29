// Package paths provides platform-aware default directories for thimble.
// Platform-specific implementations are in paths_windows.go and paths_unix.go.
package paths

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
)

// DataDir returns the default data directory for thimble databases and state.
//
//   - Windows: %LOCALAPPDATA%\Thimble
//   - macOS/Linux: ~/.thimble
func DataDir() string {
	return dataDir()
}

// ConfigDir returns the directory for thimble configuration (server.json, etc.).
//
//   - Windows: %APPDATA%\Thimble
//   - macOS/Linux: ~/.config/thimble
func ConfigDir() string {
	return configDir()
}

// PluginDataDir returns the base data directory, preferring the plugin
// persistent data dir when running inside a Claude Code plugin.
// Falls back to the platform-specific DataDir.
func PluginDataDir() string {
	if d := os.Getenv("THIMBLE_PLUGIN_DATA"); d != "" {
		return d
	}

	return DataDir()
}

// ProjectDataDir returns the per-project data directory under the global data dir.
// The project path is hashed to a 16-char hex digest so all databases live in
// one central location (e.g. %LOCALAPPDATA%\Thimble\projects\<digest>).
// When running as a Claude Code plugin, uses the plugin persistent data directory.
func ProjectDataDir(projectDir string) string {
	h := sha256.Sum256([]byte(projectDir))
	digest := hex.EncodeToString(h[:8])

	return filepath.Join(PluginDataDir(), "sessions", digest)
}

// IsGitWorktree detects whether dir is inside a git worktree (as opposed to the
// main repository). In a worktree, .git is a file containing "gitdir: <path>"
// rather than a directory.
func IsGitWorktree(dir string) bool {
	gitPath := filepath.Join(dir, ".git")

	info, err := os.Stat(gitPath)
	if err != nil {
		return false
	}
	// In a worktree, .git is a regular file, not a directory.
	return !info.IsDir()
}

// WorktreeSessionSuffix returns a short hash suffix suitable for per-worktree
// session DB naming. Returns empty string if the directory is not a worktree.
func WorktreeSessionSuffix(dir string) string {
	if !IsGitWorktree(dir) {
		return ""
	}

	h := sha256.Sum256([]byte(dir))

	return hex.EncodeToString(h[:4])
}
