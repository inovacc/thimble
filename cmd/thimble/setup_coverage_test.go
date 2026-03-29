package main

import (
	"testing"

	"github.com/inovacc/thimble/internal/platform"
)

func TestBuildHookConfig_AllPlatforms(t *testing.T) {
	platforms := []struct {
		id        platform.PlatformID
		wantHooks bool
	}{
		{platform.PlatformClaudeCode, true},
		{platform.PlatformGeminiCLI, true},
		{platform.PlatformVSCodeCopilot, true},
		{platform.PlatformCursor, true},
		{platform.PlatformOpenCode, false},
		{platform.PlatformCodex, false},
		{platform.PlatformUnknown, false},
	}

	for _, tt := range platforms {
		t.Run(string(tt.id), func(t *testing.T) {
			config := buildHookConfig(tt.id, "/usr/local/bin/thimble")

			_, hasHooks := config["hooks"]
			if hasHooks != tt.wantHooks {
				t.Errorf("platform %s: hasHooks = %v, want %v", tt.id, hasHooks, tt.wantHooks)
			}
		})
	}
}

func TestResolveClient_Aliases(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		wantID platform.PlatformID
		wantOK bool
	}{
		{"claude alias", "claude", platform.PlatformClaudeCode, true},
		{"gemini alias", "gemini", platform.PlatformGeminiCLI, true},
		{"vscode alias", "vscode", platform.PlatformVSCodeCopilot, true},
		{"copilot alias", "copilot", platform.PlatformVSCodeCopilot, true},
		{"cursor alias", "cursor", platform.PlatformCursor, true},
		{"opencode alias", "opencode", platform.PlatformOpenCode, true},
		{"codex alias", "codex", platform.PlatformCodex, true},
		{"full name", "claude-code", platform.PlatformClaudeCode, true},
		{"unknown", "vim", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := resolveClient(tt.input)
			if tt.wantOK {
				if err != nil {
					t.Fatalf("resolveClient(%q) error: %v", tt.input, err)
				}

				if id != tt.wantID {
					t.Errorf("resolveClient(%q) = %q, want %q", tt.input, id, tt.wantID)
				}
			} else if err == nil {
				t.Errorf("resolveClient(%q) should fail", tt.input)
			}
		})
	}
}

func TestGenerateHookConfig_DryRun(t *testing.T) {
	setupDryRun = true

	defer func() { setupDryRun = false }()

	adapter, err := platform.Get(platform.PlatformClaudeCode)
	if err != nil {
		t.Fatalf("platform.Get: %v", err)
	}

	err = generateHookConfig(platform.PlatformClaudeCode, adapter)
	if err != nil {
		t.Fatalf("generateHookConfig: %v", err)
	}
}

func TestGenerateMCPConfig_DryRun(t *testing.T) {
	setupDryRun = true

	defer func() { setupDryRun = false }()

	adapter, err := platform.Get(platform.PlatformOpenCode)
	if err != nil {
		t.Fatalf("platform.Get: %v", err)
	}

	err = generateMCPConfig(platform.PlatformOpenCode, adapter)
	if err != nil {
		t.Fatalf("generateMCPConfig: %v", err)
	}
}

func TestGenerateRoutingInstructions_DryRun(t *testing.T) {
	setupDryRun = true

	defer func() { setupDryRun = false }()

	err := generateRoutingInstructions(platform.PlatformClaudeCode)
	if err != nil {
		t.Fatalf("generateRoutingInstructions: %v", err)
	}
}

func TestDefaultPluginDir_Platforms(t *testing.T) {
	tests := []struct {
		id       platform.PlatformID
		contains string
	}{
		{platform.PlatformClaudeCode, ".claude"},
		{platform.PlatformGeminiCLI, ".gemini"},
		{platform.PlatformVSCodeCopilot, ".github"},
		{platform.PlatformCursor, ".cursor"},
		{platform.PlatformOpenCode, ".thimble"},
	}

	for _, tt := range tests {
		t.Run(string(tt.id), func(t *testing.T) {
			dir := defaultPluginDir(tt.id)
			if !containsStr(dir, tt.contains) {
				t.Errorf("defaultPluginDir(%s) = %q, want to contain %q", tt.id, dir, tt.contains)
			}
		})
	}
}
