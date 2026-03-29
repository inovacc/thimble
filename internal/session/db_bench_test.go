package session

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/inovacc/thimble/internal/model"
)

// benchSessionDB creates a SessionDB in a temp directory for benchmarks.
func benchSessionDB(b *testing.B) *SessionDB {
	b.Helper()

	dir := b.TempDir()

	sdb, err := New(filepath.Join(dir, "bench-session.db"))
	if err != nil {
		b.Fatalf("New: %v", err)
	}

	b.Cleanup(func() { sdb.Close() })

	return sdb
}

// seedEvents inserts n events with distinct data into the given session.
func seedEvents(b *testing.B, sdb *SessionDB, sessionID string, n int) {
	b.Helper()

	categories := []string{"file", "task", "rule", "decision", "cwd", "error", "env", "git"}
	types := []string{"file_write", "file_read", "file_edit", "task_create", "rule_add", "decision_made", "cwd_change", "error_hit"}

	for i := range n {
		ev := model.SessionEvent{
			Type:     types[i%len(types)],
			Category: categories[i%len(categories)],
			Data:     fmt.Sprintf(`{"tool":"tool_%d","path":"/src/file_%d.go","detail":"event %d data payload"}`, i%50, i%200, i),
			Priority: (i % 3) + 1,
		}

		if err := sdb.InsertEvent(sessionID, ev, "PostToolUse"); err != nil {
			b.Fatalf("InsertEvent %d: %v", i, err)
		}
	}
}

// BenchmarkRecordEvent benchmarks inserting 1000 events into a single session.
func BenchmarkRecordEvent(b *testing.B) {
	sdb := benchSessionDB(b)
	sessionID := "bench-record"

	_ = sdb.EnsureSession(sessionID, "/tmp/project")

	b.ResetTimer()

	for i := range b.N {
		ev := model.SessionEvent{
			Type:     "file_write",
			Category: "file",
			Data:     fmt.Sprintf(`{"tool":"Write","path":"/src/main_%d.go","iteration":%d}`, i, i),
			Priority: 2,
		}

		_ = sdb.InsertEvent(sessionID, ev, "PostToolUse")
	}
}

// BenchmarkBuildResumeSnapshot builds a resume snapshot from 500 stored events.
func BenchmarkBuildResumeSnapshot(b *testing.B) {
	sdb := benchSessionDB(b)
	sessionID := "bench-snapshot"

	_ = sdb.EnsureSession(sessionID, "/tmp/project")
	seedEvents(b, sdb, sessionID, 500)

	events, err := sdb.GetEvents(sessionID, &GetEventsOpts{Limit: 500})
	if err != nil {
		b.Fatalf("GetEvents: %v", err)
	}

	b.ResetTimer()

	for range b.N {
		_ = BuildResumeSnapshot(events, &BuildSnapshotOpts{MaxBytes: 4096, CompactCount: 1})
	}
}

// BenchmarkEventsByType benchmarks the analytics query that groups events by type.
func BenchmarkEventsByType(b *testing.B) {
	sdb := benchSessionDB(b)
	sessionID := "bench-analytics"

	_ = sdb.EnsureSession(sessionID, "/tmp/project")
	seedEvents(b, sdb, sessionID, 500)

	b.ResetTimer()

	for range b.N {
		_, _ = sdb.EventsByType(sessionID)
	}
}
