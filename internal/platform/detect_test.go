package platform

import (
	"testing"
)

func TestDetectEnvVars(t *testing.T) {
	tests := []struct {
		name     string
		envKey   string
		envValue string
		wantID   PlatformID
	}{
		{
			name:     "CLAUDE_PROJECT_DIR",
			envKey:   "CLAUDE_PROJECT_DIR",
			envValue: "/some/path",
			wantID:   PlatformClaudeCode,
		},
		{
			name:     "CLAUDE_SESSION_ID",
			envKey:   "CLAUDE_SESSION_ID",
			envValue: "sess-abc",
			wantID:   PlatformClaudeCode,
		},
		{
			name:     "GEMINI_PROJECT_DIR",
			envKey:   "GEMINI_PROJECT_DIR",
			envValue: "/gemini/path",
			wantID:   PlatformGeminiCLI,
		},
		{
			name:     "GEMINI_CLI",
			envKey:   "GEMINI_CLI",
			envValue: "true",
			wantID:   PlatformGeminiCLI,
		},
		{
			name:     "OPENCODE",
			envKey:   "OPENCODE",
			envValue: "1",
			wantID:   PlatformOpenCode,
		},
		{
			name:     "OPENCODE_PID",
			envKey:   "OPENCODE_PID",
			envValue: "12345",
			wantID:   PlatformOpenCode,
		},
		{
			name:     "CODEX_CI",
			envKey:   "CODEX_CI",
			envValue: "true",
			wantID:   PlatformCodex,
		},
		{
			name:     "CODEX_THREAD_ID",
			envKey:   "CODEX_THREAD_ID",
			envValue: "thread-1",
			wantID:   PlatformCodex,
		},
		{
			name:     "CURSOR_TRACE_ID",
			envKey:   "CURSOR_TRACE_ID",
			envValue: "trace-1",
			wantID:   PlatformCursor,
		},
		{
			name:     "CURSOR_CLI",
			envKey:   "CURSOR_CLI",
			envValue: "1",
			wantID:   PlatformCursor,
		},
		{
			name:     "VSCODE_PID",
			envKey:   "VSCODE_PID",
			envValue: "9999",
			wantID:   PlatformVSCodeCopilot,
		},
		{
			name:     "VSCODE_CWD",
			envKey:   "VSCODE_CWD",
			envValue: "/vscode/cwd",
			wantID:   PlatformVSCodeCopilot,
		},
		{
			name:     "KIRO_PROJECT_DIR",
			envKey:   "KIRO_PROJECT_DIR",
			envValue: "/kiro/project",
			wantID:   PlatformKiro,
		},
		{
			name:     "ANTIGRAVITY_SESSION",
			envKey:   "ANTIGRAVITY_SESSION",
			envValue: "ag-session",
			wantID:   PlatformAntigravity,
		},
	}

	// All env vars that could interfere with detection.
	allEnvKeys := []string{
		"THIMBLE_PLATFORM",
		"CLAUDE_PROJECT_DIR", "CLAUDE_SESSION_ID",
		"GEMINI_PROJECT_DIR", "GEMINI_CLI",
		"OPENCODE", "OPENCODE_PID",
		"CODEX_CI", "CODEX_THREAD_ID",
		"CURSOR_TRACE_ID", "CURSOR_CLI",
		"VSCODE_PID", "VSCODE_CWD",
		"KIRO_PROJECT_DIR",
		"ANTIGRAVITY_SESSION",
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear all detection env vars.
			for _, k := range allEnvKeys {
				t.Setenv(k, "")
			}
			// Set the one we want to test.
			t.Setenv(tt.envKey, tt.envValue)

			signal := Detect()
			if signal.Platform != tt.wantID {
				t.Errorf("Detect() platform = %s, want %s", signal.Platform, tt.wantID)
			}

			if signal.Confidence != ConfidenceHigh {
				t.Errorf("Detect() confidence = %s, want high", signal.Confidence)
			}
		})
	}
}

func TestDetectFallback(t *testing.T) {
	// Clear all detection env vars to force fallback.
	allEnvKeys := []string{
		"THIMBLE_PLATFORM",
		"CLAUDE_PROJECT_DIR", "CLAUDE_SESSION_ID",
		"GEMINI_PROJECT_DIR", "GEMINI_CLI",
		"OPENCODE", "OPENCODE_PID",
		"CODEX_CI", "CODEX_THREAD_ID",
		"CURSOR_TRACE_ID", "CURSOR_CLI",
		"VSCODE_PID", "VSCODE_CWD",
		"KIRO_PROJECT_DIR",
		"ANTIGRAVITY_SESSION",
	}
	for _, k := range allEnvKeys {
		t.Setenv(k, "")
	}

	signal := Detect()
	// Should return some platform (either medium from config dir or low fallback).
	if signal.Platform == "" {
		t.Error("expected non-empty platform")
	}

	if signal.Reason == "" {
		t.Error("expected non-empty reason")
	}
}

func TestDetectThimblePlatformOverride(t *testing.T) {
	allEnvKeys := []string{
		"THIMBLE_PLATFORM",
		"CLAUDE_PROJECT_DIR", "CLAUDE_SESSION_ID",
		"GEMINI_PROJECT_DIR", "GEMINI_CLI",
		"OPENCODE", "OPENCODE_PID",
		"CODEX_CI", "CODEX_THREAD_ID",
		"CURSOR_TRACE_ID", "CURSOR_CLI",
		"VSCODE_PID", "VSCODE_CWD",
		"KIRO_PROJECT_DIR",
		"ANTIGRAVITY_SESSION",
	}
	for _, k := range allEnvKeys {
		t.Setenv(k, "")
	}

	t.Setenv("THIMBLE_PLATFORM", "gemini-cli")

	signal := Detect()
	if signal.Platform != PlatformGeminiCLI {
		t.Errorf("Detect() platform = %s, want %s", signal.Platform, PlatformGeminiCLI)
	}

	if signal.Confidence != ConfidenceHigh {
		t.Errorf("Detect() confidence = %s, want high", signal.Confidence)
	}

	if signal.Reason != "THIMBLE_PLATFORM override" {
		t.Errorf("Detect() reason = %q, want %q", signal.Reason, "THIMBLE_PLATFORM override")
	}
}

func TestDetectThimblePlatformInvalid(t *testing.T) {
	allEnvKeys := []string{
		"THIMBLE_PLATFORM",
		"CLAUDE_PROJECT_DIR", "CLAUDE_SESSION_ID",
		"GEMINI_PROJECT_DIR", "GEMINI_CLI",
		"OPENCODE", "OPENCODE_PID",
		"CODEX_CI", "CODEX_THREAD_ID",
		"CURSOR_TRACE_ID", "CURSOR_CLI",
		"VSCODE_PID", "VSCODE_CWD",
		"KIRO_PROJECT_DIR",
		"ANTIGRAVITY_SESSION",
	}
	for _, k := range allEnvKeys {
		t.Setenv(k, "")
	}

	t.Setenv("THIMBLE_PLATFORM", "nonexistent")

	signal := Detect()
	// Should fall through to other detection (not crash, not return "nonexistent").
	if signal.Platform == PlatformID("nonexistent") {
		t.Error("invalid THIMBLE_PLATFORM should not be returned as platform")
	}

	if signal.Platform == "" {
		t.Error("expected non-empty platform after invalid THIMBLE_PLATFORM")
	}
}
