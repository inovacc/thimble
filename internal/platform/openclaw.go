package platform

import (
	"encoding/json"
)

// openClawAdapter implements Adapter for OpenClaw.
//
// Hook paradigm: TS Plugin
// OpenClaw uses a TypeScript plugin paradigm similar to OpenCode.
// Tool name mapping: supports both camelCase and snake_case input fields.
// Routing file: AGENTS.md
type openClawAdapter struct{}

func (a *openClawAdapter) ID() PlatformID     { return PlatformOpenClaw }
func (a *openClawAdapter) Name() string       { return "OpenClaw" }
func (a *openClawAdapter) Paradigm() Paradigm { return ParadigmTSPlugin }

func (a *openClawAdapter) Capabilities() Capabilities {
	return Capabilities{
		PreToolUse:              true,
		PostToolUse:             true,
		PreCompact:              false,
		SessionStart:            false,
		CanModifyArgs:           true,
		CanModifyOutput:         true,
		CanInjectSessionContext: false,
	}
}

func (a *openClawAdapter) ValidateHooks() []string {
	return validateHooksCommon(a.SettingsPath(), a.Capabilities(), map[string]string{
		"PreToolUse":  "tool.execute.before",
		"PostToolUse": "tool.execute.after",
	})
}

// extractStringDualCase checks both camelCase and snake_case keys.
func extractStringDualCase(m map[string]any, camelKey, snakeKey string) string {
	if v := extractString(m, camelKey); v != "" {
		return v
	}

	return extractString(m, snakeKey)
}

// extractMapDualCase checks both camelCase and snake_case keys.
func extractMapDualCase(m map[string]any, camelKey, snakeKey string) map[string]any {
	if v := extractMap(m, camelKey); v != nil {
		return v
	}

	return extractMap(m, snakeKey)
}

// extractBoolDualCase checks both camelCase and snake_case keys.
func extractBoolDualCase(m map[string]any, camelKey, snakeKey string) bool {
	if extractBool(m, camelKey) {
		return true
	}

	return extractBool(m, snakeKey)
}

func (a *openClawAdapter) ParsePreToolUse(raw json.RawMessage) NormalizedEvent {
	m := parseRawToMap(raw)

	return NormalizedEvent{
		ToolName:   extractStringDualCase(m, "toolName", "tool_name"),
		ToolInput:  extractMapDualCase(m, "params", "tool_input"),
		SessionID:  extractStringDualCase(m, "sessionId", "sessionID"),
		ProjectDir: extractStringDualCase(m, "projectDir", "directory"),
		Raw:        raw,
	}
}

func (a *openClawAdapter) ParsePostToolUse(raw json.RawMessage) NormalizedEvent {
	m := parseRawToMap(raw)

	return NormalizedEvent{
		ToolName:   extractStringDualCase(m, "toolName", "tool_name"),
		ToolInput:  extractMapDualCase(m, "params", "tool_input"),
		ToolOutput: extractStringDualCase(m, "toolOutput", "tool_output"),
		IsError:    extractBoolDualCase(m, "isError", "is_error"),
		SessionID:  extractStringDualCase(m, "sessionId", "sessionID"),
		ProjectDir: extractStringDualCase(m, "projectDir", "directory"),
		Raw:        raw,
	}
}

func (a *openClawAdapter) ParsePreCompact(raw json.RawMessage) NormalizedEvent {
	return NormalizedEvent{Raw: raw}
}

func (a *openClawAdapter) ParseSessionStart(raw json.RawMessage) NormalizedEvent {
	return NormalizedEvent{Raw: raw}
}

func (a *openClawAdapter) FormatPreToolUse(resp HookResponse) json.RawMessage {
	// Convert "ask" to block (OpenClaw doesn't support ask, only allow/deny).
	if resp.Decision == "deny" || resp.Decision == "ask" {
		return marshalJSON(map[string]any{
			"block":       true,
			"blockReason": resp.Reason,
		})
	}

	if resp.Decision == "modify" {
		return marshalJSON(map[string]any{
			"args": resp.UpdatedInput,
		})
	}

	return emptyJSON()
}

func (a *openClawAdapter) FormatPostToolUse(resp HookResponse) json.RawMessage {
	if resp.AdditionalContext != "" {
		return marshalJSON(map[string]any{
			"additionalContext": resp.AdditionalContext,
		})
	}

	return emptyJSON()
}

func (a *openClawAdapter) FormatPreCompact(_ HookResponse) json.RawMessage   { return emptyJSON() }
func (a *openClawAdapter) FormatSessionStart(_ HookResponse) json.RawMessage { return emptyJSON() }

// WorkspaceKey extracts a workspace identifier from a hook payload.
// Uses workspace_id if present, falls back to projectDir/directory.
func (a *openClawAdapter) WorkspaceKey(raw json.RawMessage) string {
	m := parseRawToMap(raw)
	if key := extractStringDualCase(m, "workspaceId", "workspace_id"); key != "" {
		return key
	}

	return extractStringDualCase(m, "projectDir", "directory")
}

func (a *openClawAdapter) SettingsPath() string {
	return "openclaw.json"
}

func (a *openClawAdapter) SessionDir() string {
	return sessionDirFor(".config", "openclaw", "thimble", "sessions")
}
