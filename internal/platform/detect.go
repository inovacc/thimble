package platform

import (
	"os"
	"path/filepath"
)

// Confidence of platform detection.
type Confidence string

const (
	ConfidenceHigh   Confidence = "high"
	ConfidenceMedium Confidence = "medium"
	ConfidenceLow    Confidence = "low"
)

// DetectionSignal represents a detected platform.
type DetectionSignal struct {
	Platform   PlatformID
	Confidence Confidence
	Reason     string
}

// Detect auto-detects the current platform by checking env vars and config dirs.
// Priority: THIMBLE_PLATFORM override > env vars (high) > config dirs (medium) > fallback to Claude Code (low).
func Detect() DetectionSignal {
	// Highest priority: explicit THIMBLE_PLATFORM override.
	if p := os.Getenv("THIMBLE_PLATFORM"); p != "" {
		if _, err := Get(PlatformID(p)); err == nil {
			return DetectionSignal{PlatformID(p), ConfidenceHigh, "THIMBLE_PLATFORM override"}
		}
	}

	// High confidence: environment variables.
	if os.Getenv("CLAUDE_PROJECT_DIR") != "" || os.Getenv("CLAUDE_SESSION_ID") != "" {
		return DetectionSignal{PlatformClaudeCode, ConfidenceHigh, "CLAUDE_PROJECT_DIR or CLAUDE_SESSION_ID set"}
	}

	if os.Getenv("GEMINI_PROJECT_DIR") != "" || os.Getenv("GEMINI_CLI") != "" {
		return DetectionSignal{PlatformGeminiCLI, ConfidenceHigh, "GEMINI_PROJECT_DIR or GEMINI_CLI set"}
	}

	if os.Getenv("OPENCODE") != "" || os.Getenv("OPENCODE_PID") != "" {
		return DetectionSignal{PlatformOpenCode, ConfidenceHigh, "OPENCODE or OPENCODE_PID set"}
	}

	if os.Getenv("CODEX_CI") != "" || os.Getenv("CODEX_THREAD_ID") != "" {
		return DetectionSignal{PlatformCodex, ConfidenceHigh, "CODEX_CI or CODEX_THREAD_ID set"}
	}

	if os.Getenv("CURSOR_TRACE_ID") != "" || os.Getenv("CURSOR_CLI") != "" {
		return DetectionSignal{PlatformCursor, ConfidenceHigh, "CURSOR_TRACE_ID or CURSOR_CLI set"}
	}

	if os.Getenv("VSCODE_PID") != "" || os.Getenv("VSCODE_CWD") != "" {
		return DetectionSignal{PlatformVSCodeCopilot, ConfidenceHigh, "VSCODE_PID or VSCODE_CWD set"}
	}

	if os.Getenv("KIRO_PROJECT_DIR") != "" {
		return DetectionSignal{PlatformKiro, ConfidenceHigh, "KIRO_PROJECT_DIR set"}
	}

	if os.Getenv("ANTIGRAVITY_SESSION") != "" {
		return DetectionSignal{PlatformAntigravity, ConfidenceHigh, "ANTIGRAVITY_SESSION set"}
	}

	// Medium confidence: config directory existence.
	home := homeDir()

	checks := []struct {
		dir      string
		platform PlatformID
		reason   string
	}{
		{filepath.Join(home, ".claude"), PlatformClaudeCode, "~/.claude/ directory exists"},
		{filepath.Join(home, ".gemini"), PlatformGeminiCLI, "~/.gemini/ directory exists"},
		{filepath.Join(home, ".codex"), PlatformCodex, "~/.codex/ directory exists"},
		{filepath.Join(home, ".cursor"), PlatformCursor, "~/.cursor/ directory exists"},
		{filepath.Join(home, ".config", "opencode"), PlatformOpenCode, "~/.config/opencode/ directory exists"},
	}
	for _, c := range checks {
		if info, err := os.Stat(c.dir); err == nil && info.IsDir() {
			return DetectionSignal{c.platform, ConfidenceMedium, c.reason}
		}
	}

	// Low confidence: fallback to Claude Code (most common).
	return DetectionSignal{PlatformClaudeCode, ConfidenceLow, "no platform detected, defaulting to Claude Code"}
}
