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
	"github.com/inovacc/thimble/internal/ghcli"
	"github.com/inovacc/thimble/internal/linter"
	"github.com/inovacc/thimble/internal/session"
	"github.com/inovacc/thimble/internal/store"
)

var handlerTestSeq atomic.Int64

// newTestBridgeWithStore creates a Bridge with real content store and session DB.
func newTestBridgeWithStore(t *testing.T) *Bridge {
	t.Helper()

	seq := handlerTestSeq.Add(1)

	tmpDir := filepath.Join(os.TempDir(), fmt.Sprintf("thimble-htest-%d-%d", os.Getpid(), seq))
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
	b.sessionID = "test-handler-session"

	t.Cleanup(func() {
		b.Close()

		_ = os.RemoveAll(tmpDir)
	})

	return b
}

// ── formatGhOutput tests ──

func TestFormatGhOutputTimedOut(t *testing.T) {
	got := formatGhOutput(&ghcli.ExecResult{TimedOut: true, Stdout: "ignored"})
	if got != "gh command timed out" {
		t.Errorf("expected timeout message, got %q", got)
	}
}

func TestFormatGhOutputStdoutOnly(t *testing.T) {
	result := formatGhOutput(&ghcli.ExecResult{Stdout: "some output"})
	if result != "some output" {
		t.Errorf("got %q, want %q", result, "some output")
	}
}

func TestFormatGhOutputStderrOnly(t *testing.T) {
	result := formatGhOutput(&ghcli.ExecResult{Stderr: "error output"})
	if result != "error output" {
		t.Errorf("got %q, want %q", result, "error output")
	}
}

func TestFormatGhOutputBothStreams(t *testing.T) {
	result := formatGhOutput(&ghcli.ExecResult{Stdout: "out", Stderr: "err"})
	if !strings.Contains(result, "out") || !strings.Contains(result, "err") {
		t.Errorf("expected both streams, got %q", result)
	}
}

func TestFormatGhOutputEmpty(t *testing.T) {
	result := formatGhOutput(&ghcli.ExecResult{})
	if result != "(no results)" {
		t.Errorf("expected '(no results)', got %q", result)
	}
}

func TestFormatGhOutputEmptyWithNonZeroExit(t *testing.T) {
	result := formatGhOutput(&ghcli.ExecResult{ExitCode: 1})
	if !strings.Contains(result, "exited with code 1") {
		t.Errorf("expected exit code message, got %q", result)
	}
}

// ── formatLintOutput tests ──

func TestFormatLintOutputSuccess(t *testing.T) {
	result := formatLintOutput(&linter.Result{Success: true})
	if result != "No lint issues found.\n" {
		t.Errorf("expected no issues message, got %q", result)
	}
}

func TestFormatLintOutputWithIssues(t *testing.T) {
	result := formatLintOutput(&linter.Result{
		Success:     false,
		TotalIssues: 2,
		Issues: []linter.Issue{
			{File: "main.go", Line: 10, Column: 5, Message: "unused var", Linter: "errcheck"},
			{File: "main.go", Line: 20, Column: 1, Message: "missing doc", Linter: "golint", SourceLine: "func Foo() {}"},
		},
	})
	if !strings.Contains(result, "2 issue(s)") {
		t.Errorf("expected '2 issue(s)', got %q", result)
	}

	if !strings.Contains(result, "unused var") {
		t.Errorf("expected issue message in output")
	}

	if !strings.Contains(result, "[func Foo() {}]") {
		t.Errorf("expected source line in output")
	}
}

func TestFormatLintOutputNoSourceLine(t *testing.T) {
	result := formatLintOutput(&linter.Result{
		Success:     false,
		TotalIssues: 1,
		Issues: []linter.Issue{
			{File: "a.go", Line: 1, Column: 1, Message: "msg", Linter: "govet"},
		},
	})
	if strings.Contains(result, "[") {
		t.Errorf("expected no source line brackets, got %q", result)
	}
}

// ── errResult tests (tools_gh.go) ──

func TestErrResult(t *testing.T) {
	r := errResult("something failed")
	if !r.IsError {
		t.Error("expected IsError = true")
	}

	tc := r.Content[0].(*mcpsdk.TextContent)
	if tc.Text != "something failed" {
		t.Errorf("text = %q, want %q", tc.Text, "something failed")
	}
}

// ── handleIndex tests ──

func TestHandleIndex(t *testing.T) {
	b := newTestBridgeWithStore(t)

	tests := []struct {
		name        string
		input       indexInput
		wantContain string
	}{
		{
			name:        "markdown_default",
			input:       indexInput{Content: "# Test\nHello world", Label: "test:md"},
			wantContain: "test:md",
		},
		{
			name:        "plain_text",
			input:       indexInput{Content: "plain text here", Label: "test:plain", ContentType: "plain"},
			wantContain: "test:plain",
		},
		{
			name:        "json_content",
			input:       indexInput{Content: `{"key":"value"}`, Label: "test:json", ContentType: "json"},
			wantContain: "test:json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _, err := b.handleIndex(context.Background(), nil, tt.input)
			if err != nil {
				t.Fatalf("handleIndex error: %v", err)
			}

			if result.IsError {
				t.Errorf("unexpected error result: %v", result.Content)
			}

			text := extractText(result)
			if !strings.Contains(text, tt.wantContain) {
				t.Errorf("output %q should contain %q", text, tt.wantContain)
			}
		})
	}
}

// ── handleStats tests ──

func TestHandleStats(t *testing.T) {
	b := newTestBridgeWithStore(t)

	// Index something to have non-zero stats.
	_, _ = b.content.Index("# Test content\nSome data", "stats-test")

	result, _, err := b.handleStats(context.Background(), nil, statsInput{})
	if err != nil {
		t.Fatalf("handleStats error: %v", err)
	}

	if result.IsError {
		t.Error("handleStats returned error result")
	}

	text := extractText(result)

	checks := []string{
		"## Knowledge Base",
		"Sources:",
		"Chunks:",
		"## Context Window Protection",
		"## Session",
		"```json",
	}
	for _, check := range checks {
		if !strings.Contains(text, check) {
			t.Errorf("output missing %q", check)
		}
	}
}

func TestHandleStatsWithToolCalls(t *testing.T) {
	b := newTestBridgeWithStore(t)

	// Populate stats to trigger Per-Tool Breakdown section.
	b.stats.mu.Lock()
	b.stats.calls["ctx_search"] = 5
	b.stats.bytesReturned["ctx_search"] = 2048
	b.stats.bytesIndexed = 4096
	b.stats.bytesSandboxed = 512
	b.stats.mu.Unlock()

	result, _, err := b.handleStats(context.Background(), nil, statsInput{})
	if err != nil {
		t.Fatalf("handleStats error: %v", err)
	}

	text := extractText(result)
	if !strings.Contains(text, "## Per-Tool Breakdown") {
		t.Error("expected Per-Tool Breakdown section")
	}

	if !strings.Contains(text, "ctx_search") {
		t.Error("expected ctx_search in breakdown")
	}
}

// ── handleDoctor tests ──

func TestHandleDoctor(t *testing.T) {
	b := newTestBridgeWithStore(t)

	result, _, err := b.handleDoctor(context.Background(), nil, doctorInput{})
	if err != nil {
		t.Fatalf("handleDoctor error: %v", err)
	}

	if result.IsError {
		t.Error("handleDoctor returned error result")
	}

	text := extractText(result)

	checks := []string{"status", "healthy", "server", "runtime", "throttle", "knowledge_base"}
	for _, check := range checks {
		if !strings.Contains(text, check) {
			t.Errorf("output missing %q", check)
		}
	}
}

// ── handleWorkspaceInfo tests ──

func TestHandleWorkspaceInfoNilWorkspace(t *testing.T) {
	b := &Bridge{
		projectDir: "/tmp/test",
		stats:      newSessionStats(),
	}

	// Set up minimal server for spanTool.
	b.server = mcpsdk.NewServer(
		&mcpsdk.Implementation{Name: "test", Version: "0.0.0"},
		nil,
	)

	result, _, err := b.handleWorkspaceInfo(context.Background(), nil, workspaceInfoInput{})
	if err != nil {
		t.Fatalf("handleWorkspaceInfo error: %v", err)
	}

	if result.IsError {
		t.Error("handleWorkspaceInfo returned error result")
	}

	text := extractText(result)
	if !strings.Contains(text, "single") {
		t.Errorf("expected workspace type 'single', got %q", text)
	}
}

func TestHandleWorkspaceInfoWithWorkspace(t *testing.T) {
	b := newTestBridgeWithStore(t)
	b.workspace = &session.Workspace{
		RootDir:  "/test/root",
		Projects: []string{"/test/root/a", "/test/root/b"},
		Type:     session.WorkspaceSingle,
	}

	result, _, err := b.handleWorkspaceInfo(context.Background(), nil, workspaceInfoInput{})
	if err != nil {
		t.Fatalf("handleWorkspaceInfo error: %v", err)
	}

	text := extractText(result)
	if !strings.Contains(text, "/test/root") {
		t.Errorf("expected root dir in output, got %q", text)
	}
}

// ── autoIndex tests ──

func TestAutoIndexEmptyContent(t *testing.T) {
	b := newTestBridgeWithStore(t)
	// Should not panic with empty content.
	b.autoIndex(context.Background(), "", "test-label")
}

func TestAutoIndexNilStore(t *testing.T) {
	b := &Bridge{stats: newSessionStats()}
	// Should not panic with nil content store.
	b.autoIndex(context.Background(), "some content", "test-label")
}

func TestAutoIndexWithContent(t *testing.T) {
	b := newTestBridgeWithStore(t)
	b.autoIndex(context.Background(), "# Hello\nSome indexed data", "auto-test")

	// Verify it was indexed by searching for it.
	results, err := b.content.SearchWithFallback("indexed data", 1, "")
	if err != nil {
		t.Fatalf("search error: %v", err)
	}

	if len(results) == 0 {
		t.Error("expected auto-indexed content to be searchable")
	}
}

// ── checkGitDeny tests ──

func TestCheckGitDenyAllowed(t *testing.T) {
	b := &Bridge{projectDir: t.TempDir()}
	if err := b.checkGitDeny("status"); err != nil {
		t.Errorf("expected git status to be allowed, got: %v", err)
	}
}

// ── intentPreview tests ──

func TestIntentPreviewNoResults(t *testing.T) {
	b := newTestBridgeWithStore(t)

	// Index something that won't match the intent.
	_, _ = b.content.IndexPlainText("unrelated data about cats", "test:cats", 20)

	result := b.intentPreview("kubernetes deployment", "test:cats", strings.Repeat("x", 6000), 0)
	if result == "" {
		t.Error("expected non-empty preview even with no matches")
	}

	if !strings.Contains(result, "test:cats") {
		t.Errorf("expected label in preview, got %q", result)
	}
}

func TestIntentPreviewWithResults(t *testing.T) {
	b := newTestBridgeWithStore(t)

	content := "# Deployment Guide\n\nUse kubectl apply to deploy to kubernetes cluster.\n"
	_, _ = b.content.Index(content, "test:k8s")

	result := b.intentPreview("kubernetes deployment", "test:k8s", strings.Repeat("x", 6000), 0)
	if result == "" {
		t.Error("expected non-empty preview")
	}

	if !strings.Contains(result, "test:k8s") {
		t.Errorf("expected label in preview, got %q", result)
	}
}

// ── rateLimitResult tests ──

func TestRateLimitResult(t *testing.T) {
	r := rateLimitResult("ctx_search")
	if !r.IsError {
		t.Error("expected IsError = true")
	}

	tc := r.Content[0].(*mcpsdk.TextContent)
	if !strings.Contains(tc.Text, "ctx_search") {
		t.Error("expected tool name in message")
	}

	if !strings.Contains(tc.Text, "rate limit") {
		t.Error("expected 'rate limit' in message")
	}
}

// ── withRateLimit tests ──

func TestWithRateLimitAllowed(t *testing.T) {
	b := newTestBridgeWithStore(t)
	// No rate limiter set — should always pass through.

	called := false
	handler := func(_ context.Context, _ *mcpsdk.CallToolRequest, _ statsInput) (*mcpsdk.CallToolResult, struct{}, error) {
		called = true
		return textResult("ok"), struct{}{}, nil
	}

	wrapped := withRateLimit(b, "ctx_stats", handler)

	result, _, err := wrapped(context.Background(), nil, statsInput{})
	if err != nil {
		t.Fatalf("withRateLimit error: %v", err)
	}

	if !called {
		t.Error("handler was not called")
	}

	if result.IsError {
		t.Error("unexpected error result")
	}
}

func TestWithRateLimitDenied(t *testing.T) {
	b := newTestBridgeWithStore(t)
	// Set a very restrictive rate limiter.
	b.rateLimiter = NewRateLimiter(0.001, 1)

	handler := func(_ context.Context, _ *mcpsdk.CallToolRequest, _ statsInput) (*mcpsdk.CallToolResult, struct{}, error) {
		return textResult("ok"), struct{}{}, nil
	}

	wrapped := withRateLimit(b, "ctx_stats", handler)

	// First call should succeed.
	result1, _, _ := wrapped(context.Background(), nil, statsInput{})
	if result1.IsError {
		t.Error("first call should succeed")
	}

	// Second call should be rate limited.
	result2, _, _ := wrapped(context.Background(), nil, statsInput{})
	if !result2.IsError {
		t.Error("second call should be rate limited")
	}
}

// ── filterResult large content tests ──

func TestFilterResultSmallContent(t *testing.T) {
	r := filterResult("small content")

	tc := r.Content[0].(*mcpsdk.TextContent)
	if tc.Text != "small content" {
		t.Errorf("small content should pass through unchanged, got %q", tc.Text)
	}
}

func TestFilterResultLargeContent(t *testing.T) {
	// Create content larger than FilterThreshold (5KB).
	large := strings.Repeat("line of text\n", 500)
	r := filterResult(large)

	tc := r.Content[0].(*mcpsdk.TextContent)
	if len(tc.Text) >= len(large) {
		t.Error("large content should be truncated")
	}

	if !strings.Contains(tc.Text, "truncated") {
		t.Error("expected 'truncated' marker in output")
	}
}

// ── progressCtxFromRequest tests ──

func TestProgressCtxFromRequestNilReq(t *testing.T) {
	ctx := context.Background()

	result := progressCtxFromRequest(ctx, nil)
	if result != ctx {
		t.Error("nil request should return original context")
	}
}

func TestProgressCtxFromRequestNilSession(t *testing.T) {
	ctx := context.Background()
	req := &mcpsdk.CallToolRequest{}

	result := progressCtxFromRequest(ctx, req)
	if result != ctx {
		t.Error("nil session should return original context")
	}
}

// ── handleReportShow validation tests ──

func TestHandleReportShowEmptyID(t *testing.T) {
	b := &Bridge{stats: newSessionStats()}

	result, _, err := b.handleReportShow(context.Background(), nil, reportShowInput{ID: ""})
	if err != nil {
		t.Fatalf("handleReportShow error: %v", err)
	}

	if !result.IsError {
		t.Error("expected error for empty ID")
	}

	text := extractText(result)
	if !strings.Contains(text, "id is required") {
		t.Errorf("expected 'id is required', got %q", text)
	}
}

// ── handleReportDelete validation tests ──

func TestHandleReportDeleteEmptyID(t *testing.T) {
	b := &Bridge{stats: newSessionStats()}

	result, _, err := b.handleReportDelete(context.Background(), nil, reportDeleteInput{ID: ""})
	if err != nil {
		t.Fatalf("handleReportDelete error: %v", err)
	}

	if !result.IsError {
		t.Error("expected error for empty ID")
	}

	text := extractText(result)
	if !strings.Contains(text, "id is required") {
		t.Errorf("expected 'id is required', got %q", text)
	}
}

func TestHandleReportDeleteNonexistent(t *testing.T) {
	b := &Bridge{stats: newSessionStats()}

	result, _, err := b.handleReportDelete(context.Background(), nil, reportDeleteInput{ID: "nonexistent-uuid"})
	if err != nil {
		t.Fatalf("handleReportDelete error: %v", err)
	}
	// Should return an error since the report doesn't exist.
	if !result.IsError {
		t.Error("expected error for nonexistent report ID")
	}
}

func TestHandleReportShowNonexistent(t *testing.T) {
	b := &Bridge{stats: newSessionStats()}

	result, _, err := b.handleReportShow(context.Background(), nil, reportShowInput{ID: "nonexistent-uuid"})
	if err != nil {
		t.Fatalf("handleReportShow error: %v", err)
	}

	if !result.IsError {
		t.Error("expected error for nonexistent report ID")
	}
}

// ── handleReportList tests ──

func TestHandleReportListNoFilter(t *testing.T) {
	b := &Bridge{stats: newSessionStats()}

	result, _, err := b.handleReportList(context.Background(), nil, reportListInput{})
	if err != nil {
		t.Fatalf("handleReportList error: %v", err)
	}
	// Should succeed (may return "No reports found." or a list).
	if result.IsError {
		t.Error("handleReportList should not return error")
	}
}

func TestHandleReportListWithFilter(t *testing.T) {
	b := &Bridge{stats: newSessionStats()}

	result, _, err := b.handleReportList(context.Background(), nil, reportListInput{TypeFilter: "doctor"})
	if err != nil {
		t.Fatalf("handleReportList error: %v", err)
	}

	if result.IsError {
		t.Error("handleReportList should not return error")
	}
}

// ── handleSearch tests ──

func TestHandleSearchEmptyStore(t *testing.T) {
	b := newTestBridgeWithStore(t)

	result, _, err := b.handleSearch(context.Background(), nil, searchInput{Query: "nonexistent"})
	if err != nil {
		t.Fatalf("handleSearch error: %v", err)
	}

	if result.IsError {
		t.Error("handleSearch should not return error for empty results")
	}

	text := extractText(result)
	if !strings.Contains(text, "0 results") {
		t.Errorf("expected '0 results' message, got %q", text)
	}
}

func TestHandleSearchWithContent(t *testing.T) {
	b := newTestBridgeWithStore(t)

	// Index content first.
	_, _ = b.content.Index("# Docker\n\nUse docker compose for local development.", "test:docker")

	result, _, err := b.handleSearch(context.Background(), nil, searchInput{Query: "docker compose"})
	if err != nil {
		t.Fatalf("handleSearch error: %v", err)
	}

	text := extractText(result)
	if !strings.Contains(text, "docker") {
		t.Errorf("expected docker in results, got %q", text)
	}
}

func TestHandleSearchMultipleQueries(t *testing.T) {
	b := newTestBridgeWithStore(t)

	_, _ = b.content.Index("# Testing\n\nUse table-driven tests in Go.", "test:testing")

	result, _, err := b.handleSearch(context.Background(), nil, searchInput{
		Queries: []string{"table-driven tests", "Go testing"},
	})
	if err != nil {
		t.Fatalf("handleSearch error: %v", err)
	}

	text := extractText(result)
	if !strings.Contains(text, "Query 1") {
		t.Error("expected Query 1 header")
	}

	if !strings.Contains(text, "Query 2") {
		t.Error("expected Query 2 header")
	}
}

// ── RateLimiter edge cases ──

func TestRateLimiterNilAllow(t *testing.T) {
	var rl *RateLimiter
	if !rl.Allow("test") {
		t.Error("nil rate limiter should always allow")
	}

	if !rl.AllowGlobal() {
		t.Error("nil rate limiter AllowGlobal should always allow")
	}
}

func TestRateLimiterDisabledRate(t *testing.T) {
	rl := NewRateLimiter(0, 5)
	if !rl.Allow("test") {
		t.Error("zero rate limiter should always allow")
	}

	if !rl.AllowGlobal() {
		t.Error("zero rate AllowGlobal should always allow")
	}
}

func TestRateLimiterMinBurst(t *testing.T) {
	rl := NewRateLimiter(1.0, 0) // burst < 1 should be clamped to 1
	if rl.burst != 1 {
		t.Errorf("burst = %d, want 1", rl.burst)
	}
}

// ── SetProgressReporter tests ──

func TestSetProgressReporterNil(t *testing.T) {
	b := &Bridge{progress: NoopReporter{}}
	b.SetProgressReporter(nil)
	// Should set to NoopReporter, not nil.
	if b.progress == nil {
		t.Error("progress should never be nil after SetProgressReporter(nil)")
	}
}

func TestSetProgressReporterCustom(t *testing.T) {
	b := &Bridge{progress: NoopReporter{}}
	b.SetProgressReporter(NoopReporter{})

	if b.progress == nil {
		t.Error("progress should not be nil")
	}
}

// ── getSharedStore tests ──

func TestGetSharedStorePreInjected(t *testing.T) {
	b := newTestBridgeWithStore(t)

	seq := handlerTestSeq.Add(1)

	tmpDir := filepath.Join(os.TempDir(), fmt.Sprintf("thimble-shared-get-%d-%d", os.Getpid(), seq))
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	shared, err := store.New(filepath.Join(tmpDir, "shared.db"))
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}

	t.Cleanup(func() { shared.Close() })

	b.sharedStore = shared

	got, err := b.getSharedStore()
	if err != nil {
		t.Fatalf("getSharedStore error: %v", err)
	}

	if got != shared {
		t.Error("expected pre-injected shared store")
	}
}

// ── Close comprehensive test ──

func TestCloseWithAllFields(t *testing.T) {
	b := newTestBridgeWithStore(t)
	// Close should not panic even when called multiple times.
	b.Close()
}

// ── handleStatsWithSessionContinuity tests ──

func TestHandleStatsWithSessionContinuity(t *testing.T) {
	b := newTestBridgeWithStore(t)

	// Ensure session exists and record some events.
	_ = b.session.EnsureSession(b.sessionID, b.projectDir)
	b.recordToolCall(context.Background(), "ctx_search", true)
	b.recordToolCall(context.Background(), "ctx_execute", false)

	result, _, err := b.handleStats(context.Background(), nil, statsInput{})
	if err != nil {
		t.Fatalf("handleStats error: %v", err)
	}

	text := extractText(result)
	// Stats should contain knowledge base or session information.
	if text == "" {
		t.Error("expected non-empty stats output")
	}
}

// ── SearchTotalOutputCap constant test ──

func TestSearchTotalOutputCapConstant(t *testing.T) {
	if SearchTotalOutputCap != 40*1024 {
		t.Errorf("SearchTotalOutputCap = %d, want %d", SearchTotalOutputCap, 40*1024)
	}
}

func TestBatchTimeoutMsConstant(t *testing.T) {
	if BatchTimeoutMs != 120_000 {
		t.Errorf("BatchTimeoutMs = %d, want 120000", BatchTimeoutMs)
	}
}
