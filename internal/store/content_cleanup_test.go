package store

import (
	"testing"
	"time"
)

func TestCleanupByAge(t *testing.T) {
	tests := []struct {
		name           string
		docs           []struct{ content, label string }
		maxAge         time.Duration
		wantSrcRemoved int
		wantSrcAfter   int
	}{
		{
			name: "no stale sources returns zero counts",
			docs: []struct{ content, label string }{
				{"# Recent\n\nFresh content.", "fresh-doc"},
			},
			maxAge:         24 * time.Hour, // 24h -- nothing is that old yet
			wantSrcRemoved: 0,
			wantSrcAfter:   1,
		},
		{
			name:           "empty store returns zero counts",
			docs:           nil,
			maxAge:         time.Hour,
			wantSrcRemoved: 0,
			wantSrcAfter:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cs := tempDB(t)

			for _, doc := range tt.docs {
				if _, err := cs.Index(doc.content, doc.label); err != nil {
					t.Fatalf("Index(%q): %v", doc.label, err)
				}
			}

			stats, err := cs.CleanupByAge(tt.maxAge)
			if err != nil {
				t.Fatalf("CleanupByAge: %v", err)
			}

			if stats.SourcesRemoved != tt.wantSrcRemoved {
				t.Errorf("SourcesRemoved = %d, want %d", stats.SourcesRemoved, tt.wantSrcRemoved)
			}

			// Verify remaining source count.
			storeStats, err := cs.GetStats()
			if err != nil {
				t.Fatalf("GetStats: %v", err)
			}

			if storeStats.Sources != tt.wantSrcAfter {
				t.Errorf("sources after cleanup = %d, want %d", storeStats.Sources, tt.wantSrcAfter)
			}
		})
	}
}

func TestCleanupStale(t *testing.T) {
	tests := []struct {
		name           string
		docs           []struct{ content, label string }
		cutoff         time.Time
		wantSrcRemoved int
		wantChunksGt0  bool
	}{
		{
			name: "cutoff in the future removes everything",
			docs: []struct{ content, label string }{
				{"# A\n\nContent A.", "doc-a"},
				{"# B\n\nContent B.", "doc-b"},
			},
			cutoff:         time.Now().Add(time.Hour),
			wantSrcRemoved: 2,
			wantChunksGt0:  true,
		},
		{
			name: "cutoff in the past removes nothing",
			docs: []struct{ content, label string }{
				{"# C\n\nContent C.", "doc-c"},
			},
			cutoff:         time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC),
			wantSrcRemoved: 0,
			wantChunksGt0:  false,
		},
		{
			name:           "empty store with future cutoff",
			docs:           nil,
			cutoff:         time.Now().Add(time.Hour),
			wantSrcRemoved: 0,
			wantChunksGt0:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cs := tempDB(t)

			for _, doc := range tt.docs {
				if _, err := cs.Index(doc.content, doc.label); err != nil {
					t.Fatalf("Index(%q): %v", doc.label, err)
				}
			}

			stats, err := cs.CleanupStale(tt.cutoff)
			if err != nil {
				t.Fatalf("CleanupStale: %v", err)
			}

			if stats.SourcesRemoved != tt.wantSrcRemoved {
				t.Errorf("SourcesRemoved = %d, want %d", stats.SourcesRemoved, tt.wantSrcRemoved)
			}

			if tt.wantChunksGt0 && stats.ChunksRemoved == 0 {
				t.Error("expected ChunksRemoved > 0")
			}

			if tt.wantChunksGt0 && stats.BytesFreed == 0 {
				t.Error("expected BytesFreed > 0")
			}

			// Verify store is actually empty after removing all.
			if tt.wantSrcRemoved > 0 {
				storeStats, err := cs.GetStats()
				if err != nil {
					t.Fatalf("GetStats: %v", err)
				}

				if storeStats.Sources != 0 {
					t.Errorf("sources after cleanup = %d, want 0", storeStats.Sources)
				}

				if storeStats.Chunks != 0 {
					t.Errorf("chunks after cleanup = %d, want 0", storeStats.Chunks)
				}
			}
		})
	}
}

func TestVacuum(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(*ContentStore)
		wantErr bool
	}{
		{
			name:    "vacuum on empty db succeeds",
			setup:   func(_ *ContentStore) {},
			wantErr: false,
		},
		{
			name: "vacuum after cleanup succeeds",
			setup: func(cs *ContentStore) {
				_, _ = cs.Index("# Temp\n\nTemporary content.", "temp-doc")
				_, _ = cs.CleanupStale(time.Now().Add(time.Hour))
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cs := tempDB(t)

			tt.setup(cs)

			err := cs.Vacuum()
			if (err != nil) != tt.wantErr {
				t.Errorf("Vacuum() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMostRecentIndexedAt(t *testing.T) {
	tests := []struct {
		name     string
		docs     []struct{ content, label string }
		wantZero bool
	}{
		{
			name:     "empty store returns zero time",
			docs:     nil,
			wantZero: true,
		},
		{
			name: "store with docs returns non-zero time",
			docs: []struct{ content, label string }{
				{"# Doc\n\nContent.", "doc"},
			},
			wantZero: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cs := tempDB(t)

			for _, doc := range tt.docs {
				if _, err := cs.Index(doc.content, doc.label); err != nil {
					t.Fatalf("Index: %v", err)
				}
			}

			ts, err := cs.MostRecentIndexedAt()
			if err != nil {
				t.Fatalf("MostRecentIndexedAt: %v", err)
			}

			if tt.wantZero && !ts.IsZero() {
				t.Errorf("expected zero time, got %v", ts)
			}

			if !tt.wantZero && ts.IsZero() {
				t.Error("expected non-zero time, got zero")
			}
		})
	}
}

func TestCleanupByAgePreservesRecentData(t *testing.T) {
	cs := tempDB(t)

	// Index two documents.
	_, err := cs.Index("# Keep\n\nThis should remain after cleanup.", "keep-doc")
	if err != nil {
		t.Fatalf("Index: %v", err)
	}

	_, err = cs.Index("# Also Keep\n\nThis should also remain.", "also-keep")
	if err != nil {
		t.Fatalf("Index: %v", err)
	}

	// Cleanup with a long maxAge -- nothing should be removed.
	stats, err := cs.CleanupByAge(365 * 24 * time.Hour)
	if err != nil {
		t.Fatalf("CleanupByAge: %v", err)
	}

	if stats.SourcesRemoved != 0 {
		t.Errorf("SourcesRemoved = %d, want 0", stats.SourcesRemoved)
	}

	if stats.ChunksRemoved != 0 {
		t.Errorf("ChunksRemoved = %d, want 0", stats.ChunksRemoved)
	}

	if stats.BytesFreed != 0 {
		t.Errorf("BytesFreed = %d, want 0", stats.BytesFreed)
	}

	// Verify data is still searchable.
	results, err := cs.Search("remain cleanup", 3, "", "OR")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}

	if len(results) == 0 {
		t.Error("expected search results after non-destructive cleanup")
	}
}
