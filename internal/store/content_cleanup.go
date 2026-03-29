package store

import (
	"fmt"
	"time"
)

// CleanupStats holds cleanup results.
type CleanupStats struct {
	SourcesRemoved int
	ChunksRemoved  int
	BytesFreed     int64
}

// CleanupStale removes sources not accessed since the given cutoff time.
// It deletes the source rows and their associated chunks from both FTS5 tables,
// then returns counts of what was removed.
func (cs *ContentStore) CleanupStale(cutoff time.Time) (CleanupStats, error) {
	cutoffStr := cutoff.UTC().Format("2006-01-02 15:04:05")

	tx, err := cs.db.Begin()
	if err != nil {
		return CleanupStats{}, fmt.Errorf("begin cleanup tx: %w", err)
	}

	defer func() { _ = tx.Rollback() }()

	// Count sources and chunks that will be removed.
	var stats CleanupStats

	if err := tx.QueryRow(
		"SELECT COUNT(*) FROM sources WHERE indexed_at < ?", cutoffStr,
	).Scan(&stats.SourcesRemoved); err != nil {
		return CleanupStats{}, fmt.Errorf("count stale sources: %w", err)
	}

	if stats.SourcesRemoved == 0 {
		return CleanupStats{}, nil
	}

	// Count chunks that belong to stale sources.
	if err := tx.QueryRow(`
		SELECT COUNT(*) FROM chunks
		WHERE rowid IN (
			SELECT chunks.rowid FROM chunks
			JOIN sources ON sources.id = chunks.source_id
			WHERE sources.indexed_at < ?
		)`, cutoffStr,
	).Scan(&stats.ChunksRemoved); err != nil {
		return CleanupStats{}, fmt.Errorf("count stale chunks: %w", err)
	}

	// Estimate bytes freed (sum of content lengths from stale chunks).
	var bytesFreed int64

	_ = tx.QueryRow(`
		SELECT COALESCE(SUM(LENGTH(content)), 0) FROM chunks
		WHERE rowid IN (
			SELECT chunks.rowid FROM chunks
			JOIN sources ON sources.id = chunks.source_id
			WHERE sources.indexed_at < ?
		)`, cutoffStr,
	).Scan(&bytesFreed)

	stats.BytesFreed = bytesFreed

	// Delete chunks from both FTS5 tables, then sources.
	if _, err := tx.Exec(`
		DELETE FROM chunks WHERE rowid IN (
			SELECT chunks.rowid FROM chunks
			JOIN sources ON sources.id = chunks.source_id
			WHERE sources.indexed_at < ?
		)`, cutoffStr); err != nil {
		return CleanupStats{}, fmt.Errorf("delete stale chunks: %w", err)
	}

	if _, err := tx.Exec(`
		DELETE FROM chunks_trigram WHERE rowid IN (
			SELECT chunks_trigram.rowid FROM chunks_trigram
			JOIN sources ON sources.id = chunks_trigram.source_id
			WHERE sources.indexed_at < ?
		)`, cutoffStr); err != nil {
		return CleanupStats{}, fmt.Errorf("delete stale trigram chunks: %w", err)
	}

	if _, err := tx.Exec(
		"DELETE FROM sources WHERE indexed_at < ?", cutoffStr,
	); err != nil {
		return CleanupStats{}, fmt.Errorf("delete stale sources: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return CleanupStats{}, fmt.Errorf("commit cleanup: %w", err)
	}

	return stats, nil
}

// CleanupByAge removes sources older than maxAge duration.
func (cs *ContentStore) CleanupByAge(maxAge time.Duration) (CleanupStats, error) {
	cutoff := time.Now().Add(-maxAge)
	return cs.CleanupStale(cutoff)
}

// Vacuum runs SQLite VACUUM to reclaim disk space after cleanup.
func (cs *ContentStore) Vacuum() error {
	_, err := cs.db.Exec("VACUUM")
	if err != nil {
		return fmt.Errorf("vacuum: %w", err)
	}

	return nil
}

// MostRecentIndexedAt returns the most recent indexed_at timestamp across
// all sources, or the zero time if the store is empty.
func (cs *ContentStore) MostRecentIndexedAt() (time.Time, error) {
	var ts *string

	err := cs.db.QueryRow(
		"SELECT MAX(indexed_at) FROM sources",
	).Scan(&ts)
	if err != nil {
		return time.Time{}, fmt.Errorf("query most recent indexed_at: %w", err)
	}

	if ts == nil {
		return time.Time{}, nil
	}

	parsed, err := time.Parse("2006-01-02 15:04:05", *ts)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse indexed_at %q: %w", *ts, err)
	}

	return parsed, nil
}
