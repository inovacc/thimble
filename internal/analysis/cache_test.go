package analysis

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewAnalysisCacheEmpty(t *testing.T) {
	c := NewAnalysisCache()
	if len(c.Entries) != 0 {
		t.Errorf("new cache should be empty, got %d entries", len(c.Entries))
	}
}

func TestCacheRoundTrip(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "cache.json")

	original := NewAnalysisCache()
	now := time.Now().Truncate(time.Second) // JSON loses sub-second on some platforms.

	original.Set("pkg/main.go", now, FileResult{
		Path:     "pkg/main.go",
		Package:  "pkg",
		Language: LangGo,
		Symbols: []Symbol{
			{Name: "Hello", Kind: KindFunction, Package: "pkg", Exported: true},
		},
	})
	original.Set("lib/util.py", now.Add(-time.Hour), FileResult{
		Path:     "lib/util.py",
		Package:  "lib",
		Language: LangPython,
	})

	if err := original.SaveCache(cachePath); err != nil {
		t.Fatalf("SaveCache: %v", err)
	}

	loaded, err := LoadCache(cachePath)
	if err != nil {
		t.Fatalf("LoadCache: %v", err)
	}

	if len(loaded.Entries) != 2 {
		t.Fatalf("loaded entries = %d, want 2", len(loaded.Entries))
	}

	entry, ok := loaded.Get("pkg/main.go")
	if !ok {
		t.Fatal("missing pkg/main.go in loaded cache")
	}

	if !entry.ModTime.Equal(now) {
		t.Errorf("ModTime = %v, want %v", entry.ModTime, now)
	}

	if entry.Result.Package != "pkg" {
		t.Errorf("Package = %q, want %q", entry.Result.Package, "pkg")
	}

	if len(entry.Result.Symbols) != 1 || entry.Result.Symbols[0].Name != "Hello" {
		t.Errorf("unexpected symbols: %v", entry.Result.Symbols)
	}
}

func TestLoadCacheMissingFile(t *testing.T) {
	c, err := LoadCache(filepath.Join(t.TempDir(), "nonexistent.json"))
	if err != nil {
		t.Fatalf("LoadCache missing file should not error: %v", err)
	}

	if len(c.Entries) != 0 {
		t.Errorf("expected empty cache for missing file, got %d entries", len(c.Entries))
	}
}

func TestLoadCacheCorruptedJSON(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "bad.json")
	_ = os.WriteFile(cachePath, []byte("{invalid json!!!"), 0o644)

	c, err := LoadCache(cachePath)
	if err != nil {
		t.Fatalf("LoadCache corrupted should not error: %v", err)
	}

	if len(c.Entries) != 0 {
		t.Errorf("expected empty cache for corrupted file, got %d entries", len(c.Entries))
	}
}

func TestCacheRemove(t *testing.T) {
	c := NewAnalysisCache()
	c.Set("a.go", time.Now(), FileResult{Path: "a.go"})
	c.Set("b.go", time.Now(), FileResult{Path: "b.go"})

	c.Remove("a.go")

	if _, ok := c.Get("a.go"); ok {
		t.Error("a.go should have been removed")
	}

	if _, ok := c.Get("b.go"); !ok {
		t.Error("b.go should still exist")
	}
}

func TestSaveCacheCreatesDirectories(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "a", "b", "c", "cache.json")

	c := NewAnalysisCache()
	c.Set("x.go", time.Now(), FileResult{Path: "x.go"})

	if err := c.SaveCache(nested); err != nil {
		t.Fatalf("SaveCache nested: %v", err)
	}

	if _, err := os.Stat(nested); err != nil {
		t.Fatalf("cache file not created: %v", err)
	}
}

func TestIncrementalSkipsUnchangedFiles(t *testing.T) {
	dir := t.TempDir()

	// Create two Go files.
	_ = os.WriteFile(filepath.Join(dir, "cached.go"), []byte(`package main
func Cached() {}
`), 0o644)

	_ = os.WriteFile(filepath.Join(dir, "fresh.go"), []byte(`package main
func Fresh() {}
`), 0o644)

	// Pre-populate cache with cached.go using its current mod time.
	cachePath := filepath.Join(dir, "test_cache.json")
	cache := NewAnalysisCache()

	info, _ := os.Stat(filepath.Join(dir, "cached.go"))
	cache.Set("cached.go", info.ModTime(), FileResult{
		Path:     filepath.Join(dir, "cached.go"),
		Package:  "main",
		Language: LangGo,
		Symbols: []Symbol{
			{Name: "Cached", Kind: KindFunction, Package: "main", Exported: true},
		},
	})

	if err := cache.SaveCache(cachePath); err != nil {
		t.Fatalf("SaveCache: %v", err)
	}

	analyzer := NewAnalyzer(dir)

	result, err := analyzer.AnalyzeIncrementalWithCache(cachePath)
	if err != nil {
		t.Fatalf("AnalyzeIncrementalWithCache: %v", err)
	}

	// Both files should appear in the result (one from cache, one freshly parsed).
	if result.Summary.TotalFiles != 2 {
		t.Errorf("TotalFiles = %d, want 2", result.Summary.TotalFiles)
	}

	// Verify cached.go used the cached result (symbol name preserved).
	foundCached := false

	for _, f := range result.Files {
		if filepath.Base(f.Path) == "cached.go" {
			foundCached = true

			if len(f.Symbols) != 1 || f.Symbols[0].Name != "Cached" {
				t.Errorf("cached.go symbols = %v, want [Cached]", f.Symbols)
			}
		}
	}

	if !foundCached {
		t.Error("cached.go not found in result")
	}
}

func TestIncrementalReparsesModifiedFiles(t *testing.T) {
	dir := t.TempDir()

	_ = os.WriteFile(filepath.Join(dir, "changing.go"), []byte(`package main
func OldFunc() {}
`), 0o644)

	// Pre-populate cache with a stale mod time.
	cachePath := filepath.Join(dir, "test_cache.json")
	cache := NewAnalysisCache()

	staleTime := time.Now().Add(-24 * time.Hour)
	cache.Set("changing.go", staleTime, FileResult{
		Path:     filepath.Join(dir, "changing.go"),
		Package:  "main",
		Language: LangGo,
		Symbols: []Symbol{
			{Name: "StaleFunc", Kind: KindFunction, Package: "main", Exported: true},
		},
	})

	if err := cache.SaveCache(cachePath); err != nil {
		t.Fatalf("SaveCache: %v", err)
	}

	analyzer := NewAnalyzer(dir)

	result, err := analyzer.AnalyzeIncrementalWithCache(cachePath)
	if err != nil {
		t.Fatalf("AnalyzeIncrementalWithCache: %v", err)
	}

	if result.Summary.TotalFiles != 1 {
		t.Fatalf("TotalFiles = %d, want 1", result.Summary.TotalFiles)
	}

	// The file should have been re-parsed since the mod time differs.
	if result.Files[0].Symbols[0].Name != "OldFunc" {
		t.Errorf("symbol = %q, want %q (stale cache should be replaced)", result.Files[0].Symbols[0].Name, "OldFunc")
	}

	// Verify cache was updated.
	updated, err := LoadCache(cachePath)
	if err != nil {
		t.Fatalf("LoadCache: %v", err)
	}

	entry, ok := updated.Get("changing.go")
	if !ok {
		t.Fatal("changing.go missing from updated cache")
	}

	if entry.Result.Symbols[0].Name != "OldFunc" {
		t.Errorf("updated cache symbol = %q, want %q", entry.Result.Symbols[0].Name, "OldFunc")
	}
}

func TestIncrementalPrunesDeletedFiles(t *testing.T) {
	dir := t.TempDir()

	_ = os.WriteFile(filepath.Join(dir, "kept.go"), []byte(`package main
func Kept() {}
`), 0o644)

	// Cache has an entry for a file that no longer exists.
	cachePath := filepath.Join(dir, "test_cache.json")
	cache := NewAnalysisCache()

	info, _ := os.Stat(filepath.Join(dir, "kept.go"))
	cache.Set("kept.go", info.ModTime(), FileResult{
		Path:    filepath.Join(dir, "kept.go"),
		Package: "main", Language: LangGo,
		Symbols: []Symbol{{Name: "Kept", Kind: KindFunction, Package: "main", Exported: true}},
	})
	cache.Set("deleted.go", time.Now(), FileResult{
		Path:    filepath.Join(dir, "deleted.go"),
		Package: "main", Language: LangGo,
		Symbols: []Symbol{{Name: "Gone", Kind: KindFunction, Package: "main", Exported: true}},
	})

	if err := cache.SaveCache(cachePath); err != nil {
		t.Fatalf("SaveCache: %v", err)
	}

	analyzer := NewAnalyzer(dir)

	result, err := analyzer.AnalyzeIncrementalWithCache(cachePath)
	if err != nil {
		t.Fatalf("AnalyzeIncrementalWithCache: %v", err)
	}

	// Only kept.go should appear.
	if result.Summary.TotalFiles != 1 {
		t.Errorf("TotalFiles = %d, want 1", result.Summary.TotalFiles)
	}

	// Verify deleted.go was pruned from cache.
	updated, err := LoadCache(cachePath)
	if err != nil {
		t.Fatalf("LoadCache: %v", err)
	}

	if _, ok := updated.Get("deleted.go"); ok {
		t.Error("deleted.go should have been pruned from cache")
	}
}
