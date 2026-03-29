package paths

import (
	"os"
	"path/filepath"
)

// MigrateProjectsToSessions renames {DataDir}/projects/ entries to
// {DataDir}/sessions/ if the old directory exists and the new one does not.
// This is a one-time migration for the v4 single-binary architecture.
func MigrateProjectsToSessions() {
	dataDir := DataDir()
	oldBase := filepath.Join(dataDir, "projects")
	newBase := filepath.Join(dataDir, "sessions")

	// Nothing to migrate if old dir doesn't exist.
	info, err := os.Stat(oldBase)
	if err != nil || !info.IsDir() {
		return
	}

	// If sessions/ already exists, skip (migration already done or mixed state).
	if _, err := os.Stat(newBase); err == nil {
		return
	}

	// Rename projects/ → sessions/.
	_ = os.Rename(oldBase, newBase)
}
