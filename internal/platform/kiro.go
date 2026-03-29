package platform

import (
	"encoding/json"
	"path/filepath"
)

// kiroAdapter implements Adapter for Kiro IDE.
//
// Hook paradigm: JSON-Stdio
// Kiro uses json-stdio hooks with tool_name, cwd, and tool_input fields.
// Detection: KIRO_PROJECT_DIR env var or MCP clientInfo.name containing "kiro".
// Routing file: KIRO.md
type kiroAdapter struct{}

func (a *kiroAdapter) ID() PlatformID     { return PlatformKiro }
func (a *kiroAdapter) Name() string       { return "Kiro" }
func (a *kiroAdapter) Paradigm() Paradigm { return ParadigmJSONStdio }

func (a *kiroAdapter) Capabilities() Capabilities {
	return Capabilities{
		PreToolUse:              true,
		PostToolUse:             true,
		PreCompact:              false,
		SessionStart:            false,
		CanModifyArgs:           false,
		CanModifyOutput:         false,
		CanInjectSessionContext: false,
	}
}

func (a *kiroAdapter) ValidateHooks() []string {
	return validateHooksCommon(a.SettingsPath(), a.Capabilities(), map[string]string{
		"PreToolUse":  "hook_event_name",
		"PostToolUse": "hook_event_name",
	})
}

func (a *kiroAdapter) ParsePreToolUse(raw json.RawMessage) NormalizedEvent {
	m := parseRawToMap(raw)

	return NormalizedEvent{
		ToolName:   extractString(m, "tool_name"),
		ToolInput:  extractMap(m, "tool_input"),
		SessionID:  extractString(m, "session_id"),
		ProjectDir: extractString(m, "cwd"),
		Raw:        raw,
	}
}

func (a *kiroAdapter) ParsePostToolUse(raw json.RawMessage) NormalizedEvent {
	m := parseRawToMap(raw)

	return NormalizedEvent{
		ToolName:   extractString(m, "tool_name"),
		ToolInput:  extractMap(m, "tool_input"),
		ToolOutput: extractString(m, "tool_output"),
		IsError:    extractBool(m, "is_error"),
		SessionID:  extractString(m, "session_id"),
		ProjectDir: extractString(m, "cwd"),
		Raw:        raw,
	}
}

func (a *kiroAdapter) ParsePreCompact(raw json.RawMessage) NormalizedEvent {
	return NormalizedEvent{Raw: raw}
}

func (a *kiroAdapter) ParseSessionStart(raw json.RawMessage) NormalizedEvent {
	return NormalizedEvent{Raw: raw}
}

func (a *kiroAdapter) FormatPreToolUse(resp HookResponse) json.RawMessage {
	if resp.Decision == "deny" {
		return marshalJSON(map[string]any{
			"exitCode": 2,
			"stderr":   resp.Reason,
		})
	}

	if resp.AdditionalContext != "" {
		return marshalJSON(map[string]any{
			"exitCode": 0,
			"stdout":   resp.AdditionalContext,
		})
	}

	return emptyJSON()
}

func (a *kiroAdapter) FormatPostToolUse(resp HookResponse) json.RawMessage {
	if resp.AdditionalContext != "" {
		return marshalJSON(map[string]any{
			"exitCode": 0,
			"stdout":   resp.AdditionalContext,
		})
	}

	return marshalJSON(map[string]any{
		"exitCode": 0,
		"stdout":   "",
	})
}

func (a *kiroAdapter) FormatPreCompact(_ HookResponse) json.RawMessage   { return emptyJSON() }
func (a *kiroAdapter) FormatSessionStart(_ HookResponse) json.RawMessage { return emptyJSON() }

func (a *kiroAdapter) SettingsPath() string {
	return filepath.Join(homeDir(), ".kiro", "settings.json")
}

func (a *kiroAdapter) SessionDir() string {
	return sessionDirFor(".kiro", "thimble", "sessions")
}
