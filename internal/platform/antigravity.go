package platform

import (
	"encoding/json"
	"path/filepath"
)

// antigravityAdapter implements Adapter for Antigravity.
//
// Hook paradigm: MCP-only (no hooks)
// Detection: MCP clientInfo.name containing "antigravity".
// Routing file: GEMINI.md (shares Gemini's routing file format)
type antigravityAdapter struct{}

func (a *antigravityAdapter) ID() PlatformID    { return PlatformAntigravity }
func (a *antigravityAdapter) Name() string      { return "Antigravity" }
func (a *antigravityAdapter) Paradigm() Paradigm { return ParadigmMCPOnly }

func (a *antigravityAdapter) Capabilities() Capabilities {
	return Capabilities{
		PreToolUse:              false,
		PostToolUse:             false,
		PreCompact:              false,
		SessionStart:            false,
		CanModifyArgs:           false,
		CanModifyOutput:         false,
		CanInjectSessionContext: false,
	}
}

func (a *antigravityAdapter) ValidateHooks() []string {
	return []string{"settings file exists: N/A (MCP-only, no hooks)"}
}

func (a *antigravityAdapter) ParsePreToolUse(raw json.RawMessage) NormalizedEvent  { return NormalizedEvent{Raw: raw} }
func (a *antigravityAdapter) ParsePostToolUse(raw json.RawMessage) NormalizedEvent { return NormalizedEvent{Raw: raw} }
func (a *antigravityAdapter) ParsePreCompact(raw json.RawMessage) NormalizedEvent  { return NormalizedEvent{Raw: raw} }
func (a *antigravityAdapter) ParseSessionStart(raw json.RawMessage) NormalizedEvent { return NormalizedEvent{Raw: raw} }

func (a *antigravityAdapter) FormatPreToolUse(_ HookResponse) json.RawMessage    { return emptyJSON() }
func (a *antigravityAdapter) FormatPostToolUse(_ HookResponse) json.RawMessage   { return emptyJSON() }
func (a *antigravityAdapter) FormatPreCompact(_ HookResponse) json.RawMessage    { return emptyJSON() }
func (a *antigravityAdapter) FormatSessionStart(_ HookResponse) json.RawMessage  { return emptyJSON() }

func (a *antigravityAdapter) SettingsPath() string {
	return filepath.Join(homeDir(), ".antigravity", "settings.json")
}

func (a *antigravityAdapter) SessionDir() string {
	return sessionDirFor(".antigravity", "thimble", "sessions")
}
