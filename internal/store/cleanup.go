package store

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
)

var staleDBRe = regexp.MustCompile(`^thimble-(\d+)\.db$`)

// CleanupStaleDBs removes DB files from previous sessions whose processes
// no longer exist. Returns the number of cleaned up databases.
func CleanupStaleDBs(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}

	pid := os.Getpid()
	cleaned := 0

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		m := staleDBRe.FindStringSubmatch(entry.Name())
		if m == nil {
			continue
		}

		filePID, _ := strconv.Atoi(m[1])
		if filePID == pid {
			continue
		}

		// Check if the process is still alive.
		proc, err := os.FindProcess(filePID)
		if err == nil {
			// On Unix, FindProcess always succeeds. Signal 0 checks existence.
			if err := proc.Signal(os.Signal(nil)); err == nil {
				continue // still alive
			}
		}

		// Process is dead — clean up DB files.
		base := filepath.Join(dir, entry.Name())
		for _, suffix := range []string{"", "-wal", "-shm"} {
			_ = os.Remove(base + suffix)
		}

		cleaned++
	}

	return cleaned
}
