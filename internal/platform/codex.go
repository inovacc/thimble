package platform

import (
	"encoding/json"
	"path/filepath"
)

// codexAdapter implements Adapter for Codex CLI.
//
// Hook paradigm: MCP-only (no hooks)
// Codex CLI does not support hooks. The only integration path is via MCP servers.
// All parse/format methods return empty/minimal results.
type codexAdapter struct{}

func (a *codexAdapter) ID() PlatformID        { return PlatformCodex }
func (a *codexAdapter) Name() string           { return "Codex CLI" }
func (a *codexAdapter) Paradigm() Paradigm     { return ParadigmMCPOnly }

func (a *codexAdapter) Capabilities() Capabilities {
	return Capabilities{
		PreToolUse:             false,
		PostToolUse:            false,
		PreCompact:             false,
		SessionStart:           false,
		CanModifyArgs:          false,
		CanModifyOutput:        false,
		CanInjectSessionContext: false,
	}
}

func (a *codexAdapter) ValidateHooks() []string {
	return []string{"settings file exists: N/A (MCP-only, no hooks)"}
}

func (a *codexAdapter) ParsePreToolUse(raw json.RawMessage) NormalizedEvent  { return NormalizedEvent{Raw: raw} }
func (a *codexAdapter) ParsePostToolUse(raw json.RawMessage) NormalizedEvent { return NormalizedEvent{Raw: raw} }
func (a *codexAdapter) ParsePreCompact(raw json.RawMessage) NormalizedEvent  { return NormalizedEvent{Raw: raw} }
func (a *codexAdapter) ParseSessionStart(raw json.RawMessage) NormalizedEvent { return NormalizedEvent{Raw: raw} }

func (a *codexAdapter) FormatPreToolUse(_ HookResponse) json.RawMessage    { return emptyJSON() }
func (a *codexAdapter) FormatPostToolUse(_ HookResponse) json.RawMessage   { return emptyJSON() }
func (a *codexAdapter) FormatPreCompact(_ HookResponse) json.RawMessage    { return emptyJSON() }
func (a *codexAdapter) FormatSessionStart(_ HookResponse) json.RawMessage  { return emptyJSON() }

func (a *codexAdapter) SettingsPath() string {
	return filepath.Join(homeDir(), ".codex", "config.toml")
}

func (a *codexAdapter) SessionDir() string {
	return sessionDirFor(".codex", "thimble", "sessions")
}
