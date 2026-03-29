package mcp

import (
	"testing"

	"github.com/inovacc/thimble/internal/platform"
)

func TestDetectPlatformFromClientInfoKnown(t *testing.T) {
	tests := []struct {
		name string
		want platform.PlatformID
	}{
		{"claude-code", platform.PlatformClaudeCode},
		{"gemini-cli-mcp-client", platform.PlatformGeminiCLI},
		{"cursor-vscode", platform.PlatformCursor},
		{"Codex", platform.PlatformCodex},
		{"Kiro CLI", platform.PlatformKiro},
	}
	for _, tt := range tests {
		got := DetectPlatformFromClientInfo(tt.name)
		if got != tt.want {
			t.Errorf("DetectPlatformFromClientInfo(%q) = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestDetectPlatformFromClientInfoCaseInsensitive(t *testing.T) {
	got := DetectPlatformFromClientInfo("CLAUDE-CODE")
	if got != platform.PlatformClaudeCode {
		t.Errorf("case-insensitive failed: got %q", got)
	}
}

func TestDetectPlatformFromClientInfoUnknownFallback(t *testing.T) {
	got := DetectPlatformFromClientInfo("some-random-client")
	if got != platform.PlatformClaudeCode {
		t.Errorf("unknown should fallback to ClaudeCode, got %q", got)
	}
}

func TestDetectPlatformFromClientInfoEmpty(t *testing.T) {
	got := DetectPlatformFromClientInfo("")
	if got != platform.PlatformClaudeCode {
		t.Errorf("empty should fallback to ClaudeCode, got %q", got)
	}
}

func TestDetectPlatformFromClientInfoSubstringMatch(t *testing.T) {
	// "my-opencode-fork" contains "opencode" as a substring but is not an exact match.
	got := DetectPlatformFromClientInfo("my-opencode-fork")
	if got != platform.PlatformOpenCode {
		t.Errorf("substring match for opencode failed: got %q, want %q", got, platform.PlatformOpenCode)
	}
}

func TestDetectPlatformFromClientInfoSubstringMatchKiro(t *testing.T) {
	got := DetectPlatformFromClientInfo("Kiro CLI v2.0")
	if got != platform.PlatformKiro {
		t.Errorf("substring match for kiro failed: got %q, want %q", got, platform.PlatformKiro)
	}
}
