package platform

import (
	"encoding/json"
	"testing"
)

func TestOpenClawParseCamelCase(t *testing.T) {
	a := &openClawAdapter{}
	raw := json.RawMessage(`{
		"toolName": "Bash",
		"params": {"command": "ls"},
		"sessionId": "sess1",
		"projectDir": "/project"
	}`)

	ev := a.ParsePreToolUse(raw)
	if ev.ToolName != "Bash" {
		t.Errorf("ToolName = %q, want Bash", ev.ToolName)
	}

	if ev.SessionID != "sess1" {
		t.Errorf("SessionID = %q, want sess1", ev.SessionID)
	}

	if ev.ProjectDir != "/project" {
		t.Errorf("ProjectDir = %q, want /project", ev.ProjectDir)
	}

	if ev.ToolInput == nil || ev.ToolInput["command"] != "ls" {
		t.Errorf("ToolInput = %v, want command=ls", ev.ToolInput)
	}
}

func TestOpenClawParseSnakeCase(t *testing.T) {
	a := &openClawAdapter{}
	raw := json.RawMessage(`{
		"tool_name": "Read",
		"tool_input": {"file_path": "/tmp/file.go"},
		"sessionID": "sess2",
		"directory": "/workspace"
	}`)

	ev := a.ParsePreToolUse(raw)
	if ev.ToolName != "Read" {
		t.Errorf("ToolName = %q, want Read", ev.ToolName)
	}

	if ev.SessionID != "sess2" {
		t.Errorf("SessionID = %q, want sess2", ev.SessionID)
	}

	if ev.ProjectDir != "/workspace" {
		t.Errorf("ProjectDir = %q, want /workspace", ev.ProjectDir)
	}
}

func TestOpenClawWorkspaceKeyFromID(t *testing.T) {
	a := &openClawAdapter{}
	raw := json.RawMessage(`{"workspace_id": "ws-abc-123", "projectDir": "/fallback"}`)

	key := a.WorkspaceKey(raw)
	if key != "ws-abc-123" {
		t.Errorf("WorkspaceKey = %q, want ws-abc-123", key)
	}
}

func TestOpenClawWorkspaceKeyFallback(t *testing.T) {
	a := &openClawAdapter{}
	raw := json.RawMessage(`{"projectDir": "/my/project"}`)

	key := a.WorkspaceKey(raw)
	if key != "/my/project" {
		t.Errorf("WorkspaceKey = %q, want /my/project", key)
	}
}

func TestOpenClawWorkspaceKeyEmpty(t *testing.T) {
	a := &openClawAdapter{}
	raw := json.RawMessage(`{}`)

	key := a.WorkspaceKey(raw)
	if key != "" {
		t.Errorf("WorkspaceKey = %q, want empty", key)
	}
}

func TestOpenClawFormatAskToBlock(t *testing.T) {
	a := &openClawAdapter{}
	resp := HookResponse{
		Decision: "ask",
		Reason:   "needs confirmation",
	}

	raw := a.FormatPreToolUse(resp)

	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if block, ok := result["block"].(bool); !ok || !block {
		t.Errorf("block = %v, want true", result["block"])
	}

	if reason, ok := result["blockReason"].(string); !ok || reason != "needs confirmation" {
		t.Errorf("blockReason = %q, want 'needs confirmation'", reason)
	}
}
