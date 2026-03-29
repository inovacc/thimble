package model

import (
	"encoding/json"
	"testing"
)

func TestExecResultJSON(t *testing.T) {
	orig := ExecResult{
		Stdout:       "hello",
		Stderr:       "warn: something",
		ExitCode:     0,
		TimedOut:     false,
		Backgrounded: true,
	}

	data, err := json.Marshal(orig)
	if err != nil {
		t.Fatal(err)
	}

	var decoded ExecResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}

	if decoded.Stdout != orig.Stdout {
		t.Errorf("Stdout = %q, want %q", decoded.Stdout, orig.Stdout)
	}

	if decoded.Backgrounded != orig.Backgrounded {
		t.Errorf("Backgrounded = %v, want %v", decoded.Backgrounded, orig.Backgrounded)
	}
}

func TestSessionEventJSON(t *testing.T) {
	orig := SessionEvent{
		Type:     "file_read",
		Category: "file",
		Data:     "/src/main.go",
		Priority: PriorityHigh,
		DataHash: "abc123",
	}

	data, err := json.Marshal(orig)
	if err != nil {
		t.Fatal(err)
	}

	var decoded SessionEvent
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}

	if decoded.Type != orig.Type || decoded.Priority != orig.Priority || decoded.DataHash != orig.DataHash {
		t.Errorf("round-trip mismatch: got %+v", decoded)
	}
}

func TestHookInputJSON(t *testing.T) {
	orig := HookInput{
		Platform:   "claude",
		Event:      "PostToolUse",
		SessionID:  "sess-123",
		ProjectDir: "/project",
		ToolCall: &ToolCall{
			ToolName:  "Bash",
			ToolInput: map[string]any{"command": "echo hi"},
			IsError:   false,
		},
		Extra: map[string]any{"key": "value"},
	}

	data, err := json.Marshal(orig)
	if err != nil {
		t.Fatal(err)
	}

	var decoded HookInput
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}

	if decoded.Platform != orig.Platform {
		t.Errorf("Platform = %q, want %q", decoded.Platform, orig.Platform)
	}

	if decoded.ToolCall == nil {
		t.Fatal("ToolCall is nil after round-trip")
	}

	if decoded.ToolCall.ToolName != "Bash" {
		t.Errorf("ToolCall.ToolName = %q, want Bash", decoded.ToolCall.ToolName)
	}
}

func TestHookOutputJSON(t *testing.T) {
	orig := HookOutput{
		Blocked: true,
		Reason:  "denied by policy",
	}

	data, err := json.Marshal(orig)
	if err != nil {
		t.Fatal(err)
	}

	var decoded HookOutput
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}

	if !decoded.Blocked || decoded.Reason != orig.Reason {
		t.Errorf("round-trip mismatch: got %+v", decoded)
	}
}

func TestSearchResultJSON(t *testing.T) {
	orig := SearchResult{
		Title:       "main.go",
		Content:     "package main",
		Source:      "file:///main.go",
		Rank:        -1.5,
		ContentType: "code",
		MatchLayer:  "porter_and",
	}

	data, err := json.Marshal(orig)
	if err != nil {
		t.Fatal(err)
	}

	var decoded SearchResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}

	if decoded.Rank != orig.Rank || decoded.MatchLayer != orig.MatchLayer {
		t.Errorf("round-trip mismatch: got %+v", decoded)
	}
}

func TestResumeSnapshotJSON(t *testing.T) {
	orig := ResumeSnapshot{
		GeneratedAt: "2026-03-17T10:00:00Z",
		Summary:     "session context",
		Events: []SessionEvent{
			{Type: "file_read", Category: "file", Data: "main.go", Priority: PriorityNormal},
			{Type: "error_tool", Category: "error", Data: "build failed", Priority: PriorityHigh},
		},
	}

	data, err := json.Marshal(orig)
	if err != nil {
		t.Fatal(err)
	}

	var decoded ResumeSnapshot
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}

	if len(decoded.Events) != 2 {
		t.Errorf("Events = %d, want 2", len(decoded.Events))
	}

	if decoded.Summary != orig.Summary {
		t.Errorf("Summary = %q, want %q", decoded.Summary, orig.Summary)
	}
}

func TestPriorityConstants(t *testing.T) {
	if PriorityLow >= PriorityNormal {
		t.Error("PriorityLow should be less than PriorityNormal")
	}

	if PriorityNormal >= PriorityHigh {
		t.Error("PriorityNormal should be less than PriorityHigh")
	}

	if PriorityHigh >= PriorityCritical {
		t.Error("PriorityHigh should be less than PriorityCritical")
	}
}

func TestIndexResultJSON(t *testing.T) {
	orig := IndexResult{
		SourceID:    42,
		Label:       "main.go",
		TotalChunks: 10,
		CodeChunks:  7,
	}

	data, err := json.Marshal(orig)
	if err != nil {
		t.Fatal(err)
	}

	var decoded IndexResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}

	if decoded.SourceID != orig.SourceID || decoded.TotalChunks != orig.TotalChunks {
		t.Errorf("round-trip mismatch: got %+v", decoded)
	}
}

func TestStoreStatsJSON(t *testing.T) {
	orig := StoreStats{Sources: 5, Chunks: 100, CodeChunks: 60}

	data, err := json.Marshal(orig)
	if err != nil {
		t.Fatal(err)
	}

	var decoded StoreStats
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}

	if decoded != orig {
		t.Errorf("round-trip mismatch: got %+v, want %+v", decoded, orig)
	}
}

func TestToolCallOmitsEmpty(t *testing.T) {
	tc := ToolCall{
		ToolName:  "Read",
		ToolInput: map[string]any{"file_path": "/a.go"},
	}

	data, err := json.Marshal(tc)
	if err != nil {
		t.Fatal(err)
	}

	s := string(data)
	// toolResponse and isError should be omitted when empty/false.
	if contains(s, "toolResponse") {
		t.Error("empty toolResponse should be omitted")
	}

	if contains(s, "isError") {
		t.Error("false isError should be omitted")
	}
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && json.Valid([]byte(s)) && findSubstring(s, substr)
}

func findSubstring(s, substr string) bool {
	for i := range len(s) - len(substr) + 1 {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}

	return false
}
