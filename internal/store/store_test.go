package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func tempDB(t *testing.T) *ContentStore {
	t.Helper()
	dir := t.TempDir()

	cs, err := New(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	t.Cleanup(func() { cs.Close() })

	return cs
}

func TestIndexAndSearch(t *testing.T) {
	cs := tempDB(t)

	result, err := cs.Index("# Hello\n\nThis is a test document about Go programming.", "test-doc")
	if err != nil {
		t.Fatalf("Index: %v", err)
	}

	if result.TotalChunks == 0 {
		t.Fatal("expected at least one chunk")
	}

	if result.Label != "test-doc" {
		t.Errorf("label = %q, want %q", result.Label, "test-doc")
	}

	results, err := cs.Search("Go programming", 3, "", "AND")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("expected search results for 'Go programming'")
	}

	if !strings.Contains(results[0].Content, "Go programming") {
		t.Errorf("result content = %q, expected to contain 'Go programming'", results[0].Content)
	}
}

func TestSearchWithFallback(t *testing.T) {
	cs := tempDB(t)

	_, err := cs.Index("# Kubernetes\n\nDeploying containers with kubectl apply.", "k8s-doc")
	if err != nil {
		t.Fatalf("Index: %v", err)
	}

	results, err := cs.SearchWithFallback("kubernetes deploy", 3, "")
	if err != nil {
		t.Fatalf("SearchWithFallback: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("expected fallback search results")
	}

	if results[0].MatchLayer == "" {
		t.Error("expected matchLayer to be set")
	}
}

func TestIndexPlainText(t *testing.T) {
	cs := tempDB(t)

	lines := make([]string, 50)
	for i := range lines {
		lines[i] = "line content for testing plain text indexing"
	}

	content := strings.Join(lines, "\n")

	result, err := cs.IndexPlainText(content, "output-log", 20)
	if err != nil {
		t.Fatalf("IndexPlainText: %v", err)
	}

	if result.TotalChunks < 2 {
		t.Errorf("expected multiple chunks, got %d", result.TotalChunks)
	}
}

func TestIndexJSON(t *testing.T) {
	cs := tempDB(t)

	jsonContent := `{
		"users": [
			{"id": 1, "name": "Alice"},
			{"id": 2, "name": "Bob"},
			{"id": 3, "name": "Charlie"}
		],
		"metadata": {"version": "1.0"}
	}`

	result, err := cs.IndexJSON(jsonContent, "api-response")
	if err != nil {
		t.Fatalf("IndexJSON: %v", err)
	}

	if result.TotalChunks == 0 {
		t.Fatal("expected chunks from JSON indexing")
	}

	results, err := cs.Search("Alice", 3, "", "AND")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("expected to find 'Alice' in indexed JSON")
	}
}

func TestDedup(t *testing.T) {
	cs := tempDB(t)

	_, err := cs.Index("# Version 1\n\nAlpha bravo unique phrase.", "my-doc")
	if err != nil {
		t.Fatalf("Index v1: %v", err)
	}

	_, err = cs.Index("# Version 2\n\nCompletely different replacement text.", "my-doc")
	if err != nil {
		t.Fatalf("Index v2: %v", err)
	}

	stats, err := cs.GetStats()
	if err != nil {
		t.Fatalf("GetStats: %v", err)
	}

	if stats.Sources != 1 {
		t.Errorf("sources = %d, want 1 (dedup should remove old)", stats.Sources)
	}

	// Search for a phrase only in v1 — should return nothing after dedup.
	results, err := cs.Search("alpha bravo unique", 3, "", "AND")
	if err != nil {
		t.Fatalf("Search old: %v", err)
	}

	if len(results) > 0 {
		t.Error("old content should have been removed by dedup")
	}
}

func TestGetStats(t *testing.T) {
	cs := tempDB(t)

	stats, err := cs.GetStats()
	if err != nil {
		t.Fatalf("GetStats: %v", err)
	}

	if stats.Sources != 0 || stats.Chunks != 0 {
		t.Errorf("empty store: sources=%d chunks=%d", stats.Sources, stats.Chunks)
	}

	_, _ = cs.Index("# Code\n\n```go\nfmt.Println(\"hello\")\n```", "code-doc")

	stats, err = cs.GetStats()
	if err != nil {
		t.Fatalf("GetStats after index: %v", err)
	}

	if stats.Sources != 1 {
		t.Errorf("sources = %d, want 1", stats.Sources)
	}

	if stats.Chunks == 0 {
		t.Error("expected at least one chunk")
	}

	if stats.CodeChunks == 0 {
		t.Error("expected at least one code chunk")
	}
}

func TestListSources(t *testing.T) {
	cs := tempDB(t)

	_, _ = cs.Index("# Doc A\n\nContent A.", "doc-a")
	_, _ = cs.Index("# Doc B\n\nContent B.", "doc-b")

	sources, err := cs.ListSources()
	if err != nil {
		t.Fatalf("ListSources: %v", err)
	}

	if len(sources) != 2 {
		t.Errorf("sources = %d, want 2", len(sources))
	}
}

func TestFuzzyCorrect(t *testing.T) {
	cs := tempDB(t)

	// Index content to populate vocabulary.
	_, _ = cs.Index("# Kubernetes\n\norchestration platform.", "k8s")

	corrected := cs.FuzzyCorrect("kuberntes")
	if corrected != "kubernetes" {
		t.Errorf("FuzzyCorrect(%q) = %q, want %q", "kuberntes", corrected, "kubernetes")
	}

	// Exact match returns empty (no correction needed).
	exact := cs.FuzzyCorrect("kubernetes")
	if exact != "" {
		t.Errorf("FuzzyCorrect(exact) = %q, want empty", exact)
	}
}

func TestCleanupStaleDBs(t *testing.T) {
	dir := t.TempDir()

	// Create a fake stale DB file with a non-existent PID.
	staleFile := filepath.Join(dir, "thimble-99999999.db")
	if err := os.WriteFile(staleFile, []byte("fake"), 0o644); err != nil {
		t.Fatal(err)
	}

	cleaned := CleanupStaleDBs(dir)
	if cleaned != 1 {
		t.Errorf("cleaned = %d, want 1", cleaned)
	}

	if _, err := os.Stat(staleFile); !os.IsNotExist(err) {
		t.Error("stale DB file should have been removed")
	}
}

func TestEmptyIndex(t *testing.T) {
	cs := tempDB(t)

	result, err := cs.Index("", "empty")
	if err != nil {
		t.Fatalf("Index empty: %v", err)
	}

	if result.TotalChunks != 0 {
		t.Errorf("chunks = %d, want 0", result.TotalChunks)
	}
}
