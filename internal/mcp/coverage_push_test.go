package mcp

import (
	"testing"

	"github.com/inovacc/thimble/internal/platform"
)

func TestSessionHashDifferentSeparator(t *testing.T) {
	// Verify that app path and project dir don't collide when concatenated.
	// "a\x00bc" != "ab\x00c"
	h1 := sessionHash("a", "bc")
	h2 := sessionHash("ab", "c")

	if h1 == h2 {
		t.Error("sessionHash should use null separator to avoid collisions")
	}
}

func TestSessionHashEmptyInputs(t *testing.T) {
	h := sessionHash("", "")
	if len(h) != 16 {
		t.Errorf("sessionHash with empty inputs should still return 16 chars, got %d", len(h))
	}
}

func TestNewSessionStatsInitialValues(t *testing.T) {
	s := newSessionStats()

	if s.bytesIndexed != 0 {
		t.Errorf("bytesIndexed = %d, want 0", s.bytesIndexed)
	}

	if s.bytesSandboxed != 0 {
		t.Errorf("bytesSandboxed = %d, want 0", s.bytesSandboxed)
	}

	if len(s.calls) != 0 {
		t.Errorf("calls should be empty, got %d entries", len(s.calls))
	}

	if len(s.bytesReturned) != 0 {
		t.Errorf("bytesReturned should be empty, got %d entries", len(s.bytesReturned))
	}
}

func TestSessionStatsSnapshotEmpty(t *testing.T) {
	s := newSessionStats()

	snap := s.snapshot()

	calls, ok := snap["calls"].(map[string]int)
	if !ok {
		t.Fatalf("calls type = %T, want map[string]int", snap["calls"])
	}

	if len(calls) != 0 {
		t.Errorf("empty snapshot calls should have 0 entries, got %d", len(calls))
	}

	bytesReturned, ok := snap["bytesReturned"].(map[string]int)
	if !ok {
		t.Fatalf("bytesReturned type = %T, want map[string]int", snap["bytesReturned"])
	}

	if len(bytesReturned) != 0 {
		t.Errorf("empty snapshot bytesReturned should have 0 entries, got %d", len(bytesReturned))
	}

	if snap["bytesIndexed"] != 0 {
		t.Errorf("bytesIndexed = %v, want 0", snap["bytesIndexed"])
	}
}

func TestSessionStatsSnapshotMultipleTools(t *testing.T) {
	s := newSessionStats()

	s.mu.Lock()
	s.calls["ctx_search"] = 10
	s.calls["ctx_index"] = 5
	s.calls["ctx_execute"] = 3
	s.bytesReturned["ctx_search"] = 4096
	s.bytesReturned["ctx_index"] = 2048
	s.bytesIndexed = 8192
	s.bytesSandboxed = 1024
	s.mu.Unlock()

	snap := s.snapshot()

	calls := snap["calls"].(map[string]int)
	if calls["ctx_search"] != 10 || calls["ctx_index"] != 5 || calls["ctx_execute"] != 3 {
		t.Errorf("unexpected calls: %v", calls)
	}

	br := snap["bytesReturned"].(map[string]int)
	if br["ctx_search"] != 4096 || br["ctx_index"] != 2048 {
		t.Errorf("unexpected bytesReturned: %v", br)
	}
}

func TestDetectPlatformFromClientInfoAllKnownPlatforms(t *testing.T) {
	tests := []struct {
		input string
		want  platform.PlatformID
	}{
		{"claude-code", platform.PlatformClaudeCode},
		{"gemini-cli-mcp-client", platform.PlatformGeminiCLI},
		{"antigravity-client", platform.PlatformAntigravity},
		{"cursor-vscode", platform.PlatformCursor},
		{"visual-studio-code", platform.PlatformVSCodeCopilot},
		{"codex", platform.PlatformCodex},
		{"codex-mcp-client", platform.PlatformCodex},
		{"kiro cli", platform.PlatformKiro},
		{"kiro", platform.PlatformKiro},
		{"opencode", platform.PlatformOpenCode},
		{"openclaw", platform.PlatformOpenClaw},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := DetectPlatformFromClientInfo(tt.input)
			if got != tt.want {
				t.Errorf("DetectPlatformFromClientInfo(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestDetectPlatformFromClientInfoWhitespace(t *testing.T) {
	got := DetectPlatformFromClientInfo("  claude-code  ")
	if got != platform.PlatformClaudeCode {
		t.Errorf("whitespace-padded input should match, got %q", got)
	}
}

func TestDetectPlatformFromClientInfoMixedCase(t *testing.T) {
	tests := []struct {
		input string
		want  platform.PlatformID
	}{
		{"Claude-Code", platform.PlatformClaudeCode},
		{"GEMINI-CLI-MCP-CLIENT", platform.PlatformGeminiCLI},
		{"Cursor-VSCode", platform.PlatformCursor},
		{"OpenCode", platform.PlatformOpenCode},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := DetectPlatformFromClientInfo(tt.input)
			if got != tt.want {
				t.Errorf("DetectPlatformFromClientInfo(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestDetectPlatformFromClientInfoSubstringEdgeCases(t *testing.T) {
	// "my-cursor-vscode-extension" contains "cursor-vscode" as substring.
	got := DetectPlatformFromClientInfo("my-cursor-vscode-extension")
	if got != platform.PlatformCursor {
		t.Errorf("substring match for cursor-vscode failed: got %q", got)
	}
}

func TestCoerceJSONArrayNestedJSON(t *testing.T) {
	// A JSON string that contains nested JSON arrays.
	input := `["a","b","c"]`

	result := coerceJSONArray(input)
	if len(result) != 3 {
		t.Fatalf("expected 3 elements, got %d", len(result))
	}

	if result[0] != "a" || result[1] != "b" || result[2] != "c" {
		t.Errorf("unexpected values: %v", result)
	}
}

func TestCoerceJSONArraySingleElement(t *testing.T) {
	input := []any{"only"}

	result := coerceJSONArray(input)
	if len(result) != 1 || result[0] != "only" {
		t.Errorf("expected [only], got %v", result)
	}
}

func TestCoerceJSONArrayBoolInput(t *testing.T) {
	result := coerceJSONArray(true)
	if result != nil {
		t.Errorf("expected nil for bool input, got %v", result)
	}
}

func TestCoerceJSONArrayMapInput(t *testing.T) {
	result := coerceJSONArray(map[string]any{"key": "val"})
	if result != nil {
		t.Errorf("expected nil for map input, got %v", result)
	}
}

func TestCoerceJSONArrayEmptyString(t *testing.T) {
	result := coerceJSONArray("")
	if result != nil {
		t.Errorf("expected nil for empty string, got %v", result)
	}
}

func TestCoerceJSONArrayEmptyJSONArray(t *testing.T) {
	result := coerceJSONArray("[]")
	if len(result) != 0 {
		t.Errorf("expected 0 elements for empty JSON array, got %d", len(result))
	}
}
