package store

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/inovacc/thimble/internal/paths"
)

// OpenShared opens (or creates) the shared ContentStore at {DataDir}/shared/content.db.
// This store is global — not scoped to any project — and is used for cross-session
// knowledge sharing.
func OpenShared() (*ContentStore, error) {
	sharedDir := filepath.Join(paths.PluginDataDir(), "shared")
	if err := os.MkdirAll(sharedDir, 0o755); err != nil {
		return nil, fmt.Errorf("create shared dir: %w", err)
	}

	cs, err := New(filepath.Join(sharedDir, "content.db"))
	if err != nil {
		return nil, fmt.Errorf("open shared store: %w", err)
	}

	// Configure embedding provider if available.
	if ep := NewEmbeddingProvider(); ep != nil {
		cs.SetEmbeddingProvider(ep)
	}

	return cs, nil
}
