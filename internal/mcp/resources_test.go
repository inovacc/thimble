package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/inovacc/thimble/internal/executor"
	"github.com/inovacc/thimble/internal/model"
	"github.com/inovacc/thimble/internal/session"
	"github.com/inovacc/thimble/internal/store"
)

var resourceTestSeq atomic.Int64

// newTestBridgeForResources creates a Bridge with session DB for resource testing.
func newTestBridgeForResources(t *testing.T) *Bridge {
	t.Helper()

	seq := resourceTestSeq.Add(1)

	tmpDir := filepath.Join(os.TempDir(), fmt.Sprintf("thimble-res-%d-%d", os.Getpid(), seq))
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	cs, err := store.New(filepath.Join(tmpDir, "content.db"))
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}

	sessDB, err := session.New(filepath.Join(tmpDir, "session.db"))
	if err != nil {
		cs.Close()
		t.Fatalf("session.New: %v", err)
	}

	exec := executor.New(&executor.Options{})
	b := NewForTest(cs, sessDB, exec)
	b.projectDir = tmpDir
	b.sessionID = "test-session"

	t.Cleanup(func() {
		b.Close()

		_ = os.RemoveAll(tmpDir)
	})

	return b
}

// readReq creates a ReadResourceRequest with the given URI.
func readReq(uri string) *mcpsdk.ReadResourceRequest {
	return &mcpsdk.ReadResourceRequest{
		Params: &mcpsdk.ReadResourceParams{URI: uri},
	}
}

func TestSessionEventsResource_Empty(t *testing.T) {
	b := newTestBridgeForResources(t)

	result, err := b.handleSessionEventsResource(context.Background(), readReq("thimble://session/events"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Contents) != 1 {
		t.Fatalf("expected 1 content, got %d", len(result.Contents))
	}

	if result.Contents[0].MIMEType != "application/json" {
		t.Errorf("expected application/json, got %s", result.Contents[0].MIMEType)
	}

	// Empty session should return empty array.
	if result.Contents[0].Text != "[]" {
		t.Errorf("expected empty array, got %s", result.Contents[0].Text)
	}
}

func TestSessionEventsResource_WithEvents(t *testing.T) {
	b := newTestBridgeForResources(t)

	// Insert events into the session.
	_ = b.session.EnsureSession("test-session", b.projectDir)
	_ = b.session.InsertEvent("test-session", model.SessionEvent{
		Type:     "mcp_tool_call",
		Category: "ctx_search",
		Data:     `{"tool":"ctx_search"}`,
		Priority: 2,
	}, "PostToolUse")

	result, err := b.handleSessionEventsResource(context.Background(), readReq("thimble://session/events"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var events []model.StoredEvent
	if err := json.Unmarshal([]byte(result.Contents[0].Text), &events); err != nil {
		t.Fatalf("unmarshal events: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	if events[0].Type != "mcp_tool_call" {
		t.Errorf("expected type mcp_tool_call, got %s", events[0].Type)
	}
}

func TestSessionEventsResource_NilSession(t *testing.T) {
	b := newTestBridgeForResources(t)
	b.session = nil

	result, err := b.handleSessionEventsResource(context.Background(), readReq("thimble://session/events"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Contents[0].Text != "[]" {
		t.Errorf("nil session should return empty array, got %s", result.Contents[0].Text)
	}
}

func TestSessionSnapshotResource_Empty(t *testing.T) {
	b := newTestBridgeForResources(t)

	result, err := b.handleSessionSnapshotResource(context.Background(), readReq("thimble://session/snapshot"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Contents[0].Text != "{}" {
		t.Errorf("expected empty object, got %s", result.Contents[0].Text)
	}
}

func TestSessionSnapshotResource_WithSnapshot(t *testing.T) {
	b := newTestBridgeForResources(t)

	_ = b.session.EnsureSession("test-session", b.projectDir)
	_ = b.session.UpsertResume("test-session", "test snapshot content", 5)

	result, err := b.handleSessionSnapshotResource(context.Background(), readReq("thimble://session/snapshot"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var resume model.ResumeRow
	if err := json.Unmarshal([]byte(result.Contents[0].Text), &resume); err != nil {
		t.Fatalf("unmarshal resume: %v", err)
	}

	if resume.Snapshot != "test snapshot content" {
		t.Errorf("expected snapshot content, got %q", resume.Snapshot)
	}

	if resume.EventCount != 5 {
		t.Errorf("expected event_count 5, got %d", resume.EventCount)
	}
}

func TestSessionSnapshotResource_NilSession(t *testing.T) {
	b := newTestBridgeForResources(t)
	b.session = nil

	result, err := b.handleSessionSnapshotResource(context.Background(), readReq("thimble://session/snapshot"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Contents[0].Text != "{}" {
		t.Errorf("nil session should return empty object, got %s", result.Contents[0].Text)
	}
}

func TestAnalysisResource_GoFile(t *testing.T) {
	b := newTestBridgeForResources(t)

	// Write a simple Go file to analyze.
	goFile := filepath.Join(b.projectDir, "example.go")
	if err := os.WriteFile(goFile, []byte(`package main

func Hello() string {
	return "hello"
}
`), 0o644); err != nil {
		t.Fatalf("write go file: %v", err)
	}

	result, err := b.handleAnalysisResource(context.Background(), readReq("thimble://analysis/"+goFile))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result.Contents[0].Text, "Hello") {
		t.Errorf("analysis should contain Hello symbol, got %s", result.Contents[0].Text)
	}
}

func TestAnalysisResource_RelativePath(t *testing.T) {
	b := newTestBridgeForResources(t)

	// Write a Go file.
	goFile := filepath.Join(b.projectDir, "rel.go")
	if err := os.WriteFile(goFile, []byte(`package main

func World() {}
`), 0o644); err != nil {
		t.Fatalf("write go file: %v", err)
	}

	result, err := b.handleAnalysisResource(context.Background(), readReq("thimble://analysis/rel.go"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result.Contents[0].Text, "World") {
		t.Errorf("analysis should contain World symbol, got %s", result.Contents[0].Text)
	}
}

func TestAnalysisResource_UnsupportedFile(t *testing.T) {
	b := newTestBridgeForResources(t)

	txtFile := filepath.Join(b.projectDir, "readme.txt")
	if err := os.WriteFile(txtFile, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	_, err := b.handleAnalysisResource(context.Background(), readReq("thimble://analysis/"+txtFile))
	if err == nil {
		t.Fatal("expected error for unsupported file type")
	}
}

func TestAnalysisResource_EmptyPath(t *testing.T) {
	b := newTestBridgeForResources(t)

	_, err := b.handleAnalysisResource(context.Background(), readReq("thimble://analysis/"))
	if err == nil {
		t.Fatal("expected error for empty path")
	}
}

func TestAnalysisResource_InvalidPrefix(t *testing.T) {
	b := newTestBridgeForResources(t)

	_, err := b.handleAnalysisResource(context.Background(), readReq("thimble://other/something"))
	if err == nil {
		t.Fatal("expected error for invalid URI prefix")
	}
}

func TestPluginsResource_Empty(t *testing.T) {
	b := newTestBridgeForResources(t)
	b.projectDir = t.TempDir()

	result, err := b.handlePluginsResource(context.Background(), readReq("thimble://plugins"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var plugins []json.RawMessage
	if err := json.Unmarshal([]byte(result.Contents[0].Text), &plugins); err != nil {
		t.Fatalf("unmarshal plugins: %v", err)
	}

	if result.Contents[0].MIMEType != "application/json" {
		t.Errorf("expected application/json, got %s", result.Contents[0].MIMEType)
	}
}

func TestPluginsResource_ValidJSON(t *testing.T) {
	b := newTestBridgeForResources(t)

	result, err := b.handlePluginsResource(context.Background(), readReq("thimble://plugins"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !json.Valid([]byte(result.Contents[0].Text)) {
		t.Errorf("plugins response is not valid JSON: %s", result.Contents[0].Text)
	}
}
