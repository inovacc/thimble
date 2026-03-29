package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewHookDebugger(t *testing.T) {
	dbg := newHookDebugger("claude-code", "PreToolUse")

	if dbg == nil {
		t.Fatal("newHookDebugger returned nil")
	}

	if dbg.dir == "" {
		t.Error("debugger dir is empty")
	}

	if dbg.ts == "" {
		t.Error("debugger timestamp is empty")
	}

	// Verify the debug directory was created.
	info, err := os.Stat(dbg.dir)
	if err != nil {
		t.Fatalf("debug dir stat: %v", err)
	}

	if !info.IsDir() {
		t.Error("debug dir is not a directory")
	}
}

func TestHookDebugger_WriteInput(t *testing.T) {
	tmp := t.TempDir()
	dbg := &hookDebugger{dir: tmp, ts: "20260317_120000.000"}

	dbg.writeInput("PreToolUse", []byte(`{"tool":"test"}`))

	files, err := os.ReadDir(tmp)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}

	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}

	data, err := os.ReadFile(filepath.Join(tmp, files[0].Name()))
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	if string(data) != `{"tool":"test"}` {
		t.Errorf("content = %q, want %q", string(data), `{"tool":"test"}`)
	}
}

func TestEventMap_Coverage(t *testing.T) {
	// Verify all expected events are mapped.
	expected := []string{
		"pretooluse", "posttooluse", "precompact", "sessionstart", "userpromptsubmit",
		"beforetool", "aftertool", "precompress",
		"PreToolUse", "PostToolUse", "PreCompact", "SessionStart",
	}

	for _, e := range expected {
		if _, ok := eventMap[e]; !ok {
			t.Errorf("eventMap missing key %q", e)
		}
	}
}
