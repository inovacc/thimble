package hooklog

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStats_WithErrors(t *testing.T) {
	l := newTestLogger(t)

	l.Log(Entry{Platform: "claude-code", Event: "PreToolUse", ToolName: "Bash"})
	l.Log(Entry{Platform: "claude-code", Event: "PreToolUse", ToolName: "Read", Blocked: true})
	l.Log(Entry{Platform: "claude-code", Event: "PostToolUse", ToolName: "Edit", Error: "timeout"})

	total, allowed, blocked, errored := l.Stats()
	if total != 3 {
		t.Errorf("total = %d, want 3", total)
	}

	if allowed != 1 {
		t.Errorf("allowed = %d, want 1", allowed)
	}

	if blocked != 1 {
		t.Errorf("blocked = %d, want 1", blocked)
	}

	if errored != 1 {
		t.Errorf("errored = %d, want 1", errored)
	}
}

func TestStats_EmptyFile(t *testing.T) {
	l := newTestLogger(t)

	total, allowed, blocked, errored := l.Stats()
	if total != 0 || allowed != 0 || blocked != 0 || errored != 0 {
		t.Errorf("empty stats = (%d, %d, %d, %d), want all zeros", total, allowed, blocked, errored)
	}
}

func TestRead_EmptyFile(t *testing.T) {
	l := newTestLogger(t)

	entries, err := l.Read(nil)
	if err != nil {
		t.Fatalf("Read empty: %v", err)
	}

	if len(entries) != 0 {
		t.Errorf("expected 0 entries from empty file, got %d", len(entries))
	}
}

func TestRead_CorruptedLines(t *testing.T) {
	l := newTestLogger(t)

	// Write a valid entry first.
	l.Log(Entry{Platform: "claude-code", Event: "PreToolUse", ToolName: "Bash"})

	// Append a corrupted line directly.
	f, err := os.OpenFile(l.Path(), os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}

	_, _ = f.WriteString("this is not json\n")
	_ = f.Close()

	// Read should skip corrupted lines.
	entries, err := l.Read(nil)
	if err != nil {
		t.Fatalf("Read with corrupted lines: %v", err)
	}

	if len(entries) != 1 {
		t.Errorf("expected 1 valid entry, got %d", len(entries))
	}
}

func TestRead_FilterByProjectDir(t *testing.T) {
	l := newTestLogger(t)

	l.Log(Entry{Platform: "claude-code", Event: "PreToolUse", ProjectDir: "/project/a"})
	l.Log(Entry{Platform: "claude-code", Event: "PreToolUse", ProjectDir: "/project/b"})
	l.Log(Entry{Platform: "claude-code", Event: "PostToolUse", ProjectDir: "/project/a"})

	entries, err := l.Read(&ReadOptions{ProjectDir: "/project/a"})
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	if len(entries) != 2 {
		t.Errorf("expected 2 entries for /project/a, got %d", len(entries))
	}
}

func TestClear_NoFile(t *testing.T) {
	dir := t.TempDir()

	l, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}

	// Clear on a file that doesn't exist should return an error.
	err = l.Clear()
	if err == nil {
		t.Error("expected error clearing nonexistent log")
	}
}

func TestNew_CreatesLogger(t *testing.T) {
	dir := t.TempDir()

	l, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	expectedPath := filepath.Join(dir, "hooklog.jsonl")
	if l.Path() != expectedPath {
		t.Errorf("Path = %q, want %q", l.Path(), expectedPath)
	}
}

func TestClose_IsNoop(t *testing.T) {
	l := newTestLogger(t)
	if err := l.Close(); err != nil {
		t.Errorf("Close should be no-op, got error: %v", err)
	}

	// Can close multiple times.
	if err := l.Close(); err != nil {
		t.Errorf("second Close should also be no-op, got error: %v", err)
	}
}
