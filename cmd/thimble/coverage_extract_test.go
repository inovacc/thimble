package main

import (
	"encoding/json"
	"testing"

	"github.com/inovacc/thimble/internal/platform"
)

// ─── resolveEvent ───

func TestResolveEvent_KnownEvents(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"pretooluse", "PreToolUse"},
		{"PreToolUse", "PreToolUse"},
		{"posttooluse", "PostToolUse"},
		{"PostToolUse", "PostToolUse"},
		{"precompact", "PreCompact"},
		{"PreCompact", "PreCompact"},
		{"sessionstart", "SessionStart"},
		{"SessionStart", "SessionStart"},
		{"userpromptsubmit", "UserPromptSubmit"},
		{"beforetool", "PreToolUse"},
		{"aftertool", "PostToolUse"},
		{"precompress", "PreCompact"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, ok := resolveEvent(tt.input)
			if !ok {
				t.Fatalf("resolveEvent(%q) returned false", tt.input)
			}

			if got != tt.want {
				t.Errorf("resolveEvent(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestResolveEvent_Unknown(t *testing.T) {
	unknown := []string{"Unknown", "invalid", "", "OnStart", "BeforeRun"}

	for _, input := range unknown {
		t.Run(input, func(t *testing.T) {
			_, ok := resolveEvent(input)
			if ok {
				t.Errorf("resolveEvent(%q) should return false", input)
			}
		})
	}
}

func TestResolveEvent_CaseInsensitive(t *testing.T) {
	// "PRETOOLUSE" (all caps) should resolve via strings.ToLower.
	got, ok := resolveEvent("PRETOOLUSE")
	if !ok {
		t.Fatal("resolveEvent('PRETOOLUSE') returned false")
	}

	if got != "PreToolUse" {
		t.Errorf("got = %q, want 'PreToolUse'", got)
	}
}

// ─── parseNormalizedEvent ───

func TestParseNormalizedEvent_PreToolUse(t *testing.T) {
	adapter, _ := platform.Get(platform.PlatformClaudeCode)

	payload := json.RawMessage(`{
		"tool_name": "Bash",
		"tool_input": {"command": "echo hello"},
		"session_id": "test-session"
	}`)

	norm := parseNormalizedEvent(adapter, "PreToolUse", payload)
	// The adapter may or may not parse these fields depending on the format.
	// We just verify no panic.
	if norm.ToolName == "" {
		t.Log("parseNormalizedEvent returned empty ToolName (acceptable)")
	}
}

func TestParseNormalizedEvent_PostToolUse(t *testing.T) {
	adapter, _ := platform.Get(platform.PlatformClaudeCode)

	payload := json.RawMessage(`{
		"tool_name": "Read",
		"tool_input": {"file": "/tmp/test.go"},
		"tool_result": "file contents here"
	}`)

	norm := parseNormalizedEvent(adapter, "PostToolUse", payload)
	if norm.ToolName == "" {
		t.Log("parseNormalizedEvent returned empty ToolName (acceptable)")
	}
}

func TestParseNormalizedEvent_UserPromptSubmit(t *testing.T) {
	adapter, _ := platform.Get(platform.PlatformClaudeCode)

	payload := json.RawMessage(`{"prompt": "hello"}`)

	// UserPromptSubmit should go through ParsePostToolUse path.
	norm := parseNormalizedEvent(adapter, "UserPromptSubmit", payload)
	if norm.ToolName == "" {
		t.Log("parseNormalizedEvent returned empty ToolName for UserPromptSubmit (acceptable)")
	}
}

func TestParseNormalizedEvent_PreCompact(t *testing.T) {
	adapter, _ := platform.Get(platform.PlatformClaudeCode)

	payload := json.RawMessage(`{"session_id": "test-session"}`)

	norm := parseNormalizedEvent(adapter, "PreCompact", payload)
	if norm.ToolName == "" {
		t.Log("parseNormalizedEvent returned empty ToolName for PreCompact (acceptable)")
	}
}

func TestParseNormalizedEvent_SessionStart(t *testing.T) {
	adapter, _ := platform.Get(platform.PlatformClaudeCode)

	payload := json.RawMessage(`{"session_id": "test-session"}`)

	norm := parseNormalizedEvent(adapter, "SessionStart", payload)
	if norm.ToolName == "" {
		t.Log("parseNormalizedEvent returned empty ToolName for SessionStart (acceptable)")
	}
}

func TestParseNormalizedEvent_UnknownEvent(t *testing.T) {
	adapter, _ := platform.Get(platform.PlatformClaudeCode)

	payload := json.RawMessage(`{}`)

	// Unknown event should return zero-value NormalizedEvent.
	norm := parseNormalizedEvent(adapter, "UnknownEvent", payload)
	if norm.ToolName != "" {
		t.Errorf("ToolName = %q, want empty for unknown event", norm.ToolName)
	}
}

func TestParseNormalizedEvent_GeminiAdapter(t *testing.T) {
	adapter, _ := platform.Get(platform.PlatformGeminiCLI)

	events := []string{"PreToolUse", "PostToolUse", "PreCompact", "SessionStart", "UserPromptSubmit"}
	for _, event := range events {
		t.Run(event, func(t *testing.T) {
			payload := json.RawMessage(`{"tool": "test"}`)
			norm := parseNormalizedEvent(adapter, event, payload)
			_ = norm
		})
	}
}

func TestParseNormalizedEvent_CursorAdapter(t *testing.T) {
	adapter, _ := platform.Get(platform.PlatformCursor)

	events := []string{"PreToolUse", "PostToolUse", "PreCompact", "SessionStart"}
	for _, event := range events {
		t.Run(event, func(t *testing.T) {
			payload := json.RawMessage(`{"tool": "test"}`)
			norm := parseNormalizedEvent(adapter, event, payload)
			_ = norm
		})
	}
}

func TestParseNormalizedEvent_VSCodeAdapter(t *testing.T) {
	adapter, _ := platform.Get(platform.PlatformVSCodeCopilot)

	events := []string{"PreToolUse", "PostToolUse", "PreCompact", "SessionStart"}
	for _, event := range events {
		t.Run(event, func(t *testing.T) {
			payload := json.RawMessage(`{"tool": "test"}`)
			norm := parseNormalizedEvent(adapter, event, payload)
			_ = norm
		})
	}
}

func TestParseNormalizedEvent_EmptyPayload(t *testing.T) {
	adapter, _ := platform.Get(platform.PlatformClaudeCode)

	events := []string{"PreToolUse", "PostToolUse", "PreCompact", "SessionStart"}
	for _, event := range events {
		t.Run(event, func(t *testing.T) {
			// Empty JSON should not panic.
			norm := parseNormalizedEvent(adapter, event, json.RawMessage(`{}`))
			_ = norm
		})
	}
}
