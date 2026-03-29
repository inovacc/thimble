package platform

import (
	"encoding/json"
	"testing"
)

func TestKiroParsePreToolUse(t *testing.T) {
	a := &kiroAdapter{}
	raw := json.RawMessage(`{
		"hook_event_name": "pretooluse",
		"tool_name": "execute_bash",
		"tool_input": {"command": "ls"},
		"cwd": "/home/user/project",
		"session_id": "abc123"
	}`)

	ev := a.ParsePreToolUse(raw)
	if ev.ToolName != "execute_bash" {
		t.Errorf("ToolName = %q, want execute_bash", ev.ToolName)
	}

	if ev.ProjectDir != "/home/user/project" {
		t.Errorf("ProjectDir = %q, want /home/user/project", ev.ProjectDir)
	}

	if ev.SessionID != "abc123" {
		t.Errorf("SessionID = %q, want abc123", ev.SessionID)
	}
}

func TestKiroFormatPreToolUseDeny(t *testing.T) {
	a := &kiroAdapter{}
	resp := HookResponse{
		Decision: "deny",
		Reason:   "command blocked",
	}

	raw := a.FormatPreToolUse(resp)

	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	exitCode, ok := result["exitCode"].(float64)
	if !ok || exitCode != 2 {
		t.Errorf("exitCode = %v, want 2", result["exitCode"])
	}

	stderr, ok := result["stderr"].(string)
	if !ok || stderr != "command blocked" {
		t.Errorf("stderr = %q, want 'command blocked'", stderr)
	}
}

func TestKiroFormatPreToolUseAllow(t *testing.T) {
	a := &kiroAdapter{}
	resp := HookResponse{
		Decision:          "allow",
		AdditionalContext: "some context",
	}

	raw := a.FormatPreToolUse(resp)

	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	exitCode, ok := result["exitCode"].(float64)
	if !ok || exitCode != 0 {
		t.Errorf("exitCode = %v, want 0", result["exitCode"])
	}

	stdout, ok := result["stdout"].(string)
	if !ok || stdout != "some context" {
		t.Errorf("stdout = %q, want 'some context'", stdout)
	}
}
