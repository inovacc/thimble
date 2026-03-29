package mcp

import (
	"context"
	"log/slog"
	"sync"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// ProgressReporter reports progress for long-running tool operations.
// Implementations must be safe for concurrent use.
type ProgressReporter interface {
	// Report sends a progress update for the named tool.
	// progress is the current step (must increase), total is the expected
	// number of steps (0 if unknown), and message describes the current state.
	Report(ctx context.Context, toolName string, progress, total float64, message string)
}

// NoopReporter discards all progress reports. Used as the default.
type NoopReporter struct{}

// Report implements ProgressReporter.
func (NoopReporter) Report(context.Context, string, float64, float64, string) {}

// LogReporter logs progress updates via slog to stderr.
type LogReporter struct {
	Logger *slog.Logger
}

// Report implements ProgressReporter by logging at info level.
func (r *LogReporter) Report(_ context.Context, toolName string, progress, total float64, message string) {
	r.Logger.Info("tool progress",
		"tool", toolName,
		"progress", progress,
		"total", total,
		"message", message,
	)
}

// MCPReporter sends MCP notifications/progress to the connected client session.
// Falls back to logging if the session is unavailable or notification fails.
type MCPReporter struct {
	Logger *slog.Logger
}

// Report implements ProgressReporter by sending MCP progress notifications.
func (r *MCPReporter) Report(ctx context.Context, toolName string, progress, total float64, message string) {
	session, token := sessionAndToken(ctx)

	if session == nil || token == nil {
		// No active session or no progress token — fall back to logging.
		r.Logger.Debug("progress (no session)",
			"tool", toolName,
			"progress", progress,
			"total", total,
			"message", message,
		)

		return
	}

	params := &mcpsdk.ProgressNotificationParams{
		ProgressToken: token,
		Progress:      progress,
		Total:         total,
		Message:       message,
	}

	if err := session.NotifyProgress(ctx, params); err != nil {
		r.Logger.Debug("progress notification failed",
			"tool", toolName,
			"error", err,
		)
	}
}

// RecordingReporter captures progress reports for testing.
type RecordingReporter struct {
	mu      sync.Mutex
	Reports []ProgressReport
}

// ProgressReport is a single captured progress event.
type ProgressReport struct {
	ToolName string
	Progress float64
	Total    float64
	Message  string
}

// Report implements ProgressReporter by recording the event.
func (r *RecordingReporter) Report(_ context.Context, toolName string, progress, total float64, message string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.Reports = append(r.Reports, ProgressReport{
		ToolName: toolName,
		Progress: progress,
		Total:    total,
		Message:  message,
	})
}

// Len returns the number of recorded reports.
func (r *RecordingReporter) Len() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	return len(r.Reports)
}

// Reset clears recorded reports.
func (r *RecordingReporter) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.Reports = r.Reports[:0]
}

// progressKey is the context key type for passing session + token through handlers.
type progressKey struct{}

// progressCtx holds the server session and progress token extracted from a request.
type progressCtx struct {
	session *mcpsdk.ServerSession
	token   any
}

// withProgress stores the session and progress token in the context.
func withProgress(ctx context.Context, session *mcpsdk.ServerSession, token any) context.Context {
	return context.WithValue(ctx, progressKey{}, &progressCtx{
		session: session,
		token:   token,
	})
}

// sessionAndToken retrieves the session and progress token from the context.
func sessionAndToken(ctx context.Context) (*mcpsdk.ServerSession, any) {
	v, ok := ctx.Value(progressKey{}).(*progressCtx)
	if !ok || v == nil {
		return nil, nil
	}

	return v.session, v.token
}
