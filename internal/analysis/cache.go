package analysis

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// CacheEntry holds a cached analysis result for a single file along with
// the file's last modification time at the point of analysis.
type CacheEntry struct {
	ModTime time.Time  `json:"mod_time"`
	Result  FileResult `json:"result"`
}

// AnalysisCache stores per-file analysis results keyed by relative path.
// It enables incremental analysis by skipping files whose mod time has
// not changed since the last run.
type AnalysisCache struct {
	Entries map[string]CacheEntry `json:"entries"`
}

// NewAnalysisCache creates an empty cache.
func NewAnalysisCache() *AnalysisCache {
	return &AnalysisCache{
		Entries: make(map[string]CacheEntry),
	}
}

// LoadCache reads and deserializes a cache from the given JSON file path.
// If the file does not exist, a fresh empty cache is returned (no error).
func LoadCache(path string) (*AnalysisCache, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return NewAnalysisCache(), nil
		}

		return nil, fmt.Errorf("read cache %s: %w", path, err)
	}

	var cache AnalysisCache
	if err := json.Unmarshal(data, &cache); err != nil {
		// Corrupted cache: start fresh.
		return NewAnalysisCache(), nil //nolint:nilerr // intentional: treat corrupt cache as empty
	}

	if cache.Entries == nil {
		cache.Entries = make(map[string]CacheEntry)
	}

	return &cache, nil
}

// SaveCache serializes the cache to the given JSON file path, creating
// parent directories as needed.
func (c *AnalysisCache) SaveCache(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir for cache: %w", err)
	}

	data, err := json.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal cache: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write cache %s: %w", path, err)
	}

	return nil
}

// Get returns the cached entry for the given relative path and whether it was found.
func (c *AnalysisCache) Get(relPath string) (CacheEntry, bool) {
	entry, ok := c.Entries[relPath]
	return entry, ok
}

// Set stores a file result in the cache with the given mod time.
func (c *AnalysisCache) Set(relPath string, modTime time.Time, result FileResult) {
	c.Entries[relPath] = CacheEntry{
		ModTime: modTime,
		Result:  result,
	}
}

// Remove deletes a cache entry for the given relative path.
func (c *AnalysisCache) Remove(relPath string) {
	delete(c.Entries, relPath)
}
