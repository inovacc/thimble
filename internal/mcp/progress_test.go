package mcp

import (
	"context"
	"log/slog"
	"os"
	"testing"
)

func TestNoopReporter(t *testing.T) {
	r := NoopReporter{}

	// Should not panic.
	r.Report(context.Background(), "test_tool", 1, 2, "halfway")
}

func TestLogReporter(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelError, // suppress output in tests
	}))

	r := &LogReporter{Logger: logger}

	// Should not panic.
	r.Report(context.Background(), "test_tool", 0, 3, "starting")
	r.Report(context.Background(), "test_tool", 1, 3, "step 1")
	r.Report(context.Background(), "test_tool", 3, 3, "done")
}

func TestRecordingReporter(t *testing.T) {
	r := &RecordingReporter{}

	ctx := context.Background()

	r.Report(ctx, "tool_a", 0, 2, "start")
	r.Report(ctx, "tool_a", 1, 2, "middle")
	r.Report(ctx, "tool_a", 2, 2, "done")

	if r.Len() != 3 {
		t.Fatalf("Len() = %d, want 3", r.Len())
	}

	// Verify first report.
	got := r.Reports[0]
	if got.ToolName != "tool_a" {
		t.Errorf("Reports[0].ToolName = %q, want %q", got.ToolName, "tool_a")
	}

	if got.Progress != 0 {
		t.Errorf("Reports[0].Progress = %f, want 0", got.Progress)
	}

	if got.Total != 2 {
		t.Errorf("Reports[0].Total = %f, want 2", got.Total)
	}

	if got.Message != "start" {
		t.Errorf("Reports[0].Message = %q, want %q", got.Message, "start")
	}

	// Verify last report.
	last := r.Reports[2]
	if last.Progress != 2 || last.Message != "done" {
		t.Errorf("Reports[2] = {Progress:%f Message:%q}, want {Progress:2 Message:done}", last.Progress, last.Message)
	}

	// Reset.
	r.Reset()

	if r.Len() != 0 {
		t.Errorf("Len() after Reset() = %d, want 0", r.Len())
	}
}

func TestMCPReporterNoSession(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	r := &MCPReporter{Logger: logger}

	// No session in context — should not panic, falls back to debug log.
	r.Report(context.Background(), "test_tool", 1, 2, "progress")
}

func TestWithProgressAndSessionAndToken(t *testing.T) {
	ctx := context.Background()

	// No progress in context.
	session, token := sessionAndToken(ctx)
	if session != nil || token != nil {
		t.Error("expected nil session and token from empty context")
	}

	// With progress context but nil session — via withProgress directly.
	ctx2 := withProgress(ctx, nil, nil)

	session2, token2 := sessionAndToken(ctx2)
	if session2 != nil || token2 != nil {
		t.Error("expected nil session and token with nil session in context")
	}
}

func TestProgressCtxFromRequestNil(t *testing.T) {
	ctx := context.Background()

	// nil request should return context unchanged.
	ctx2 := progressCtxFromRequest(ctx, nil)

	session, token := sessionAndToken(ctx2)
	if session != nil || token != nil {
		t.Error("expected nil session and token from nil request")
	}
}

func TestSetProgressReporter(t *testing.T) {
	b := &Bridge{progress: NoopReporter{}}

	rec := &RecordingReporter{}
	b.SetProgressReporter(rec)

	// Verify reporter was set.
	b.progress.Report(context.Background(), "test", 1, 1, "ok")

	if rec.Len() != 1 {
		t.Fatalf("expected 1 report after SetProgressReporter, got %d", rec.Len())
	}

	// Setting nil should default to noop.
	b.SetProgressReporter(nil)

	// Should not panic.
	b.progress.Report(context.Background(), "test", 1, 1, "ok")
}

func TestRecordingReporterConcurrency(t *testing.T) {
	r := &RecordingReporter{}

	ctx := context.Background()
	done := make(chan struct{})

	// Run multiple goroutines reporting concurrently.
	for range 10 {
		go func() {
			for j := range 100 {
				r.Report(ctx, "concurrent_tool", float64(j), 100, "step")
			}

			done <- struct{}{}
		}()
	}

	for range 10 {
		<-done
	}

	if r.Len() != 1000 {
		t.Errorf("Len() = %d, want 1000", r.Len())
	}
}
