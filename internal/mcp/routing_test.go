package mcp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/inovacc/thimble/internal/platform"
)

func TestWriteRoutingIfNeededMCPOnly(t *testing.T) {
	dir := t.TempDir()
	b := &Bridge{
		projectDir:       dir,
		detectedPlatform: platform.PlatformKiro, // MCP-only, no SessionStart
	}

	b.writeRoutingIfNeeded()

	// Kiro routing file should be written.
	routingFile := filepath.Join(dir, "KIRO.md")
	if _, err := os.Stat(routingFile); os.IsNotExist(err) {
		t.Error("expected routing file to be written for MCP-only platform")
	}
}

func TestWriteRoutingIfNeededHookedPlatform(t *testing.T) {
	dir := t.TempDir()
	b := &Bridge{
		projectDir:       dir,
		detectedPlatform: platform.PlatformClaudeCode, // Has SessionStart hooks
	}

	b.writeRoutingIfNeeded()

	// Claude Code has SessionStart hooks, so routing should NOT be written.
	routingFile := filepath.Join(dir, "CLAUDE.md")
	if _, err := os.Stat(routingFile); err == nil {
		t.Error("routing file should NOT be written for hooked platform")
	}
}
