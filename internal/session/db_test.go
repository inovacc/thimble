package session

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/inovacc/thimble/internal/model"
)

func tempSessionDB(t *testing.T) *SessionDB {
	t.Helper()
	dir := t.TempDir()

	sdb, err := New(filepath.Join(dir, "test-session.db"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	t.Cleanup(func() { sdb.Close() })

	return sdb
}

func TestInsertAndGetEvents(t *testing.T) {
	sdb := tempSessionDB(t)
	sid := "test-session-1"

	_ = sdb.EnsureSession(sid, "/tmp/project")

	err := sdb.InsertEvent(sid, model.SessionEvent{
		Type: "file_read", Category: "file",
		Data: "/path/to/file.go", Priority: 1,
	}, "PostToolUse")
	if err != nil {
		t.Fatalf("InsertEvent: %v", err)
	}

	events, err := sdb.GetEvents(sid, nil)
	if err != nil {
		t.Fatalf("GetEvents: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("events = %d, want 1", len(events))
	}

	if events[0].Type != "file_read" {
		t.Errorf("type = %q, want %q", events[0].Type, "file_read")
	}

	if events[0].Data != "/path/to/file.go" {
		t.Errorf("data = %q, want %q", events[0].Data, "/path/to/file.go")
	}
}

func TestDeduplication(t *testing.T) {
	sdb := tempSessionDB(t)
	sid := "dedup-session"
	_ = sdb.EnsureSession(sid, "/tmp/project")

	event := model.SessionEvent{
		Type: "file_read", Category: "file",
		Data: "/path/to/same-file.go", Priority: 1,
	}

	// Insert the same event twice.
	_ = sdb.InsertEvent(sid, event, "PostToolUse")
	_ = sdb.InsertEvent(sid, event, "PostToolUse")

	cnt, err := sdb.GetEventCount(sid)
	if err != nil {
		t.Fatalf("GetEventCount: %v", err)
	}

	if cnt != 1 {
		t.Errorf("count = %d, want 1 (dedup should prevent duplicate)", cnt)
	}
}

func TestFIFOEviction(t *testing.T) {
	sdb := tempSessionDB(t)
	sid := "eviction-session"
	_ = sdb.EnsureSession(sid, "/tmp/project")

	// Fill to max.
	for i := range MaxEventsPerSession {
		err := sdb.InsertEvent(sid, model.SessionEvent{
			Type: "file_read", Category: "file",
			Data:     "/path/to/file-" + string(rune('A'+i%26)) + ".go",
			Priority: 2,
		}, "PostToolUse")
		if err != nil {
			t.Fatalf("InsertEvent %d: %v", i, err)
		}
	}

	cnt, _ := sdb.GetEventCount(sid)
	if cnt != MaxEventsPerSession {
		t.Fatalf("count = %d, want %d", cnt, MaxEventsPerSession)
	}

	// Insert one more — should evict lowest priority.
	_ = sdb.InsertEvent(sid, model.SessionEvent{
		Type: "git", Category: "git",
		Data: "commit", Priority: 3,
	}, "PostToolUse")

	cnt, _ = sdb.GetEventCount(sid)
	if cnt != MaxEventsPerSession {
		t.Errorf("count = %d after eviction, want %d", cnt, MaxEventsPerSession)
	}
}

func TestGetEventsFiltered(t *testing.T) {
	sdb := tempSessionDB(t)
	sid := "filter-session"
	_ = sdb.EnsureSession(sid, "/tmp/project")

	_ = sdb.InsertEvent(sid, model.SessionEvent{
		Type: "file_read", Category: "file", Data: "a.go", Priority: 1,
	}, "")
	_ = sdb.InsertEvent(sid, model.SessionEvent{
		Type: "git", Category: "git", Data: "commit", Priority: 2,
	}, "")
	_ = sdb.InsertEvent(sid, model.SessionEvent{
		Type: "file_write", Category: "file", Data: "b.go", Priority: 1,
	}, "")

	// Filter by type.
	events, _ := sdb.GetEvents(sid, &GetEventsOpts{Type: "git"})
	if len(events) != 1 {
		t.Errorf("type filter: got %d events, want 1", len(events))
	}

	// Filter by priority.
	events, _ = sdb.GetEvents(sid, &GetEventsOpts{MinPriority: 2})
	if len(events) != 1 {
		t.Errorf("priority filter: got %d events, want 1", len(events))
	}
}

func TestEnsureSessionAndStats(t *testing.T) {
	sdb := tempSessionDB(t)
	sid := "stats-session"

	_ = sdb.EnsureSession(sid, "/home/user/project")
	// Idempotent — should not error.
	_ = sdb.EnsureSession(sid, "/home/user/project")

	meta, err := sdb.GetSessionStats(sid)
	if err != nil {
		t.Fatalf("GetSessionStats: %v", err)
	}

	if meta == nil {
		t.Fatal("meta is nil")
	}

	if meta.ProjectDir != "/home/user/project" {
		t.Errorf("project_dir = %q", meta.ProjectDir)
	}

	if meta.EventCount != 0 {
		t.Errorf("event_count = %d, want 0", meta.EventCount)
	}
}

func TestResumeLifecycle(t *testing.T) {
	sdb := tempSessionDB(t)
	sid := "resume-session"

	// No resume initially.
	r, err := sdb.GetResume(sid)
	if err != nil {
		t.Fatalf("GetResume: %v", err)
	}

	if r != nil {
		t.Fatal("expected nil resume")
	}

	// Upsert.
	_ = sdb.UpsertResume(sid, "<snapshot>data</snapshot>", 5)

	r, _ = sdb.GetResume(sid)
	if r == nil {
		t.Fatal("expected resume after upsert")
	}

	if r.Snapshot != "<snapshot>data</snapshot>" {
		t.Errorf("snapshot = %q", r.Snapshot)
	}

	if r.Consumed != 0 {
		t.Errorf("consumed = %d, want 0", r.Consumed)
	}

	// Mark consumed.
	_ = sdb.MarkResumeConsumed(sid)

	r, _ = sdb.GetResume(sid)
	if r.Consumed != 1 {
		t.Errorf("consumed = %d, want 1", r.Consumed)
	}

	// Upsert again — should reset consumed.
	_ = sdb.UpsertResume(sid, "<snapshot>updated</snapshot>", 10)

	r, _ = sdb.GetResume(sid)
	if r.Consumed != 0 {
		t.Errorf("consumed after re-upsert = %d, want 0", r.Consumed)
	}
}

func TestDeleteSession(t *testing.T) {
	sdb := tempSessionDB(t)
	sid := "delete-session"

	_ = sdb.EnsureSession(sid, "/tmp")
	_ = sdb.InsertEvent(sid, model.SessionEvent{
		Type: "file_read", Category: "file", Data: "x.go", Priority: 1,
	}, "")
	_ = sdb.UpsertResume(sid, "snap", 1)

	_ = sdb.DeleteSession(sid)

	cnt, _ := sdb.GetEventCount(sid)
	if cnt != 0 {
		t.Errorf("events after delete = %d", cnt)
	}

	meta, _ := sdb.GetSessionStats(sid)
	if meta != nil {
		t.Error("meta should be nil after delete")
	}

	r, _ := sdb.GetResume(sid)
	if r != nil {
		t.Error("resume should be nil after delete")
	}
}

func TestTouchActivityAndGet(t *testing.T) {
	sdb := tempSessionDB(t)
	sid := "activity-session"

	// Initially no activity.
	a, err := sdb.GetActivity(sid)
	if err != nil {
		t.Fatalf("GetActivity: %v", err)
	}

	if a != nil {
		t.Fatal("expected nil activity initially")
	}

	// Touch with a tool call.
	if err := sdb.TouchActivity(sid, "thimble", "/home/user/project", false); err != nil {
		t.Fatalf("TouchActivity (tool_call): %v", err)
	}

	a, err = sdb.GetActivity(sid)
	if err != nil {
		t.Fatalf("GetActivity after touch: %v", err)
	}

	if a == nil {
		t.Fatal("expected activity after touch")
	}

	if a.ToolCalls != 1 {
		t.Errorf("tool_calls = %d, want 1", a.ToolCalls)
	}

	if a.Queries != 0 {
		t.Errorf("queries = %d, want 0", a.Queries)
	}

	if a.AppName != "thimble" {
		t.Errorf("app_name = %q, want %q", a.AppName, "thimble")
	}

	// Touch with a query.
	if err := sdb.TouchActivity(sid, "thimble", "/home/user/project", true); err != nil {
		t.Fatalf("TouchActivity (query): %v", err)
	}

	a, _ = sdb.GetActivity(sid)
	if a.ToolCalls != 1 {
		t.Errorf("tool_calls = %d, want 1", a.ToolCalls)
	}

	if a.Queries != 1 {
		t.Errorf("queries = %d, want 1", a.Queries)
	}
}

func TestEvictStaleSessions(t *testing.T) {
	sdb := tempSessionDB(t)
	sid := "stale-session"

	_ = sdb.EnsureSession(sid, "/tmp")
	_ = sdb.TouchActivity(sid, "thimble", "/tmp", false)

	// Evict with 0 days should remove it (last_used_at is "now", but 0-day threshold = now).
	// Actually 0 defaults to 7. Sessions just created won't be evicted.
	evicted, err := sdb.EvictStaleSessions(7)
	if err != nil {
		t.Fatalf("EvictStaleSessions: %v", err)
	}

	if evicted != 0 {
		t.Errorf("evicted = %d, want 0 (session is fresh)", evicted)
	}
}

func TestDedupWindowConstant(t *testing.T) {
	if DedupWindow != 15 {
		t.Errorf("DedupWindow = %d, want 15", DedupWindow)
	}
}

func TestDedupWindowCoversDistantDuplicates(t *testing.T) {
	sdb := tempSessionDB(t)
	sid := "dedup-window-session"
	_ = sdb.EnsureSession(sid, "/tmp/project")

	// Insert the target event first.
	target := model.SessionEvent{
		Type: "file_read", Category: "file",
		Data: "/path/to/target.go", Priority: 1,
	}
	_ = sdb.InsertEvent(sid, target, "PostToolUse")

	// Insert 14 different events (still within the 15-event window).
	for i := range 14 {
		_ = sdb.InsertEvent(sid, model.SessionEvent{
			Type: "file_read", Category: "file",
			Data: fmt.Sprintf("/path/to/other-%d.go", i), Priority: 1,
		}, "PostToolUse")
	}

	// Re-insert the target — should be deduped (within window of 15).
	_ = sdb.InsertEvent(sid, target, "PostToolUse")

	cnt, _ := sdb.GetEventCount(sid)
	if cnt != 15 {
		t.Errorf("count = %d, want 15 (target duplicate should be deduped within window)", cnt)
	}
}

func TestIncrementCompactCount(t *testing.T) {
	sdb := tempSessionDB(t)
	sid := "compact-session"
	_ = sdb.EnsureSession(sid, "/tmp")

	_ = sdb.IncrementCompactCount(sid)
	_ = sdb.IncrementCompactCount(sid)

	meta, _ := sdb.GetSessionStats(sid)
	if meta.CompactCount != 2 {
		t.Errorf("compact_count = %d, want 2", meta.CompactCount)
	}
}

func TestEventsByType(t *testing.T) {
	sdb := tempSessionDB(t)
	sid := "types-session"
	_ = sdb.EnsureSession(sid, "/tmp")

	_ = sdb.InsertEvent(sid, model.SessionEvent{
		Type: "file_read", Category: "file", Data: "a.go", Priority: 1,
	}, "")
	_ = sdb.InsertEvent(sid, model.SessionEvent{
		Type: "file_read", Category: "file", Data: "b.go", Priority: 1,
	}, "")
	_ = sdb.InsertEvent(sid, model.SessionEvent{
		Type: "git", Category: "git", Data: "commit", Priority: 2,
	}, "")
	_ = sdb.InsertEvent(sid, model.SessionEvent{
		Type: "error", Category: "error", Data: "something failed", Priority: 3,
	}, "")

	byType, err := sdb.EventsByType(sid)
	if err != nil {
		t.Fatalf("EventsByType: %v", err)
	}

	if byType["file_read"] != 2 {
		t.Errorf("file_read = %d, want 2", byType["file_read"])
	}

	if byType["git"] != 1 {
		t.Errorf("git = %d, want 1", byType["git"])
	}

	if byType["error"] != 1 {
		t.Errorf("error = %d, want 1", byType["error"])
	}
}

func TestEventsByTypeEmpty(t *testing.T) {
	sdb := tempSessionDB(t)

	byType, err := sdb.EventsByType("nonexistent")
	if err != nil {
		t.Fatalf("EventsByType: %v", err)
	}

	if len(byType) != 0 {
		t.Errorf("expected empty map, got %d entries", len(byType))
	}
}

func TestTopTools(t *testing.T) {
	sdb := tempSessionDB(t)
	sid := "top-tools-session"
	_ = sdb.EnsureSession(sid, "/tmp")

	// Insert events with tool JSON in data field.
	for i := range 5 {
		_ = sdb.InsertEvent(sid, model.SessionEvent{
			Type: "tool_call", Category: "tool",
			Data:     fmt.Sprintf(`{"tool":"ctx_search","session":"s1","call":%d}`, i),
			Priority: 2,
		}, "mcp")
	}

	for i := range 3 {
		_ = sdb.InsertEvent(sid, model.SessionEvent{
			Type: "tool_call", Category: "tool",
			Data:     fmt.Sprintf(`{"tool":"ctx_execute","session":"s1","call":%d}`, i),
			Priority: 2,
		}, "mcp")
	}

	_ = sdb.InsertEvent(sid, model.SessionEvent{
		Type: "tool_call", Category: "tool",
		Data:     `{"tool":"ctx_index","session":"s1"}`,
		Priority: 2,
	}, "mcp")

	top, err := sdb.TopTools(sid, 10)
	if err != nil {
		t.Fatalf("TopTools: %v", err)
	}

	if len(top) != 3 {
		t.Fatalf("top tools = %d, want 3", len(top))
	}

	if top[0].Name != "ctx_search" || top[0].Count != 5 {
		t.Errorf("top[0] = %+v, want ctx_search/5", top[0])
	}

	if top[1].Name != "ctx_execute" || top[1].Count != 3 {
		t.Errorf("top[1] = %+v, want ctx_execute/3", top[1])
	}

	if top[2].Name != "ctx_index" || top[2].Count != 1 {
		t.Errorf("top[2] = %+v, want ctx_index/1", top[2])
	}
}

func TestTopToolsEmpty(t *testing.T) {
	sdb := tempSessionDB(t)

	top, err := sdb.TopTools("nonexistent", 10)
	if err != nil {
		t.Fatalf("TopTools: %v", err)
	}

	if len(top) != 0 {
		t.Errorf("expected empty, got %d", len(top))
	}
}

func TestTopToolsDefaultLimit(t *testing.T) {
	sdb := tempSessionDB(t)
	sid := "limit-session"
	_ = sdb.EnsureSession(sid, "/tmp")

	// With 0 limit, should default to 10.
	top, err := sdb.TopTools(sid, 0)
	if err != nil {
		t.Fatalf("TopTools with 0 limit: %v", err)
	}

	// Just verify it doesn't error; empty is fine.
	_ = top
}

func TestSessionDuration(t *testing.T) {
	sdb := tempSessionDB(t)
	sid := "duration-session"
	_ = sdb.EnsureSession(sid, "/tmp")

	// Insert events — they get created_at = datetime('now') so duration will be ~0.
	_ = sdb.InsertEvent(sid, model.SessionEvent{
		Type: "file_read", Category: "file", Data: "a.go", Priority: 1,
	}, "")
	_ = sdb.InsertEvent(sid, model.SessionEvent{
		Type: "git", Category: "git", Data: "commit", Priority: 2,
	}, "")

	dur, err := sdb.SessionDuration(sid)
	if err != nil {
		t.Fatalf("SessionDuration: %v", err)
	}

	// Both events inserted nearly simultaneously, so duration should be ~0.
	if dur < 0 {
		t.Errorf("duration = %v, expected >= 0", dur)
	}
}

func TestSessionDurationEmpty(t *testing.T) {
	sdb := tempSessionDB(t)

	dur, err := sdb.SessionDuration("nonexistent")
	if err != nil {
		t.Fatalf("SessionDuration: %v", err)
	}

	if dur != 0 {
		t.Errorf("duration = %v, want 0 for empty session", dur)
	}
}
