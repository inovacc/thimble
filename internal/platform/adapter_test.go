package platform

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestAllAdaptersSettingsPath verifies SettingsPath returns non-empty for all adapters.
func TestAllAdaptersSettingsPath(t *testing.T) {
	for _, id := range AllPlatformIDs() {
		adapter, err := Get(id)
		if err != nil {
			t.Fatalf("Get(%s): %v", id, err)
		}

		p := adapter.SettingsPath()
		if p == "" {
			t.Errorf("%s: SettingsPath() returned empty", id)
		}
	}
}

// TestAllAdaptersSessionDir verifies SessionDir returns non-empty for all adapters.
func TestAllAdaptersSessionDir(t *testing.T) {
	for _, id := range AllPlatformIDs() {
		adapter, err := Get(id)
		if err != nil {
			t.Fatalf("Get(%s): %v", id, err)
		}

		d := adapter.SessionDir()
		if d == "" {
			t.Errorf("%s: SessionDir() returned empty", id)
		}
	}
}

// TestAllAdaptersValidateHooks verifies ValidateHooks returns diagnostics for all adapters.
func TestAllAdaptersValidateHooks(t *testing.T) {
	for _, id := range AllPlatformIDs() {
		adapter, err := Get(id)
		if err != nil {
			t.Fatalf("Get(%s): %v", id, err)
		}

		diags := adapter.ValidateHooks()
		if len(diags) == 0 {
			t.Errorf("%s: ValidateHooks() returned no diagnostics", id)
		}
	}
}

// TestAllAdaptersFormatPostToolUse covers FormatPostToolUse for all adapters.
func TestAllAdaptersFormatPostToolUse(t *testing.T) {
	tests := []struct {
		name string
		resp HookResponse
	}{
		{
			name: "empty response",
			resp: HookResponse{},
		},
		{
			name: "with additional context",
			resp: HookResponse{AdditionalContext: "extra info"},
		},
		{
			name: "with updated output",
			resp: HookResponse{UpdatedOutput: "modified output"},
		},
	}

	for _, id := range AllPlatformIDs() {
		adapter, err := Get(id)
		if err != nil {
			t.Fatalf("Get(%s): %v", id, err)
		}

		for _, tt := range tests {
			t.Run(string(id)+"/"+tt.name, func(t *testing.T) {
				out := adapter.FormatPostToolUse(tt.resp)
				if out == nil {
					return // nil is valid (passthrough)
				}
				// Must be valid JSON.
				var m map[string]any
				if err := json.Unmarshal(out, &m); err != nil {
					t.Errorf("invalid JSON: %v, raw: %s", err, out)
				}
			})
		}
	}
}

// TestAllAdaptersFormatPreCompact covers FormatPreCompact for all adapters.
func TestAllAdaptersFormatPreCompact(t *testing.T) {
	tests := []struct {
		name string
		resp HookResponse
	}{
		{
			name: "empty",
			resp: HookResponse{},
		},
		{
			name: "with context",
			resp: HookResponse{Context: "compaction context"},
		},
	}

	for _, id := range AllPlatformIDs() {
		adapter, err := Get(id)
		if err != nil {
			t.Fatalf("Get(%s): %v", id, err)
		}

		for _, tt := range tests {
			t.Run(string(id)+"/"+tt.name, func(t *testing.T) {
				out := adapter.FormatPreCompact(tt.resp)
				if out == nil {
					return
				}

				if !json.Valid(out) {
					t.Errorf("invalid JSON: %s", out)
				}
			})
		}
	}
}

// TestAllAdaptersFormatSessionStart covers FormatSessionStart for all adapters.
func TestAllAdaptersFormatSessionStart(t *testing.T) {
	tests := []struct {
		name string
		resp HookResponse
	}{
		{
			name: "empty",
			resp: HookResponse{},
		},
		{
			name: "with context",
			resp: HookResponse{Context: "session start context"},
		},
	}

	for _, id := range AllPlatformIDs() {
		adapter, err := Get(id)
		if err != nil {
			t.Fatalf("Get(%s): %v", id, err)
		}

		for _, tt := range tests {
			t.Run(string(id)+"/"+tt.name, func(t *testing.T) {
				out := adapter.FormatSessionStart(tt.resp)
				if out == nil {
					return
				}

				if !json.Valid(out) {
					t.Errorf("invalid JSON: %s", out)
				}
			})
		}
	}
}

// TestClaudeCodeFormatPreToolUseContext tests the "context" branch.
func TestClaudeCodeFormatPreToolUseContext(t *testing.T) {
	a := &claudeCodeAdapter{}
	out := a.FormatPreToolUse(HookResponse{
		Decision:          "context",
		AdditionalContext: "important context",
	})

	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	hso, ok := m["hookSpecificOutput"].(map[string]any)
	if !ok {
		t.Fatal("missing hookSpecificOutput")
	}

	if hso["additionalContext"] != "important context" {
		t.Errorf("additionalContext = %v, want 'important context'", hso["additionalContext"])
	}
}

// TestClaudeCodeFormatPreToolUseAsk tests the "ask" branch.
func TestClaudeCodeFormatPreToolUseAsk(t *testing.T) {
	a := &claudeCodeAdapter{}
	out := a.FormatPreToolUse(HookResponse{Decision: "ask"})

	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	hso, ok := m["hookSpecificOutput"].(map[string]any)
	if !ok {
		t.Fatal("missing hookSpecificOutput")
	}

	if hso["permissionDecision"] != "ask" {
		t.Errorf("permissionDecision = %v, want ask", hso["permissionDecision"])
	}
}

// TestClaudeCodeFormatPreToolUseModify tests the "modify" branch.
func TestClaudeCodeFormatPreToolUseModify(t *testing.T) {
	a := &claudeCodeAdapter{}
	out := a.FormatPreToolUse(HookResponse{
		Decision:     "modify",
		UpdatedInput: map[string]any{"command": "echo safe"},
	})

	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	hso, ok := m["hookSpecificOutput"].(map[string]any)
	if !ok {
		t.Fatal("missing hookSpecificOutput")
	}

	if hso["updatedInput"] == nil {
		t.Error("missing updatedInput")
	}
}

// TestClaudeCodeFormatPostToolUseUpdatedOutput tests the updatedMCPToolOutput branch.
func TestClaudeCodeFormatPostToolUseUpdatedOutput(t *testing.T) {
	a := &claudeCodeAdapter{}
	out := a.FormatPostToolUse(HookResponse{
		UpdatedOutput: "modified output content",
	})

	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	hso, ok := m["hookSpecificOutput"].(map[string]any)
	if !ok {
		t.Fatal("missing hookSpecificOutput")
	}

	if hso["updatedMCPToolOutput"] != "modified output content" {
		t.Errorf("updatedMCPToolOutput = %v", hso["updatedMCPToolOutput"])
	}
}

// TestClaudeCodeFormatPostToolUseNil tests passthrough (nil) when empty.
func TestClaudeCodeFormatPostToolUseNil(t *testing.T) {
	a := &claudeCodeAdapter{}

	out := a.FormatPostToolUse(HookResponse{})
	if out != nil {
		t.Errorf("expected nil, got %s", out)
	}
}

// TestClaudeCodeFormatPreCompact tests FormatPreCompact with context.
func TestClaudeCodeFormatPreCompact(t *testing.T) {
	a := &claudeCodeAdapter{}
	out := a.FormatPreCompact(HookResponse{Context: "compact context"})

	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	hso, ok := m["hookSpecificOutput"].(map[string]any)
	if !ok {
		t.Fatal("missing hookSpecificOutput")
	}

	if hso["additionalContext"] != "compact context" {
		t.Errorf("additionalContext = %v", hso["additionalContext"])
	}

	if hso["hookEventName"] != "PreCompact" {
		t.Errorf("hookEventName = %v, want PreCompact", hso["hookEventName"])
	}
}

// TestClaudeCodeFormatPreCompactNil tests passthrough when no context.
func TestClaudeCodeFormatPreCompactNil(t *testing.T) {
	a := &claudeCodeAdapter{}

	out := a.FormatPreCompact(HookResponse{})
	if out != nil {
		t.Errorf("expected nil, got %s", out)
	}
}

// TestClaudeCodeFormatSessionStart tests FormatSessionStart with context.
func TestClaudeCodeFormatSessionStart(t *testing.T) {
	a := &claudeCodeAdapter{}
	out := a.FormatSessionStart(HookResponse{Context: "session context"})

	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	hso, ok := m["hookSpecificOutput"].(map[string]any)
	if !ok {
		t.Fatal("missing hookSpecificOutput")
	}

	if hso["hookEventName"] != "SessionStart" {
		t.Errorf("hookEventName = %v, want SessionStart", hso["hookEventName"])
	}
}

// TestClaudeCodeFormatSessionStartNil tests passthrough when empty.
func TestClaudeCodeFormatSessionStartNil(t *testing.T) {
	a := &claudeCodeAdapter{}

	out := a.FormatSessionStart(HookResponse{})
	if out != nil {
		t.Errorf("expected nil, got %s", out)
	}
}

// TestCursorFormatContext tests Cursor's context decision.
func TestCursorFormatContext(t *testing.T) {
	a := &cursorAdapter{}
	out := a.FormatPreToolUse(HookResponse{
		Decision:          "context",
		AdditionalContext: "cursor context info",
	})

	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if m["additional_context"] != "cursor context info" {
		t.Errorf("additional_context = %v", m["additional_context"])
	}
}

// TestCursorFormatPostToolUseContext tests Cursor's PostToolUse with context.
func TestCursorFormatPostToolUseContext(t *testing.T) {
	a := &cursorAdapter{}
	out := a.FormatPostToolUse(HookResponse{AdditionalContext: "post context"})

	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if m["additional_context"] != "post context" {
		t.Errorf("additional_context = %v", m["additional_context"])
	}
}

// TestCursorFormatPostToolUseEmpty tests Cursor's PostToolUse returns {} when empty.
func TestCursorFormatPostToolUseEmpty(t *testing.T) {
	a := &cursorAdapter{}

	out := a.FormatPostToolUse(HookResponse{})
	if string(out) != "{}" {
		t.Errorf("expected {}, got %s", out)
	}
}

// TestCursorFormatSessionStartContext tests Cursor's SessionStart with context.
func TestCursorFormatSessionStartContext(t *testing.T) {
	a := &cursorAdapter{}
	out := a.FormatSessionStart(HookResponse{Context: "start context"})

	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if m["additional_context"] != "start context" {
		t.Errorf("additional_context = %v", m["additional_context"])
	}
}

// TestCursorFormatSessionStartEmpty tests Cursor's SessionStart returns {} when empty.
func TestCursorFormatSessionStartEmpty(t *testing.T) {
	a := &cursorAdapter{}

	out := a.FormatSessionStart(HookResponse{})
	if string(out) != "{}" {
		t.Errorf("expected {}, got %s", out)
	}
}

// TestCursorSessionIDFromEnv tests the CURSOR_TRACE_ID env fallback.
func TestCursorSessionIDFromEnv(t *testing.T) {
	t.Setenv("CURSOR_TRACE_ID", "trace-789")

	a := &cursorAdapter{}
	raw := json.RawMessage(`{"tool_name": "Read"}`)

	ev := a.ParsePreToolUse(raw)
	if ev.SessionID != "trace-789" {
		t.Errorf("SessionID = %q, want trace-789", ev.SessionID)
	}
}

// TestGeminiCLIFormatContext tests Gemini CLI's context decision.
func TestGeminiCLIFormatContext(t *testing.T) {
	a := &geminiCLIAdapter{}
	out := a.FormatPreToolUse(HookResponse{
		Decision:          "context",
		AdditionalContext: "gemini context",
	})

	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	hso, ok := m["hookSpecificOutput"].(map[string]any)
	if !ok {
		t.Fatal("missing hookSpecificOutput")
	}

	if hso["additionalContext"] != "gemini context" {
		t.Errorf("additionalContext = %v", hso["additionalContext"])
	}
}

// TestGeminiCLIFormatPostToolUseContext tests PostToolUse with context.
func TestGeminiCLIFormatPostToolUseContext(t *testing.T) {
	a := &geminiCLIAdapter{}
	out := a.FormatPostToolUse(HookResponse{AdditionalContext: "post context"})

	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	hso, ok := m["hookSpecificOutput"].(map[string]any)
	if !ok {
		t.Fatal("missing hookSpecificOutput")
	}

	if hso["additionalContext"] != "post context" {
		t.Errorf("additionalContext = %v", hso["additionalContext"])
	}
}

// TestGeminiCLIFormatPostToolUseEmpty tests PostToolUse returns {} when empty.
func TestGeminiCLIFormatPostToolUseEmpty(t *testing.T) {
	a := &geminiCLIAdapter{}

	out := a.FormatPostToolUse(HookResponse{})
	if string(out) != "{}" {
		t.Errorf("expected {}, got %s", out)
	}
}

// TestGeminiCLIFormatPreCompactContext tests PreCompact with context.
func TestGeminiCLIFormatPreCompactContext(t *testing.T) {
	a := &geminiCLIAdapter{}

	out := a.FormatPreCompact(HookResponse{Context: "compress context"})
	if !json.Valid(out) {
		t.Errorf("invalid JSON: %s", out)
	}
}

// TestGeminiCLIFormatSessionStartContext tests SessionStart with context.
func TestGeminiCLIFormatSessionStartContext(t *testing.T) {
	a := &geminiCLIAdapter{}

	out := a.FormatSessionStart(HookResponse{Context: "start context"})
	if !json.Valid(out) {
		t.Errorf("invalid JSON: %s", out)
	}
}

// TestGeminiCLIProjectDirEnv tests projectDir with GEMINI_PROJECT_DIR.
func TestGeminiCLIProjectDirEnv(t *testing.T) {
	t.Setenv("GEMINI_PROJECT_DIR", "/gemini/project")
	t.Setenv("CLAUDE_PROJECT_DIR", "/claude/project")

	a := &geminiCLIAdapter{}
	raw := json.RawMessage(`{"tool_name": "Read"}`)

	ev := a.ParsePreToolUse(raw)
	if ev.ProjectDir != "/gemini/project" {
		t.Errorf("ProjectDir = %q, want /gemini/project", ev.ProjectDir)
	}
}

// TestGeminiCLIProjectDirFallback tests projectDir fallback to CLAUDE_PROJECT_DIR.
func TestGeminiCLIProjectDirFallback(t *testing.T) {
	t.Setenv("GEMINI_PROJECT_DIR", "")
	t.Setenv("CLAUDE_PROJECT_DIR", "/claude/project")

	a := &geminiCLIAdapter{}
	raw := json.RawMessage(`{"tool_name": "Read"}`)

	ev := a.ParsePreToolUse(raw)
	if ev.ProjectDir != "/claude/project" {
		t.Errorf("ProjectDir = %q, want /claude/project", ev.ProjectDir)
	}
}

// TestVSCodeCopilotFormatContext tests VS Code's context decision.
func TestVSCodeCopilotFormatContext(t *testing.T) {
	a := &vscodeCopilotAdapter{}
	out := a.FormatPreToolUse(HookResponse{
		Decision:          "context",
		AdditionalContext: "vscode context",
	})

	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	hso, ok := m["hookSpecificOutput"].(map[string]any)
	if !ok {
		t.Fatal("missing hookSpecificOutput")
	}

	if hso["additionalContext"] != "vscode context" {
		t.Errorf("additionalContext = %v", hso["additionalContext"])
	}

	if hso["hookEventName"] != "PreToolUse" {
		t.Errorf("hookEventName = %v, want PreToolUse", hso["hookEventName"])
	}
}

// TestVSCodeCopilotFormatPostToolUse tests PostToolUse with context.
func TestVSCodeCopilotFormatPostToolUse(t *testing.T) {
	a := &vscodeCopilotAdapter{}
	out := a.FormatPostToolUse(HookResponse{AdditionalContext: "post ctx"})

	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	hso, ok := m["hookSpecificOutput"].(map[string]any)
	if !ok {
		t.Fatal("missing hookSpecificOutput")
	}

	if hso["hookEventName"] != "PostToolUse" {
		t.Errorf("hookEventName = %v, want PostToolUse", hso["hookEventName"])
	}
}

// TestVSCodeCopilotFormatPreCompact tests PreCompact with context.
func TestVSCodeCopilotFormatPreCompact(t *testing.T) {
	a := &vscodeCopilotAdapter{}

	out := a.FormatPreCompact(HookResponse{Context: "compact"})
	if !json.Valid(out) {
		t.Errorf("invalid JSON: %s", out)
	}
}

// TestVSCodeCopilotFormatSessionStart tests SessionStart with context.
func TestVSCodeCopilotFormatSessionStart(t *testing.T) {
	a := &vscodeCopilotAdapter{}

	out := a.FormatSessionStart(HookResponse{Context: "start"})
	if !json.Valid(out) {
		t.Errorf("invalid JSON: %s", out)
	}
}

// TestOpenCodeFormatModify tests the "modify" branch.
func TestOpenCodeFormatModify(t *testing.T) {
	a := &openCodeAdapter{}
	out := a.FormatPreToolUse(HookResponse{
		Decision:     "modify",
		UpdatedInput: map[string]any{"command": "safe cmd"},
	})

	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if m["args"] == nil {
		t.Error("missing args in output")
	}
}

// TestOpenCodeFormatPostToolUseContext tests PostToolUse with context.
func TestOpenCodeFormatPostToolUseContext(t *testing.T) {
	a := &openCodeAdapter{}
	out := a.FormatPostToolUse(HookResponse{AdditionalContext: "oc context"})

	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if m["additionalContext"] != "oc context" {
		t.Errorf("additionalContext = %v", m["additionalContext"])
	}
}

// TestOpenCodeFormatPreCompact tests PreCompact with context.
func TestOpenCodeFormatPreCompact(t *testing.T) {
	a := &openCodeAdapter{}

	out := a.FormatPreCompact(HookResponse{Context: "compact"})
	if !json.Valid(out) {
		t.Errorf("invalid JSON: %s", out)
	}
}

// TestKiroFormatPostToolUse tests Kiro's PostToolUse.
func TestKiroFormatPostToolUse(t *testing.T) {
	a := &kiroAdapter{}
	out := a.FormatPostToolUse(HookResponse{AdditionalContext: "kiro ctx"})

	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if m["stdout"] != "kiro ctx" {
		t.Errorf("stdout = %v, want 'kiro ctx'", m["stdout"])
	}
}

// TestKiroFormatPostToolUseEmpty tests Kiro's PostToolUse without context.
func TestKiroFormatPostToolUseEmpty(t *testing.T) {
	a := &kiroAdapter{}
	out := a.FormatPostToolUse(HookResponse{})

	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	exitCode, ok := m["exitCode"].(float64)
	if !ok || exitCode != 0 {
		t.Errorf("exitCode = %v, want 0", m["exitCode"])
	}
}

// TestKiroParsePostToolUse tests Kiro's PostToolUse parsing.
func TestKiroParsePostToolUse(t *testing.T) {
	a := &kiroAdapter{}
	raw := json.RawMessage(`{
		"tool_name": "read_file",
		"tool_input": {"path": "/tmp/f.go"},
		"tool_output": "file contents",
		"is_error": true,
		"session_id": "s1",
		"cwd": "/project"
	}`)

	ev := a.ParsePostToolUse(raw)
	if ev.ToolName != "read_file" {
		t.Errorf("ToolName = %q", ev.ToolName)
	}

	if ev.ToolOutput != "file contents" {
		t.Errorf("ToolOutput = %q", ev.ToolOutput)
	}

	if !ev.IsError {
		t.Error("IsError should be true")
	}
}

// TestOpenClawFormatDeny tests OpenClaw deny decision.
func TestOpenClawFormatDeny(t *testing.T) {
	a := &openClawAdapter{}
	out := a.FormatPreToolUse(HookResponse{Decision: "deny", Reason: "blocked"})

	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if block, ok := m["block"].(bool); !ok || !block {
		t.Errorf("block = %v, want true", m["block"])
	}
}

// TestOpenClawFormatModify tests OpenClaw modify decision.
func TestOpenClawFormatModify(t *testing.T) {
	a := &openClawAdapter{}
	out := a.FormatPreToolUse(HookResponse{
		Decision:     "modify",
		UpdatedInput: map[string]any{"x": 1},
	})

	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if m["args"] == nil {
		t.Error("missing args")
	}
}

// TestOpenClawFormatPostToolUse tests PostToolUse with and without context.
func TestOpenClawFormatPostToolUse(t *testing.T) {
	a := &openClawAdapter{}
	out := a.FormatPostToolUse(HookResponse{AdditionalContext: "oc ctx"})

	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if m["additionalContext"] != "oc ctx" {
		t.Errorf("additionalContext = %v", m["additionalContext"])
	}

	out2 := a.FormatPostToolUse(HookResponse{})
	if string(out2) != "{}" {
		t.Errorf("expected {}, got %s", out2)
	}
}

// TestOpenClawParsePostToolUseDualCase tests PostToolUse with dual-case fields.
func TestOpenClawParsePostToolUseDualCase(t *testing.T) {
	a := &openClawAdapter{}
	raw := json.RawMessage(`{
		"toolName": "Bash",
		"params": {"command": "ls"},
		"toolOutput": "output",
		"isError": true,
		"sessionId": "s1",
		"projectDir": "/p"
	}`)

	ev := a.ParsePostToolUse(raw)
	if ev.ToolOutput != "output" {
		t.Errorf("ToolOutput = %q", ev.ToolOutput)
	}

	if !ev.IsError {
		t.Error("IsError should be true")
	}
}

// TestOpenClawCapabilities tests OpenClaw capabilities.
func TestOpenClawCapabilities(t *testing.T) {
	a := &openClawAdapter{}
	caps := a.Capabilities()
	assertCap(t, "PreToolUse", caps.PreToolUse, true)
	assertCap(t, "PostToolUse", caps.PostToolUse, true)
	assertCap(t, "PreCompact", caps.PreCompact, false)
	assertCap(t, "SessionStart", caps.SessionStart, false)
	assertCap(t, "CanModifyArgs", caps.CanModifyArgs, true)
	assertCap(t, "CanModifyOutput", caps.CanModifyOutput, true)
}

// TestKiroCapabilities tests Kiro capabilities.
func TestKiroCapabilities(t *testing.T) {
	a := &kiroAdapter{}
	caps := a.Capabilities()
	assertCap(t, "PreToolUse", caps.PreToolUse, true)
	assertCap(t, "PostToolUse", caps.PostToolUse, true)
	assertCap(t, "PreCompact", caps.PreCompact, false)
	assertCap(t, "SessionStart", caps.SessionStart, false)
}

// TestAntigravityCapabilities tests Antigravity capabilities.
func TestAntigravityCapabilities(t *testing.T) {
	a := &antigravityAdapter{}
	caps := a.Capabilities()
	assertCap(t, "PreToolUse", caps.PreToolUse, false)
	assertCap(t, "PostToolUse", caps.PostToolUse, false)
	assertCap(t, "PreCompact", caps.PreCompact, false)
	assertCap(t, "SessionStart", caps.SessionStart, false)
}

// TestAntigravityValidateHooks tests Antigravity returns N/A diagnostic.
func TestAntigravityValidateHooks(t *testing.T) {
	a := &antigravityAdapter{}

	diags := a.ValidateHooks()
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(diags))
	}

	if !hasPrefix(diags[0], "settings file exists: N/A") {
		t.Errorf("diag = %q, want N/A", diags[0])
	}
}

// TestEnsureDir tests ensureDir creates directories.
func TestEnsureDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "sub", "dir")

	result := ensureDir(dir)
	if result != dir {
		t.Errorf("ensureDir returned %q, want %q", result, dir)
	}

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("directory not created: %v", err)
	}

	if !info.IsDir() {
		t.Error("expected directory")
	}
}

// TestExtractBoolMissing tests extractBool with missing key.
func TestExtractBoolMissing(t *testing.T) {
	m := map[string]any{"other": "value"}
	if extractBool(m, "missing") {
		t.Error("expected false for missing key")
	}
}

// TestExtractBoolWrongType tests extractBool with non-bool value.
func TestExtractBoolWrongType(t *testing.T) {
	m := map[string]any{"key": "not-a-bool"}
	if extractBool(m, "key") {
		t.Error("expected false for non-bool value")
	}
}

// TestExtractBoolTrue tests extractBool with true value.
func TestExtractBoolTrue(t *testing.T) {
	m := map[string]any{"key": true}
	if !extractBool(m, "key") {
		t.Error("expected true")
	}
}

// TestParseRawToMapInvalidJSON tests parseRawToMap with invalid JSON.
func TestParseRawToMapInvalidJSON(t *testing.T) {
	m := parseRawToMap(json.RawMessage(`not-json`))
	if m == nil {
		t.Fatal("expected non-nil map")
	}

	if len(m) != 0 {
		t.Errorf("expected empty map, got %v", m)
	}
}

// TestExtractBoolDualCaseSnakeFallback tests the snake_case fallback path.
func TestExtractBoolDualCaseSnakeFallback(t *testing.T) {
	m := map[string]any{"is_error": true}
	if !extractBoolDualCase(m, "isError", "is_error") {
		t.Error("expected true from snake_case fallback")
	}
}

// TestGetUnknownPlatformID tests Get with PlatformUnknown.
func TestGetUnknownPlatformID(t *testing.T) {
	_, err := Get(PlatformUnknown)
	if err == nil {
		t.Error("expected error for PlatformUnknown")
	}
}
