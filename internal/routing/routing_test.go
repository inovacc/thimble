package routing

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/inovacc/thimble/internal/platform"
)

func TestGetConfigClaudeCode(t *testing.T) {
	cfg := GetConfig(platform.PlatformClaudeCode)
	if cfg.FileName != "CLAUDE.md" {
		t.Errorf("FileName = %q, want CLAUDE.md", cfg.FileName)
	}

	if cfg.ProjectRelativePath != "CLAUDE.md" {
		t.Errorf("ProjectRelativePath = %q, want CLAUDE.md", cfg.ProjectRelativePath)
	}

	if !strings.Contains(cfg.GlobalPath, ".claude") {
		t.Errorf("GlobalPath = %q, expected to contain .claude", cfg.GlobalPath)
	}
}

func TestGetConfigGeminiCLI(t *testing.T) {
	cfg := GetConfig(platform.PlatformGeminiCLI)
	if cfg.FileName != "GEMINI.md" {
		t.Errorf("FileName = %q, want GEMINI.md", cfg.FileName)
	}

	if !strings.Contains(cfg.GlobalPath, ".gemini") {
		t.Errorf("GlobalPath = %q, expected to contain .gemini", cfg.GlobalPath)
	}
}

func TestGetConfigVSCodeCopilot(t *testing.T) {
	cfg := GetConfig(platform.PlatformVSCodeCopilot)
	if cfg.FileName != "copilot-instructions.md" {
		t.Errorf("FileName = %q, want copilot-instructions.md", cfg.FileName)
	}

	if cfg.ProjectRelativePath != filepath.Join(".github", "copilot-instructions.md") {
		t.Errorf("ProjectRelativePath = %q, unexpected", cfg.ProjectRelativePath)
	}
}

func TestGetConfigCursor(t *testing.T) {
	cfg := GetConfig(platform.PlatformCursor)
	if cfg.FileName != ".cursorrules" {
		t.Errorf("FileName = %q, want .cursorrules", cfg.FileName)
	}
}

func TestGetConfigOpenCode(t *testing.T) {
	cfg := GetConfig(platform.PlatformOpenCode)
	if cfg.FileName != "AGENTS.md" {
		t.Errorf("FileName = %q, want AGENTS.md", cfg.FileName)
	}

	if cfg.GlobalPath != "" {
		t.Errorf("GlobalPath = %q, want empty for opencode", cfg.GlobalPath)
	}
}

func TestGetConfigDefault(t *testing.T) {
	cfg := GetConfig(platform.PlatformID("unknown"))
	if cfg.FileName != "CLAUDE.md" {
		t.Errorf("default FileName = %q, want CLAUDE.md", cfg.FileName)
	}
}

func TestGenerateInstructionsContainsToolSelection(t *testing.T) {
	platforms := []platform.PlatformID{
		platform.PlatformClaudeCode,
		platform.PlatformGeminiCLI,
		platform.PlatformVSCodeCopilot,
		platform.PlatformCursor,
		platform.PlatformOpenCode,
	}
	for _, p := range platforms {
		content := GenerateInstructions(p)
		if !strings.Contains(content, "context_window_protection") {
			t.Errorf("%s: missing context_window_protection block", p)
		}

		if !strings.Contains(content, "tool_selection_hierarchy") {
			t.Errorf("%s: missing tool_selection_hierarchy section", p)
		}

		if !strings.Contains(content, "batch_execute") {
			t.Errorf("%s: missing batch_execute reference", p)
		}

		if !strings.Contains(content, "forbidden_actions") {
			t.Errorf("%s: missing forbidden_actions section", p)
		}
	}
}

func TestGenerateInstructionsClaudeCodeSpecific(t *testing.T) {
	content := GenerateInstructions(platform.PlatformClaudeCode)
	if !strings.Contains(content, "ctx stats") {
		t.Error("Claude Code instructions should mention ctx stats")
	}

	if !strings.Contains(content, "ctx doctor") {
		t.Error("Claude Code instructions should mention ctx doctor")
	}

	if !strings.Contains(content, "ctx upgrade") {
		t.Error("Claude Code instructions should mention ctx upgrade")
	}
}

func TestGenerateInstructionsGeminiSpecific(t *testing.T) {
	content := GenerateInstructions(platform.PlatformGeminiCLI)
	if !strings.Contains(content, "ctx stats") {
		t.Error("Gemini instructions should mention ctx stats")
	}
	// Gemini doesn't have upgrade command.
	if strings.Contains(content, "ctx upgrade") {
		t.Error("Gemini instructions should not mention ctx upgrade")
	}
}

func TestWriteInstructionsNewFile(t *testing.T) {
	dir := t.TempDir()

	path, err := WriteInstructions(dir, platform.PlatformClaudeCode)
	if err != nil {
		t.Fatalf("WriteInstructions error: %v", err)
	}

	if path == "" {
		t.Fatal("expected non-empty path")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}

	if !strings.Contains(string(data), "context_window_protection") {
		t.Error("written file should contain context_window_protection block")
	}
}

func TestWriteInstructionsAlreadyPresent(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "CLAUDE.md")
	_ = os.WriteFile(target, []byte("# Project\n\n<context_window_protection>\nExisting routing.\n</context_window_protection>\n"), 0o644)

	path, err := WriteInstructions(dir, platform.PlatformClaudeCode)
	if err != nil {
		t.Fatalf("WriteInstructions error: %v", err)
	}

	if path != "" {
		t.Error("expected empty path when routing instructions already present")
	}
}

func TestWriteInstructionsAppendsToExisting(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "CLAUDE.md")
	existing := "# My Project\n\nExisting content.\n"
	_ = os.WriteFile(target, []byte(existing), 0o644)

	path, err := WriteInstructions(dir, platform.PlatformClaudeCode)
	if err != nil {
		t.Fatalf("WriteInstructions error: %v", err)
	}

	if path == "" {
		t.Fatal("expected non-empty path when appending")
	}

	data, _ := os.ReadFile(path)

	content := string(data)
	if !strings.HasPrefix(content, "# My Project") {
		t.Error("should preserve existing content at start")
	}

	if !strings.Contains(content, "context_window_protection") {
		t.Error("should contain appended thimble routing instructions")
	}
}

func TestWriteInstructionsSubdirectory(t *testing.T) {
	dir := t.TempDir()
	// VS Code Copilot writes to .github/copilot-instructions.md — should create subdir.
	path, err := WriteInstructions(dir, platform.PlatformVSCodeCopilot)
	if err != nil {
		t.Fatalf("WriteInstructions error: %v", err)
	}

	if path == "" {
		t.Fatal("expected non-empty path")
	}

	if !strings.Contains(path, ".github") {
		t.Errorf("path %q should contain .github subdirectory", path)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("file should exist")
	}
}
