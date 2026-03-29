package mcp

import (
	"context"
	"strings"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/inovacc/thimble/internal/model"
)

func TestHandleSessionInsights(t *testing.T) {
	db := newTestSession(t)

	b := &Bridge{
		session:    db,
		projectDir: "/tmp/test",
		stats:      newSessionStats(),
	}

	_ = db.EnsureSession("default", "/tmp/test")

	// Insert various events.
	_ = db.InsertEvent("default", model.SessionEvent{
		Type: "tool_call", Category: "tool",
		Data:     `{"tool":"ctx_search","session":"s1"}`,
		Priority: 2,
	}, "mcp")
	_ = db.InsertEvent("default", model.SessionEvent{
		Type: "tool_call", Category: "tool",
		Data:     `{"tool":"ctx_search","session":"s1","call":2}`,
		Priority: 2,
	}, "mcp")
	_ = db.InsertEvent("default", model.SessionEvent{
		Type: "tool_call", Category: "tool",
		Data:     `{"tool":"ctx_execute","session":"s1"}`,
		Priority: 2,
	}, "mcp")
	_ = db.InsertEvent("default", model.SessionEvent{
		Type: "error", Category: "error",
		Data:     "something failed",
		Priority: 3,
	}, "mcp")

	result, _, err := b.handleSessionInsights(context.Background(), &mcpsdk.CallToolRequest{}, sessionInsightsInput{})
	if err != nil {
		t.Fatalf("handleSessionInsights error: %v", err)
	}

	if result.IsError {
		t.Error("handleSessionInsights returned error result")
	}

	text := extractText(result)

	// Verify key sections are present.
	checks := []string{
		"Session Insights",
		"Total events:",
		"Events by Type",
		"Top Tools",
		"ctx_search",
		"ctx_execute",
		"Error count:",
	}

	for _, check := range checks {
		if !strings.Contains(text, check) {
			t.Errorf("output missing %q", check)
		}
	}
}

func TestHandleSessionInsightsNoSession(t *testing.T) {
	b := &Bridge{
		session: nil,
		stats:   newSessionStats(),
	}

	result, _, err := b.handleSessionInsights(context.Background(), &mcpsdk.CallToolRequest{}, sessionInsightsInput{})
	if err != nil {
		t.Fatalf("handleSessionInsights error: %v", err)
	}

	if !result.IsError {
		t.Error("expected error result when session is nil")
	}
}

func TestHandleSessionInsightsEmpty(t *testing.T) {
	db := newTestSession(t)

	b := &Bridge{
		session:    db,
		projectDir: "/tmp/test",
		stats:      newSessionStats(),
	}

	_ = db.EnsureSession("default", "/tmp/test")

	result, _, err := b.handleSessionInsights(context.Background(), &mcpsdk.CallToolRequest{}, sessionInsightsInput{})
	if err != nil {
		t.Fatalf("handleSessionInsights error: %v", err)
	}

	if result.IsError {
		t.Error("handleSessionInsights returned error for empty session")
	}

	text := extractText(result)
	if !strings.Contains(text, "Total events:** 0") {
		t.Errorf("expected total events 0 in output, got: %s", text)
	}
}

func TestHandleSessionInsightsCustomSessionID(t *testing.T) {
	db := newTestSession(t)

	b := &Bridge{
		session:    db,
		projectDir: "/tmp/test",
		stats:      newSessionStats(),
	}

	_ = db.EnsureSession("custom-session", "/tmp/test")
	_ = db.InsertEvent("custom-session", model.SessionEvent{
		Type: "file_read", Category: "file",
		Data: "a.go", Priority: 1,
	}, "")

	result, _, err := b.handleSessionInsights(context.Background(), &mcpsdk.CallToolRequest{}, sessionInsightsInput{
		SessionID: "custom-session",
	})
	if err != nil {
		t.Fatalf("handleSessionInsights error: %v", err)
	}

	if result.IsError {
		t.Error("handleSessionInsights returned error")
	}

	text := extractText(result)
	if !strings.Contains(text, "Total events:** 1") {
		t.Errorf("expected total events 1 in output, got: %s", text)
	}
}

func TestHandleSessionInsightsWithGoal(t *testing.T) {
	db := newTestSession(t)

	b := &Bridge{
		session:    db,
		projectDir: "/tmp/test",
		stats:      newSessionStats(),
	}

	_ = db.EnsureSession("default", "/tmp/test")

	b.SetActiveGoal("fix bug #42")

	result, _, err := b.handleSessionInsights(context.Background(), &mcpsdk.CallToolRequest{}, sessionInsightsInput{})
	if err != nil {
		t.Fatalf("handleSessionInsights error: %v", err)
	}

	text := extractText(result)
	if !strings.Contains(text, "fix bug #42") {
		t.Errorf("expected active goal in output, got: %s", text)
	}
}

// extractText pulls the text from a CallToolResult.
func extractText(r *mcpsdk.CallToolResult) string {
	if r == nil || len(r.Content) == 0 {
		return ""
	}

	if tc, ok := r.Content[0].(*mcpsdk.TextContent); ok {
		return tc.Text
	}

	return ""
}
