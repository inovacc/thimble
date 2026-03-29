package hooklog

import (
	"testing"
	"time"
)

func newTestLogger(t *testing.T) *Logger {
	t.Helper()

	l, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	t.Cleanup(func() { _ = l.Close() })

	return l
}

func TestLogAndRead(t *testing.T) {
	l := newTestLogger(t)

	l.Log(Entry{
		Timestamp:  time.Date(2026, 3, 18, 10, 0, 0, 0, time.UTC),
		Platform:   "claude-code",
		Event:      "PreToolUse",
		ToolName:   "Bash",
		DurationMs: 3,
	})
	l.Log(Entry{
		Timestamp:  time.Date(2026, 3, 18, 10, 0, 1, 0, time.UTC),
		Platform:   "claude-code",
		Event:      "PreToolUse",
		ToolName:   "Read",
		Blocked:    true,
		Reason:     "file denied",
		DurationMs: 1,
	})
	l.Log(Entry{
		Timestamp:  time.Date(2026, 3, 18, 10, 0, 2, 0, time.UTC),
		Platform:   "gemini-cli",
		Event:      "PostToolUse",
		ToolName:   "Edit",
		DurationMs: 5,
	})

	entries, err := l.Read(nil)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	if entries[0].Platform != "claude-code" || entries[0].ToolName != "Bash" {
		t.Errorf("unexpected first entry: %+v", entries[0])
	}

	if !entries[1].Blocked || entries[1].Reason != "file denied" {
		t.Errorf("unexpected second entry: %+v", entries[1])
	}
}

func TestReadWithFilters(t *testing.T) {
	l := newTestLogger(t)

	l.Log(Entry{Platform: "claude-code", Event: "PreToolUse", ToolName: "Bash"})
	l.Log(Entry{Platform: "claude-code", Event: "PreToolUse", ToolName: "Read", Blocked: true, Reason: "denied"})
	l.Log(Entry{Platform: "gemini-cli", Event: "PostToolUse", ToolName: "Edit"})
	l.Log(Entry{Platform: "claude-code", Event: "SessionStart"})

	tests := []struct {
		name     string
		opts     ReadOptions
		expected int
	}{
		{"filter by platform", ReadOptions{Platform: "gemini-cli"}, 1},
		{"filter by event", ReadOptions{Event: "PreToolUse"}, 2},
		{"filter by tool", ReadOptions{ToolName: "Bash"}, 1},
		{"blocked only", ReadOptions{BlockedOnly: true}, 1},
		{"limit", ReadOptions{Limit: 2}, 2},
		{"combined", ReadOptions{Platform: "claude-code", Event: "PreToolUse"}, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entries, err := l.Read(&tt.opts)
			if err != nil {
				t.Fatalf("Read: %v", err)
			}

			if len(entries) != tt.expected {
				t.Errorf("expected %d entries, got %d", tt.expected, len(entries))
			}
		})
	}
}

func TestReadLimit(t *testing.T) {
	l := newTestLogger(t)

	for i := range 10 {
		l.Log(Entry{
			Timestamp: time.Date(2026, 3, 18, 10, 0, i, 0, time.UTC),
			Platform:  "claude-code",
			Event:     "PreToolUse",
			ToolName:  "Bash",
		})
	}

	entries, err := l.Read(&ReadOptions{Limit: 3})
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	// Most recent 3: seconds 7, 8, 9.
	if entries[0].Timestamp.Second() != 7 {
		t.Errorf("expected second=7, got %d", entries[0].Timestamp.Second())
	}
}

func TestClear(t *testing.T) {
	l := newTestLogger(t)

	l.Log(Entry{Platform: "claude-code", Event: "PreToolUse"})

	if err := l.Clear(); err != nil {
		t.Fatalf("Clear: %v", err)
	}

	entries, err := l.Read(nil)
	if err != nil {
		t.Fatalf("Read after clear: %v", err)
	}

	if len(entries) != 0 {
		t.Errorf("expected 0 entries after clear, got %d", len(entries))
	}
}

func TestStats(t *testing.T) {
	l := newTestLogger(t)

	l.Log(Entry{Platform: "claude-code", Event: "PreToolUse", ToolName: "Bash"})
	l.Log(Entry{Platform: "claude-code", Event: "PreToolUse", ToolName: "Read", Blocked: true})
	l.Log(Entry{Platform: "claude-code", Event: "PostToolUse", ToolName: "Edit"})

	total, allowed, blocked, _ := l.Stats()
	if total != 3 {
		t.Errorf("expected total=3, got %d", total)
	}

	if allowed != 2 {
		t.Errorf("expected allowed=2, got %d", allowed)
	}

	if blocked != 1 {
		t.Errorf("expected blocked=1, got %d", blocked)
	}
}

func TestLogAutoTimestamp(t *testing.T) {
	l := newTestLogger(t)

	before := time.Now().UTC()

	l.Log(Entry{Platform: "test", Event: "PreToolUse"})

	after := time.Now().UTC()

	entries, err := l.Read(nil)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	ts := entries[0].Timestamp
	if ts.Before(before.Add(-time.Second)) || ts.After(after.Add(time.Second)) {
		t.Errorf("auto-timestamp %v not in expected range [%v, %v]", ts, before, after)
	}
}

func TestDebugMode(t *testing.T) {
	l := newTestLogger(t)

	// Without debug, payloads should NOT be stored.
	l.Log(Entry{
		Platform:    "claude-code",
		Event:       "PreToolUse",
		ToolName:    "Bash",
		ToolInput:   `{"command":"ls"}`,
		Response:    `{"result":"ok"}`,
		GRPCPayload: `{"raw":"payload"}`,
	})

	entries, err := l.Read(nil)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	if entries[0].ToolInput != "" {
		t.Errorf("expected empty ToolInput without debug, got %q", entries[0].ToolInput)
	}

	// Enable debug mode.
	l.SetDebug(true)

	if !l.Debug() {
		t.Error("expected debug=true")
	}

	l.Log(Entry{
		Platform:    "claude-code",
		Event:       "PreToolUse",
		ToolName:    "Read",
		ToolInput:   `{"file_path":"/foo"}`,
		Response:    `{"result":"content"}`,
		GRPCPayload: `{"full":"payload"}`,
	})

	entries, err = l.Read(nil)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	last := entries[len(entries)-1]
	if last.ToolInput != `{"file_path":"/foo"}` {
		t.Errorf("expected ToolInput with debug, got %q", last.ToolInput)
	}

	if last.Response != `{"result":"content"}` {
		t.Errorf("expected Response with debug, got %q", last.Response)
	}

	if last.GRPCPayload != `{"full":"payload"}` {
		t.Errorf("expected GRPCPayload with debug, got %q", last.GRPCPayload)
	}
}

func TestPath(t *testing.T) {
	l := newTestLogger(t)
	if l.Path() == "" {
		t.Error("expected non-empty path")
	}
}
