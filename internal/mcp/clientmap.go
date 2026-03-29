package mcp

import (
	"strings"

	"github.com/inovacc/thimble/internal/platform"
)

// clientInfoMap maps MCP clientInfo.name strings to platform IDs.
var clientInfoMap = map[string]platform.PlatformID{
	"claude-code":           platform.PlatformClaudeCode,
	"gemini-cli-mcp-client": platform.PlatformGeminiCLI,
	"antigravity-client":    platform.PlatformAntigravity,
	"cursor-vscode":         platform.PlatformCursor,
	"visual-studio-code":    platform.PlatformVSCodeCopilot,
	"codex":                 platform.PlatformCodex,
	"codex-mcp-client":      platform.PlatformCodex,
	"kiro cli":              platform.PlatformKiro,
	"kiro":                  platform.PlatformKiro,
	"opencode":              platform.PlatformOpenCode,
	"openclaw":              platform.PlatformOpenClaw,
}

// DetectPlatformFromClientInfo maps an MCP clientInfo.name to a PlatformID.
// The lookup is case-insensitive. Falls back to PlatformClaudeCode if unknown.
func DetectPlatformFromClientInfo(name string) platform.PlatformID {
	lower := strings.ToLower(strings.TrimSpace(name))
	if id, ok := clientInfoMap[lower]; ok {
		return id
	}

	// Substring matching for partial matches.
	for key, id := range clientInfoMap {
		if strings.Contains(lower, key) {
			return id
		}
	}

	return platform.PlatformClaudeCode
}
