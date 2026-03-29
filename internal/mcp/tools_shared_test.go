package mcp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/inovacc/thimble/internal/executor"
	"github.com/inovacc/thimble/internal/session"
	"github.com/inovacc/thimble/internal/store"
)

var sharedTestSeq atomic.Int64

// newTestBridgeWithShared creates a Bridge with a pre-initialized shared store for testing.
// Uses a manual temp dir instead of t.TempDir() to avoid Windows file-lock cleanup failures
// with SQLite WAL mode.
func newTestBridgeWithShared(t *testing.T) *Bridge {
	t.Helper()

	seq := sharedTestSeq.Add(1)

	tmpDir := filepath.Join(os.TempDir(), fmt.Sprintf("thimble-shared-%d-%d", os.Getpid(), seq))
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

	// Pre-initialize shared store with a temp path.
	shared, err := store.New(filepath.Join(tmpDir, "shared_content.db"))
	if err != nil {
		t.Fatalf("shared store.New: %v", err)
	}

	b.sharedStore = shared

	t.Cleanup(func() {
		b.Close()

		_ = os.RemoveAll(tmpDir)
	})

	return b
}

func TestHandleSharedIndex(t *testing.T) {
	b := newTestBridgeWithShared(t)

	input := sharedIndexInput{
		Content: "# Cross-Project Pattern\n\nAlways use structured logging with slog.",
		Label:   "shared:conventions",
	}

	result, _, err := b.handleSharedIndex(context.Background(), nil, input)
	if err != nil {
		t.Fatalf("handleSharedIndex error: %v", err)
	}

	if result.IsError {
		t.Fatalf("expected success, got error: %v", result.Content)
	}

	text := result.Content[0].(*mcpsdk.TextContent).Text
	if !strings.Contains(text, "shared:conventions") {
		t.Errorf("result text %q should contain label", text)
	}
}

func TestHandleSharedSearch(t *testing.T) {
	b := newTestBridgeWithShared(t)

	// First index something.
	indexInput := sharedIndexInput{
		Content: "# Deployment\n\nUse blue-green deployments for zero-downtime releases.",
		Label:   "shared:deploy",
	}

	_, _, err := b.handleSharedIndex(context.Background(), nil, indexInput)
	if err != nil {
		t.Fatalf("handleSharedIndex error: %v", err)
	}

	// Now search for it.
	searchIn := sharedSearchInput{
		Query: "blue-green deployment",
	}

	result, _, err := b.handleSharedSearch(context.Background(), nil, searchIn)
	if err != nil {
		t.Fatalf("handleSharedSearch error: %v", err)
	}

	if result.IsError {
		t.Fatalf("expected success, got error: %v", result.Content)
	}

	text := result.Content[0].(*mcpsdk.TextContent).Text
	if !strings.Contains(text, "blue-green") {
		t.Errorf("search result %q should contain indexed content", text)
	}
}

func TestHandleSharedSearchMultipleQueries(t *testing.T) {
	b := newTestBridgeWithShared(t)

	// Index content.
	if _, _, err := b.handleSharedIndex(context.Background(), nil, sharedIndexInput{
		Content: "# API Design\n\nUse RESTful conventions with proper HTTP status codes.",
		Label:   "shared:api",
	}); err != nil {
		t.Fatalf("handleSharedIndex error: %v", err)
	}

	// Search with multiple queries.
	searchIn := sharedSearchInput{
		Queries: []string{"REST API", "HTTP status"},
	}

	result, _, err := b.handleSharedSearch(context.Background(), nil, searchIn)
	if err != nil {
		t.Fatalf("handleSharedSearch error: %v", err)
	}

	text := result.Content[0].(*mcpsdk.TextContent).Text
	if !strings.Contains(text, "Query 1") {
		t.Error("expected Query 1 header")
	}

	if !strings.Contains(text, "Query 2") {
		t.Error("expected Query 2 header")
	}
}

func TestHandleSharedSearchEmptyStore(t *testing.T) {
	b := newTestBridgeWithShared(t)

	searchIn := sharedSearchInput{
		Query: "nonexistent topic",
	}

	result, _, err := b.handleSharedSearch(context.Background(), nil, searchIn)
	if err != nil {
		t.Fatalf("handleSharedSearch error: %v", err)
	}

	text := result.Content[0].(*mcpsdk.TextContent).Text
	if !strings.Contains(text, "0 results") {
		t.Errorf("expected '0 results' message, got %q", text)
	}
}

func TestHandleSharedList(t *testing.T) {
	b := newTestBridgeWithShared(t)

	// Empty list first.
	result, _, err := b.handleSharedList(context.Background(), nil, sharedListInput{})
	if err != nil {
		t.Fatalf("handleSharedList error: %v", err)
	}

	text := result.Content[0].(*mcpsdk.TextContent).Text
	if !strings.Contains(text, "empty") {
		t.Errorf("expected empty message, got %q", text)
	}

	// Index some content.
	if _, _, err := b.handleSharedIndex(context.Background(), nil, sharedIndexInput{
		Content: "# Testing\n\nUse table-driven tests in Go.",
		Label:   "shared:testing",
	}); err != nil {
		t.Fatalf("handleSharedIndex error: %v", err)
	}

	// List should now show the source.
	result, _, err = b.handleSharedList(context.Background(), nil, sharedListInput{})
	if err != nil {
		t.Fatalf("handleSharedList error: %v", err)
	}

	text = result.Content[0].(*mcpsdk.TextContent).Text
	if !strings.Contains(text, "shared:testing") {
		t.Errorf("expected source label in list, got %q", text)
	}

	if !strings.Contains(text, "Sources:") {
		t.Error("expected stats header in list output")
	}
}

func TestHandleSharedIndexPlainText(t *testing.T) {
	b := newTestBridgeWithShared(t)

	input := sharedIndexInput{
		Content:     "Line 1\nLine 2\nLine 3",
		Label:       "shared:plain",
		ContentType: "plain",
	}

	result, _, err := b.handleSharedIndex(context.Background(), nil, input)
	if err != nil {
		t.Fatalf("handleSharedIndex error: %v", err)
	}

	if result.IsError {
		t.Fatalf("expected success, got error: %v", result.Content)
	}
}

func TestHandleSharedIndexJSON(t *testing.T) {
	b := newTestBridgeWithShared(t)

	input := sharedIndexInput{
		Content:     `{"key": "value", "nested": {"a": 1}}`,
		Label:       "shared:json",
		ContentType: "json",
	}

	result, _, err := b.handleSharedIndex(context.Background(), nil, input)
	if err != nil {
		t.Fatalf("handleSharedIndex error: %v", err)
	}

	if result.IsError {
		t.Fatalf("expected success, got error: %v", result.Content)
	}
}
