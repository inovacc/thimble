package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenShared(t *testing.T) {
	// Use a temp dir as the plugin data dir so we don't touch real data.
	tmpDir := t.TempDir()
	t.Setenv("THIMBLE_PLUGIN_DATA", tmpDir)

	cs, err := OpenShared()
	if err != nil {
		t.Fatalf("OpenShared() error: %v", err)
	}

	defer cs.Close()

	// Verify the DB was created at the expected path.
	expectedPath := filepath.Join(tmpDir, "shared", "content.db")
	if cs.DBPath() != expectedPath {
		t.Errorf("DBPath() = %q, want %q", cs.DBPath(), expectedPath)
	}

	// Verify the file actually exists on disk.
	if _, err := os.Stat(expectedPath); err != nil {
		t.Errorf("shared content.db not found on disk: %v", err)
	}
}

func TestOpenSharedIndexAndSearch(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("THIMBLE_PLUGIN_DATA", tmpDir)

	cs, err := OpenShared()
	if err != nil {
		t.Fatalf("OpenShared() error: %v", err)
	}

	defer cs.Close()

	// Index some content.
	res, err := cs.Index("# Shared Knowledge\n\nThis is cross-project documentation about deployment patterns.", "shared:deployment")
	if err != nil {
		t.Fatalf("Index() error: %v", err)
	}

	if res.TotalChunks == 0 {
		t.Error("expected at least 1 chunk")
	}

	if res.Label != "shared:deployment" {
		t.Errorf("Label = %q, want %q", res.Label, "shared:deployment")
	}

	// Search for the content.
	results, err := cs.SearchWithFallback("deployment patterns", 5, "")
	if err != nil {
		t.Fatalf("SearchWithFallback() error: %v", err)
	}

	if len(results) == 0 {
		t.Error("expected search results, got none")
	}
}

func TestOpenSharedListSources(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("THIMBLE_PLUGIN_DATA", tmpDir)

	cs, err := OpenShared()
	if err != nil {
		t.Fatalf("OpenShared() error: %v", err)
	}

	defer cs.Close()

	// Initially empty.
	sources, err := cs.ListSources()
	if err != nil {
		t.Fatalf("ListSources() error: %v", err)
	}

	if len(sources) != 0 {
		t.Errorf("expected 0 sources, got %d", len(sources))
	}

	// Index content, then verify list.
	_, err = cs.Index("# Test Content\n\nSome shared knowledge.", "shared:test-source")
	if err != nil {
		t.Fatalf("Index() error: %v", err)
	}

	sources, err = cs.ListSources()
	if err != nil {
		t.Fatalf("ListSources() error: %v", err)
	}

	if len(sources) != 1 {
		t.Fatalf("expected 1 source, got %d", len(sources))
	}

	if sources[0].Label != "shared:test-source" {
		t.Errorf("Label = %q, want %q", sources[0].Label, "shared:test-source")
	}
}

func TestOpenSharedIdempotent(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("THIMBLE_PLUGIN_DATA", tmpDir)

	// Open twice — should work fine (same DB).
	cs1, err := OpenShared()
	if err != nil {
		t.Fatalf("first OpenShared() error: %v", err)
	}

	cs1.Close()

	cs2, err := OpenShared()
	if err != nil {
		t.Fatalf("second OpenShared() error: %v", err)
	}

	defer cs2.Close()

	// Data from first open should persist.
	stats, err := cs2.GetStats()
	if err != nil {
		t.Fatalf("GetStats() error: %v", err)
	}

	// Empty store is fine — just verify no error.
	if stats.Sources != 0 {
		t.Errorf("expected 0 sources in fresh store, got %d", stats.Sources)
	}
}
