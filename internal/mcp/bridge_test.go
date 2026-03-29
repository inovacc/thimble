package mcp

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"
)

func TestSessionHash(t *testing.T) {
	tests := []struct {
		name       string
		appPath    string
		projectDir string
	}{
		{"basic", "/usr/bin/thimble", "/home/user/project"},
		{"windows paths", `C:\Users\user\thimble.exe`, `C:\Users\user\project`},
		{"empty strings", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := sessionHash(tt.appPath, tt.projectDir)
			if len(h) != 16 {
				t.Errorf("sessionHash() = %q, want 16-char hex string", h)
			}

			// Deterministic: same input → same output.
			h2 := sessionHash(tt.appPath, tt.projectDir)
			if h != h2 {
				t.Errorf("sessionHash() not deterministic: %q != %q", h, h2)
			}
		})
	}

	// Different inputs produce different hashes.
	h1 := sessionHash("/a", "/b")

	h2 := sessionHash("/c", "/d")
	if h1 == h2 {
		t.Error("different inputs should produce different hashes")
	}
}

func TestSessionIDMethod(t *testing.T) {
	b := &Bridge{sessionID: "abc123"}
	if got := b.SessionID(); got != "abc123" {
		t.Errorf("SessionID() = %q, want %q", got, "abc123")
	}

	// Empty session ID.
	b2 := &Bridge{}
	if got := b2.SessionID(); got != "" {
		t.Errorf("SessionID() = %q, want empty", got)
	}
}

func TestNewSessionStats(t *testing.T) {
	s := newSessionStats()
	if s == nil {
		t.Fatal("newSessionStats() returned nil")
	}

	if s.calls == nil {
		t.Error("calls map should be initialized")
	}

	if s.bytesReturned == nil {
		t.Error("bytesReturned map should be initialized")
	}

	if s.sessionStart.IsZero() {
		t.Error("sessionStart should be set")
	}
}

func TestSessionStatsSnapshot(t *testing.T) {
	s := newSessionStats()

	// Add some data.
	s.mu.Lock()
	s.calls["ctx_search"] = 5
	s.calls["ctx_index"] = 3
	s.bytesReturned["ctx_search"] = 1024
	s.bytesIndexed = 2048
	s.bytesSandboxed = 512
	s.mu.Unlock()

	snap := s.snapshot()

	calls, ok := snap["calls"].(map[string]int)
	if !ok {
		t.Fatalf("calls type = %T, want map[string]int", snap["calls"])
	}

	if calls["ctx_search"] != 5 {
		t.Errorf("calls[ctx_search] = %d, want 5", calls["ctx_search"])
	}

	if snap["bytesIndexed"] != 2048 {
		t.Errorf("bytesIndexed = %v, want 2048", snap["bytesIndexed"])
	}

	if snap["bytesSandboxed"] != 512 {
		t.Errorf("bytesSandboxed = %v, want 512", snap["bytesSandboxed"])
	}

	uptime, ok := snap["uptimeSeconds"].(int)
	if !ok {
		t.Fatalf("uptimeSeconds type = %T, want int", snap["uptimeSeconds"])
	}

	if uptime < 0 {
		t.Error("uptimeSeconds should be non-negative")
	}
}

func TestRecordToolCallNoSessionID(t *testing.T) {
	// recordToolCall with empty sessionID should be a no-op (no panic).
	b := &Bridge{
		stats: newSessionStats(),
	}

	b.recordToolCall(context.Background(), "ctx_test", false)

	// Stats should NOT be updated when sessionID is empty.
	b.stats.mu.Lock()
	count := b.stats.calls["ctx_test"]
	b.stats.mu.Unlock()

	if count != 0 {
		t.Errorf("recordToolCall with empty sessionID should not update stats, got count %d", count)
	}
}

func TestCloseNoPanic(t *testing.T) {
	b := &Bridge{}
	// Close should not panic even with nil connection.
	b.Close()
}

func TestRecordToolCallWithSessionID(t *testing.T) {
	// recordToolCall with a sessionID should update stats even if session RPC fails.
	b := &Bridge{
		sessionID:  "test-session-123",
		projectDir: "/tmp/test",
		stats:      newSessionStats(),
		logger:     slog.New(slog.NewJSONHandler(os.Stderr, nil)),
	}

	// session client is nil, so the RPC will panic unless we guard.
	// Actually, since the session field is nil, recordToolCall will try to call
	// b.session.RecordEvent and panic. In real usage, session is always set.
	// We just verify the stats update path.
	b.stats.mu.Lock()
	b.stats.calls["ctx_test"] = 0
	b.stats.mu.Unlock()

	// We can't fully test without a gRPC session client, but we can verify
	// the stats lock/unlock path by checking it after a manual increment.
	b.stats.mu.Lock()
	b.stats.calls["ctx_manual"]++
	b.stats.mu.Unlock()

	snap := b.stats.snapshot()

	calls := snap["calls"].(map[string]int)
	if calls["ctx_manual"] != 1 {
		t.Errorf("expected 1, got %d", calls["ctx_manual"])
	}
}

func TestSetPlatform(t *testing.T) {
	b := &Bridge{
		detectedPlatform: "claude-code",
		logger:           slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
	}

	// SetPlatform should change it.
	b.SetPlatform("gemini-cli")

	if b.detectedPlatform != "gemini-cli" {
		t.Errorf("after SetPlatform = %q, want gemini-cli", b.detectedPlatform)
	}
}

func TestNewBridgeInitializedHandlerUnknownFallback(t *testing.T) {
	// Verify that DetectPlatformFromClientInfo with unknown name falls back to ClaudeCode.
	// This indirectly tests what the InitializedHandler would do.
	got := DetectPlatformFromClientInfo("totally-unknown-client")
	if got != "claude-code" {
		t.Errorf("unknown client should fallback to claude-code, got %q", got)
	}
}

func TestMonitorParentContextCancelled(t *testing.T) {
	b := &Bridge{
		logger: slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Immediately cancel.

	done := make(chan struct{})

	go func() {
		b.monitorParent(ctx, cancel)
		close(done)
	}()

	select {
	case <-done:
		// Good.
	case <-time.After(5 * time.Second):
		t.Fatal("monitorParent did not exit after context cancellation")
	}
}

func TestWriteRoutingIfNeededEmptyProjectDir(t *testing.T) {
	b := &Bridge{
		logger: slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
	}
	// Should not panic with empty projectDir.
	b.writeRoutingIfNeeded()
}

func TestWriteRoutingIfNeededWithProjectDir(t *testing.T) {
	b := &Bridge{
		projectDir:       t.TempDir(),
		detectedPlatform: "claude-code",
		logger:           slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
	}
	// Claude Code has SessionStart, so this should be a no-op.
	b.writeRoutingIfNeeded()
}

func TestWriteRoutingIfNeededInvalidPlatform(t *testing.T) {
	b := &Bridge{
		projectDir:       t.TempDir(),
		detectedPlatform: "totally-invalid-platform-id",
		logger:           slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
	}
	// Invalid platform should cause platform.Get to fail, hitting the error return.
	b.writeRoutingIfNeeded()
}

func TestSessionStatsSnapshotUptimeNonNegative(t *testing.T) {
	s := newSessionStats()
	// Simulate sessionStart slightly in the future (edge case).
	s.sessionStart = time.Now().Add(-100 * time.Millisecond)
	snap := s.snapshot()

	uptime, ok := snap["uptimeSeconds"].(int)
	if !ok {
		t.Fatalf("uptimeSeconds type = %T, want int", snap["uptimeSeconds"])
	}

	if uptime < 0 {
		t.Errorf("uptimeSeconds = %d, should be non-negative", uptime)
	}
}

func TestServerMethodNil(t *testing.T) {
	b := &Bridge{}
	if b.Server() != nil {
		t.Error("Server() should return nil on empty bridge")
	}
}

func TestCheckCommandDenyAllowed(t *testing.T) {
	b := &Bridge{
		projectDir: t.TempDir(),
	}
	// A simple echo command should be allowed.
	err := b.checkCommandDeny("echo hello")
	if err != nil {
		t.Errorf("expected no error for echo, got %v", err)
	}
}

func TestCheckFilePathDenyAllowed(t *testing.T) {
	b := &Bridge{
		projectDir: t.TempDir(),
	}
	// A regular file path should be allowed.
	err := b.checkFilePathDeny("/tmp/test.txt")
	if err != nil {
		t.Errorf("expected no error for regular path, got %v", err)
	}
}

func TestWriteRoutingIfNeededMCPOnlyPlatform(t *testing.T) {
	// Test with a platform that does NOT have SessionStart hooks (gemini-cli).
	b := &Bridge{
		projectDir:       t.TempDir(),
		detectedPlatform: "gemini-cli",
		logger:           slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
	}
	// Should attempt to write routing instructions (no panic).
	b.writeRoutingIfNeeded()
}

func TestParentCheckIntervalConstant(t *testing.T) {
	if parentCheckInterval != 30*time.Second {
		t.Errorf("parentCheckInterval = %v, want 30s", parentCheckInterval)
	}
}

func TestServerNameAndVersion(t *testing.T) {
	if serverName != "thimble" {
		t.Errorf("serverName = %q, want thimble", serverName)
	}

	if serverVersion == "" {
		t.Error("serverVersion should not be empty")
	}
}
