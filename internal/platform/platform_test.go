package platform

import (
	"encoding/json"
	"testing"
)

func TestAllAdaptersImplementInterface(t *testing.T) {
	for _, id := range AllPlatformIDs() {
		adapter, err := Get(id)
		if err != nil {
			t.Fatalf("Get(%s) error: %v", id, err)
		}

		if adapter.ID() != id {
			t.Errorf("adapter.ID() = %s, want %s", adapter.ID(), id)
		}

		if adapter.Name() == "" {
			t.Errorf("adapter.Name() is empty for %s", id)
		}

		if adapter.Paradigm() == "" {
			t.Errorf("adapter.Paradigm() is empty for %s", id)
		}
	}
}

func TestGetUnknownPlatform(t *testing.T) {
	_, err := Get("nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown platform")
	}
}

func TestClaudeCodeParsePreToolUse(t *testing.T) {
	raw := json.RawMessage(`{
		"tool_name": "Bash",
		"tool_input": {"command": "ls -la"},
		"session_id": "abc-123",
		"transcript_path": "/home/user/.claude/projects/test/sessions/d1e2f3a4-b5c6-7890-abcd-ef1234567890.jsonl"
	}`)

	adapter, _ := Get(PlatformClaudeCode)
	ev := adapter.ParsePreToolUse(raw)

	if ev.ToolName != "Bash" {
		t.Errorf("ToolName = %q, want %q", ev.ToolName, "Bash")
	}

	if ev.ToolInput["command"] != "ls -la" {
		t.Errorf("ToolInput[command] = %v, want %q", ev.ToolInput["command"], "ls -la")
	}
	// Should extract UUID from transcript_path.
	if ev.SessionID != "d1e2f3a4-b5c6-7890-abcd-ef1234567890" {
		t.Errorf("SessionID = %q, want UUID from transcript_path", ev.SessionID)
	}
}

func TestClaudeCodeParseSessionStart(t *testing.T) {
	raw := json.RawMessage(`{"source": "compact", "session_id": "s1"}`)
	adapter, _ := Get(PlatformClaudeCode)
	ev := adapter.ParseSessionStart(raw)

	if ev.Source != "compact" {
		t.Errorf("Source = %q, want %q", ev.Source, "compact")
	}

	if ev.SessionID != "s1" {
		t.Errorf("SessionID = %q, want %q", ev.SessionID, "s1")
	}
}

func TestClaudeCodeFormatDeny(t *testing.T) {
	adapter, _ := Get(PlatformClaudeCode)
	out := adapter.FormatPreToolUse(HookResponse{
		Decision: "deny",
		Reason:   "blocked by policy",
	})

	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// Claude Code wraps in hookSpecificOutput.
	hso, ok := m["hookSpecificOutput"].(map[string]any)
	if !ok {
		t.Fatalf("missing hookSpecificOutput wrapper, got: %s", out)
	}

	if hso["permissionDecision"] != "deny" {
		t.Errorf("permissionDecision = %v, want deny", hso["permissionDecision"])
	}

	if hso["reason"] != "blocked by policy" {
		t.Errorf("reason = %v, want 'blocked by policy'", hso["reason"])
	}
}

func TestGeminiCLIFormatDeny(t *testing.T) {
	adapter, _ := Get(PlatformGeminiCLI)
	out := adapter.FormatPreToolUse(HookResponse{
		Decision: "deny",
		Reason:   "not allowed",
	})

	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	// Gemini uses "decision" not "permissionDecision".
	if m["decision"] != "deny" {
		t.Errorf("decision = %v, want deny", m["decision"])
	}
}

func TestGeminiCLIFormatModify(t *testing.T) {
	adapter, _ := Get(PlatformGeminiCLI)
	out := adapter.FormatPreToolUse(HookResponse{
		Decision:     "modify",
		UpdatedInput: map[string]any{"command": "echo safe"},
	})

	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	hso, ok := m["hookSpecificOutput"].(map[string]any)
	if !ok {
		t.Fatal("missing hookSpecificOutput wrapper")
	}

	if hso["tool_input"] == nil {
		t.Error("missing tool_input in hookSpecificOutput")
	}
}

func TestVSCodeCopilotFormatDeny(t *testing.T) {
	adapter, _ := Get(PlatformVSCodeCopilot)
	out := adapter.FormatPreToolUse(HookResponse{
		Decision: "deny",
		Reason:   "blocked",
	})

	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	// VS Code uses same permissionDecision as Claude Code.
	if m["permissionDecision"] != "deny" {
		t.Errorf("permissionDecision = %v, want deny", m["permissionDecision"])
	}
}

func TestVSCodeCopilotFormatModify(t *testing.T) {
	adapter, _ := Get(PlatformVSCodeCopilot)
	out := adapter.FormatPreToolUse(HookResponse{
		Decision:     "modify",
		UpdatedInput: map[string]any{"filePath": "/tmp/test"},
	})

	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	// VS Code wraps in hookSpecificOutput with hookEventName.
	hso, ok := m["hookSpecificOutput"].(map[string]any)
	if !ok {
		t.Fatal("missing hookSpecificOutput wrapper")
	}

	if hso["hookEventName"] != "PreToolUse" {
		t.Errorf("hookEventName = %v, want PreToolUse", hso["hookEventName"])
	}

	if hso["updatedInput"] == nil {
		t.Error("missing updatedInput in hookSpecificOutput")
	}
}

func TestCursorFormatDeny(t *testing.T) {
	adapter, _ := Get(PlatformCursor)
	out := adapter.FormatPreToolUse(HookResponse{
		Decision: "deny",
		Reason:   "forbidden",
	})

	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	// Cursor uses "permission" + "user_message".
	if m["permission"] != "deny" {
		t.Errorf("permission = %v, want deny", m["permission"])
	}

	if m["user_message"] != "forbidden" {
		t.Errorf("user_message = %v, want forbidden", m["user_message"])
	}
}

func TestCursorParseSessionID(t *testing.T) {
	raw := json.RawMessage(`{"tool_name": "Shell", "conversation_id": "conv-456"}`)
	adapter, _ := Get(PlatformCursor)
	ev := adapter.ParsePreToolUse(raw)

	if ev.SessionID != "conv-456" {
		t.Errorf("SessionID = %q, want %q", ev.SessionID, "conv-456")
	}
}

func TestCursorProjectDirFromWorkspaceRoots(t *testing.T) {
	raw := json.RawMessage(`{"tool_name": "Shell", "workspace_roots": ["/home/user/project"]}`)
	adapter, _ := Get(PlatformCursor)
	ev := adapter.ParsePreToolUse(raw)

	if ev.ProjectDir != "/home/user/project" {
		t.Errorf("ProjectDir = %q, want %q", ev.ProjectDir, "/home/user/project")
	}
}

func TestCodexAdapterNoOps(t *testing.T) {
	adapter, _ := Get(PlatformCodex)
	caps := adapter.Capabilities()

	if caps.PreToolUse || caps.PostToolUse || caps.PreCompact || caps.SessionStart {
		t.Error("Codex should have no hook capabilities")
	}

	out := adapter.FormatPreToolUse(HookResponse{Decision: "deny"})
	if string(out) != "{}" {
		t.Errorf("Codex FormatPreToolUse should return {}, got %s", out)
	}
}

func TestOpenCodeFormatDeny(t *testing.T) {
	adapter, _ := Get(PlatformOpenCode)
	out := adapter.FormatPreToolUse(HookResponse{
		Decision: "deny",
		Reason:   "blocked",
	})

	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	// OpenCode signals errors via error flag.
	if m["error"] != true {
		t.Errorf("error = %v, want true", m["error"])
	}
}

func TestDetectReturnsSignal(t *testing.T) {
	signal := Detect()
	// Should always return a valid platform (at least fallback).
	if signal.Platform == "" {
		t.Error("Detect() returned empty platform")
	}

	if signal.Confidence == "" {
		t.Error("Detect() returned empty confidence")
	}

	if signal.Reason == "" {
		t.Error("Detect() returned empty reason")
	}
}

func TestParseEmptyPayload(t *testing.T) {
	raw := json.RawMessage(`{}`)

	for _, id := range AllPlatformIDs() {
		adapter, _ := Get(id)
		// Should not panic on empty payload.
		_ = adapter.ParsePreToolUse(raw)
		_ = adapter.ParsePostToolUse(raw)
		_ = adapter.ParsePreCompact(raw)
		_ = adapter.ParseSessionStart(raw)
	}
}

func TestFormatAllowReturnsNilOrEmpty(t *testing.T) {
	resp := HookResponse{Decision: "allow"}

	for _, id := range AllPlatformIDs() {
		adapter, _ := Get(id)

		out := adapter.FormatPreToolUse(resp)
		// "allow" means passthrough — platforms return nil (no stdout) or "{}".
		if out != nil && string(out) != "{}" {
			t.Errorf("%s FormatPreToolUse(allow) = %s, want nil or {}", id, out)
		}
	}
}

func TestClaudeCodeFormatPostToolUseContext(t *testing.T) {
	adapter, _ := Get(PlatformClaudeCode)
	out := adapter.FormatPostToolUse(HookResponse{
		AdditionalContext: "session context here",
	})

	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// Claude Code wraps in hookSpecificOutput.
	hso, ok := m["hookSpecificOutput"].(map[string]any)
	if !ok {
		t.Fatalf("missing hookSpecificOutput wrapper, got: %s", out)
	}

	if hso["additionalContext"] != "session context here" {
		t.Errorf("additionalContext = %v, want 'session context here'", hso["additionalContext"])
	}
}

func TestClaudeCodeSessionIDFallbackChain(t *testing.T) {
	// No transcript_path, no session_id → falls back to env or ppid.
	raw := json.RawMessage(`{"tool_name": "Read"}`)
	adapter, _ := Get(PlatformClaudeCode)
	ev := adapter.ParsePreToolUse(raw)

	// Should start with "pid-" when no session info available.
	if ev.SessionID == "" {
		t.Error("SessionID should not be empty")
	}
}
