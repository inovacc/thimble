package session

import (
	"testing"

	"github.com/inovacc/thimble/internal/model"
)

func TestExportImportRoundTrip(t *testing.T) {
	srcDB := tempSessionDB(t)
	dstDB := tempSessionDB(t)

	sid := "export-test-session"
	_ = srcDB.EnsureSession(sid, "/tmp/test-project")

	// Insert several events.
	events := []model.SessionEvent{
		{Type: "file_read", Category: "file", Data: "/path/to/a.go", Priority: 1},
		{Type: "file_write", Category: "file", Data: "/path/to/b.go", Priority: 2},
		{Type: "tool_call", Category: "tool", Data: `{"tool":"ctx_search"}`, Priority: 3},
	}

	for _, e := range events {
		if err := srcDB.InsertEvent(sid, e, "PostToolUse"); err != nil {
			t.Fatalf("InsertEvent: %v", err)
		}
	}

	// Add a resume snapshot.
	if err := srcDB.UpsertResume(sid, "test snapshot content", 3); err != nil {
		t.Fatalf("UpsertResume: %v", err)
	}

	// Export.
	exported, err := srcDB.ExportSession(sid)
	if err != nil {
		t.Fatalf("ExportSession: %v", err)
	}

	if exported.Version != "1" {
		t.Errorf("version = %q, want %q", exported.Version, "1")
	}

	if exported.SessionID != sid {
		t.Errorf("session_id = %q, want %q", exported.SessionID, sid)
	}

	if len(exported.Events) != 3 {
		t.Fatalf("events = %d, want 3", len(exported.Events))
	}

	if exported.Metadata["project_dir"] != "/tmp/test-project" {
		t.Errorf("project_dir = %q, want %q", exported.Metadata["project_dir"], "/tmp/test-project")
	}

	if exported.ResumeSnapshot != "test snapshot content" {
		t.Errorf("resume_snapshot = %q, want %q", exported.ResumeSnapshot, "test snapshot content")
	}

	// Serialize to JSON and back.
	jsonBytes, err := ExportJSON(exported)
	if err != nil {
		t.Fatalf("ExportJSON: %v", err)
	}

	reimported, err := ImportJSON(jsonBytes)
	if err != nil {
		t.Fatalf("ImportJSON: %v", err)
	}

	// Import into destination DB.
	if err := dstDB.ImportSession(reimported); err != nil {
		t.Fatalf("ImportSession: %v", err)
	}

	// Verify events in destination.
	dstEvents, err := dstDB.GetEvents(sid, nil)
	if err != nil {
		t.Fatalf("GetEvents on dst: %v", err)
	}

	if len(dstEvents) != 3 {
		t.Fatalf("dst events = %d, want 3", len(dstEvents))
	}

	// Verify event types match.
	wantTypes := []string{"file_read", "file_write", "tool_call"}
	for i, want := range wantTypes {
		if dstEvents[i].Type != want {
			t.Errorf("event[%d].type = %q, want %q", i, dstEvents[i].Type, want)
		}
	}

	// Verify metadata was preserved.
	meta, err := dstDB.GetSessionStats(sid)
	if err != nil {
		t.Fatalf("GetSessionStats on dst: %v", err)
	}

	if meta == nil {
		t.Fatal("session meta is nil in dst")
	}

	if meta.ProjectDir != "/tmp/test-project" {
		t.Errorf("dst project_dir = %q, want %q", meta.ProjectDir, "/tmp/test-project")
	}

	if meta.EventCount != 3 {
		t.Errorf("dst event_count = %d, want 3", meta.EventCount)
	}

	// Verify resume snapshot was preserved.
	resume, err := dstDB.GetResume(sid)
	if err != nil {
		t.Fatalf("GetResume on dst: %v", err)
	}

	if resume == nil {
		t.Fatal("resume is nil in dst")
	}

	if resume.Snapshot != "test snapshot content" {
		t.Errorf("dst snapshot = %q, want %q", resume.Snapshot, "test snapshot content")
	}
}

func TestExportEmptySession(t *testing.T) {
	sdb := tempSessionDB(t)
	sid := "empty-session"
	_ = sdb.EnsureSession(sid, "/tmp/empty")

	exported, err := sdb.ExportSession(sid)
	if err != nil {
		t.Fatalf("ExportSession: %v", err)
	}

	if len(exported.Events) != 0 {
		t.Errorf("events = %d, want 0", len(exported.Events))
	}

	if exported.ResumeSnapshot != "" {
		t.Errorf("resume should be empty, got %q", exported.ResumeSnapshot)
	}
}

func TestImportNilExport(t *testing.T) {
	sdb := tempSessionDB(t)

	if err := sdb.ImportSession(nil); err == nil {
		t.Fatal("expected error for nil import")
	}
}

func TestImportEmptySessionID(t *testing.T) {
	sdb := tempSessionDB(t)

	if err := sdb.ImportSession(&SessionExport{}); err == nil {
		t.Fatal("expected error for empty session ID")
	}
}

func TestListSessions(t *testing.T) {
	sdb := tempSessionDB(t)

	// Create two sessions.
	_ = sdb.EnsureSession("session-a", "/tmp/a")
	_ = sdb.EnsureSession("session-b", "/tmp/b")

	_ = sdb.InsertEvent("session-a", model.SessionEvent{
		Type: "file_read", Category: "file", Data: "file1.go", Priority: 1,
	}, "PostToolUse")

	_ = sdb.InsertEvent("session-b", model.SessionEvent{
		Type: "file_read", Category: "file", Data: "file2.go", Priority: 1,
	}, "PostToolUse")

	_ = sdb.InsertEvent("session-b", model.SessionEvent{
		Type: "file_write", Category: "file", Data: "file3.go", Priority: 2,
	}, "PostToolUse")

	sessions, err := sdb.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}

	if len(sessions) != 2 {
		t.Fatalf("sessions = %d, want 2", len(sessions))
	}

	// Find session-b (should have 2 events).
	found := false

	for _, s := range sessions {
		if s.SessionID == "session-b" {
			found = true

			if s.EventCount != 2 {
				t.Errorf("session-b event_count = %d, want 2", s.EventCount)
			}
		}
	}

	if !found {
		t.Error("session-b not found in list")
	}
}

func TestExportImportPreservesTimestamps(t *testing.T) {
	srcDB := tempSessionDB(t)
	dstDB := tempSessionDB(t)

	sid := "timestamp-test"
	_ = srcDB.EnsureSession(sid, "/tmp/ts")

	_ = srcDB.InsertEvent(sid, model.SessionEvent{
		Type: "file_read", Category: "file", Data: "ts-file.go", Priority: 1,
	}, "PostToolUse")

	exported, err := srcDB.ExportSession(sid)
	if err != nil {
		t.Fatalf("ExportSession: %v", err)
	}

	originalTS := exported.Events[0].CreatedAt

	if err := dstDB.ImportSession(exported); err != nil {
		t.Fatalf("ImportSession: %v", err)
	}

	dstEvents, err := dstDB.GetEvents(sid, nil)
	if err != nil {
		t.Fatalf("GetEvents: %v", err)
	}

	if dstEvents[0].CreatedAt != originalTS {
		t.Errorf("timestamp changed: %q -> %q", originalTS, dstEvents[0].CreatedAt)
	}
}
