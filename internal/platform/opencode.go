package platform

import (
	"encoding/json"
)

// openCodeAdapter implements Adapter for OpenCode.
//
// Hook paradigm: TS Plugin (not JSON stdin/stdout)
// Hook names: tool.execute.before, tool.execute.after, experimental.session.compacting
// Blocking: throw Error in plugin handler
// Session ID: input.sessionID (camelCase, uppercase ID)
// Note: OpenCode uses a TypeScript plugin paradigm. The Go binary cannot
// directly serve as a TS plugin, so this adapter provides parse/format
// compatibility for any bridge layer.
type openCodeAdapter struct{}

func (a *openCodeAdapter) ID() PlatformID     { return PlatformOpenCode }
func (a *openCodeAdapter) Name() string       { return "OpenCode" }
func (a *openCodeAdapter) Paradigm() Paradigm { return ParadigmTSPlugin }

func (a *openCodeAdapter) Capabilities() Capabilities {
	return Capabilities{
		PreToolUse:              true,
		PostToolUse:             true,
		PreCompact:              true,  // experimental
		SessionStart:            false, // broken (issue #14808)
		CanModifyArgs:           true,
		CanModifyOutput:         true, // caveat: TUI bug for bash
		CanInjectSessionContext: false,
	}
}

func (a *openCodeAdapter) ValidateHooks() []string {
	return validateHooksCommon(a.SettingsPath(), a.Capabilities(), map[string]string{
		"PreToolUse":  "tool.execute.before",
		"PostToolUse": "tool.execute.after",
		"PreCompact":  "experimental.session.compacting",
	})
}

func (a *openCodeAdapter) ParsePreToolUse(raw json.RawMessage) NormalizedEvent {
	m := parseRawToMap(raw)

	return NormalizedEvent{
		ToolName:   extractString(m, "tool_name"),
		ToolInput:  extractMap(m, "tool_input"),
		SessionID:  extractString(m, "sessionID"),
		ProjectDir: extractString(m, "directory"),
		Raw:        raw,
	}
}

func (a *openCodeAdapter) ParsePostToolUse(raw json.RawMessage) NormalizedEvent {
	m := parseRawToMap(raw)

	return NormalizedEvent{
		ToolName:   extractString(m, "tool_name"),
		ToolInput:  extractMap(m, "tool_input"),
		ToolOutput: extractString(m, "tool_output"),
		IsError:    extractBool(m, "is_error"),
		SessionID:  extractString(m, "sessionID"),
		ProjectDir: extractString(m, "directory"),
		Raw:        raw,
	}
}

func (a *openCodeAdapter) ParsePreCompact(raw json.RawMessage) NormalizedEvent {
	m := parseRawToMap(raw)

	return NormalizedEvent{
		SessionID:  extractString(m, "sessionID"),
		ProjectDir: extractString(m, "directory"),
		Raw:        raw,
	}
}

func (a *openCodeAdapter) ParseSessionStart(raw json.RawMessage) NormalizedEvent {
	// OpenCode SessionStart is broken — return minimal event.
	m := parseRawToMap(raw)

	return NormalizedEvent{
		SessionID:  extractString(m, "sessionID"),
		Source:     "startup",
		ProjectDir: extractString(m, "directory"),
		Raw:        raw,
	}
}

func (a *openCodeAdapter) FormatPreToolUse(resp HookResponse) json.RawMessage {
	if resp.Decision == "deny" {
		// OpenCode blocks via thrown errors in TS plugin.
		// For JSON bridge: return error indicator.
		return marshalJSON(map[string]any{
			"error":  true,
			"reason": resp.Reason,
		})
	}

	if resp.Decision == "modify" {
		return marshalJSON(map[string]any{
			"args": resp.UpdatedInput,
		})
	}

	return emptyJSON()
}

func (a *openCodeAdapter) FormatPostToolUse(resp HookResponse) json.RawMessage {
	if resp.AdditionalContext != "" {
		return marshalJSON(map[string]any{
			"additionalContext": resp.AdditionalContext,
		})
	}

	return emptyJSON()
}

func (a *openCodeAdapter) FormatPreCompact(resp HookResponse) json.RawMessage {
	if resp.Context != "" {
		return marshalJSON(resp.Context)
	}

	return emptyJSON()
}

func (a *openCodeAdapter) FormatSessionStart(resp HookResponse) json.RawMessage {
	return emptyJSON()
}

func (a *openCodeAdapter) SettingsPath() string {
	return "opencode.json"
}

func (a *openCodeAdapter) SessionDir() string {
	return sessionDirFor(".config", "opencode", "thimble", "sessions")
}
