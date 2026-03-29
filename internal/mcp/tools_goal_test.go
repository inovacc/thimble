package mcp

import (
	"context"
	"path/filepath"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/inovacc/thimble/internal/session"
)

func newTestSession(t *testing.T) *session.SessionDB {
	t.Helper()

	db, err := session.New(filepath.Join(t.TempDir(), "session.db"))
	if err != nil {
		t.Fatalf("create session db: %v", err)
	}

	t.Cleanup(func() { db.Close() })

	return db
}

func TestActiveGoalGetSet(t *testing.T) {
	b := &Bridge{}

	if got := b.ActiveGoal(); got != "" {
		t.Errorf("ActiveGoal() on new bridge = %q, want empty", got)
	}

	b.SetActiveGoal("implement auth")

	if got := b.ActiveGoal(); got != "implement auth" {
		t.Errorf("ActiveGoal() = %q, want %q", got, "implement auth")
	}

	b.SetActiveGoal("")

	if got := b.ActiveGoal(); got != "" {
		t.Errorf("ActiveGoal() after clear = %q, want empty", got)
	}
}

func TestHandleSetGoal(t *testing.T) {
	db := newTestSession(t)

	b := &Bridge{
		session:    db,
		projectDir: "/tmp/test",
		stats:      newSessionStats(),
	}

	result, _, err := b.handleSetGoal(context.Background(), &mcpsdk.CallToolRequest{}, setGoalInput{
		Goal: "implement auth module",
	})
	if err != nil {
		t.Fatalf("handleSetGoal error: %v", err)
	}

	if result.IsError {
		t.Error("handleSetGoal returned error result")
	}

	if got := b.ActiveGoal(); got != "implement auth module" {
		t.Errorf("ActiveGoal() = %q, want %q", got, "implement auth module")
	}

	// Verify event was recorded.
	events, err := db.GetEvents("default", &session.GetEventsOpts{Type: "goal_set"})
	if err != nil {
		t.Fatalf("GetEvents error: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("expected 1 goal_set event, got %d", len(events))
	}

	if events[0].Category != "goal" {
		t.Errorf("event category = %q, want %q", events[0].Category, "goal")
	}
}

func TestHandleSetGoalEmpty(t *testing.T) {
	b := &Bridge{
		stats: newSessionStats(),
	}

	result, _, err := b.handleSetGoal(context.Background(), &mcpsdk.CallToolRequest{}, setGoalInput{
		Goal: "   ",
	})
	if err != nil {
		t.Fatalf("handleSetGoal error: %v", err)
	}

	if !result.IsError {
		t.Error("expected error result for empty goal")
	}
}

func TestHandleClearGoal(t *testing.T) {
	db := newTestSession(t)

	b := &Bridge{
		session:    db,
		projectDir: "/tmp/test",
		stats:      newSessionStats(),
	}

	b.SetActiveGoal("some goal")

	result, _, err := b.handleClearGoal(context.Background(), &mcpsdk.CallToolRequest{}, clearGoalInput{})
	if err != nil {
		t.Fatalf("handleClearGoal error: %v", err)
	}

	if result.IsError {
		t.Error("handleClearGoal returned error result")
	}

	if got := b.ActiveGoal(); got != "" {
		t.Errorf("ActiveGoal() after clear = %q, want empty", got)
	}

	// Verify event was recorded.
	events, err := db.GetEvents("default", &session.GetEventsOpts{Type: "goal_cleared"})
	if err != nil {
		t.Fatalf("GetEvents error: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("expected 1 goal_cleared event, got %d", len(events))
	}
}

func TestHandleClearGoalWhenNone(t *testing.T) {
	b := &Bridge{
		stats: newSessionStats(),
	}

	result, _, err := b.handleClearGoal(context.Background(), &mcpsdk.CallToolRequest{}, clearGoalInput{})
	if err != nil {
		t.Fatalf("handleClearGoal error: %v", err)
	}

	if result.IsError {
		t.Error("expected non-error result when no goal to clear")
	}
}

func TestRecordToolCallWithGoal(t *testing.T) {
	db := newTestSession(t)

	b := &Bridge{
		session:    db,
		projectDir: "/tmp/test",
		sessionID:  "test-123",
		stats:      newSessionStats(),
	}

	// Record without goal.
	b.recordToolCall(context.Background(), "ctx_search", true)

	// Set goal and record.
	b.SetActiveGoal("fix bug #42")
	b.recordToolCall(context.Background(), "ctx_execute", false)

	events, err := db.GetEvents("default", nil)
	if err != nil {
		t.Fatalf("GetEvents error: %v", err)
	}

	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}

	// First event (no goal) should not contain "goal".
	if containsGoal(events[0].Data) {
		t.Error("first event should not contain goal tag")
	}

	// Second event (with goal) should contain "goal".
	if !containsGoal(events[1].Data) {
		t.Error("second event should contain goal tag")
	}
}

func containsGoal(data string) bool {
	return len(data) > 0 && contains(data, `"goal"`)
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}

	return false
}
