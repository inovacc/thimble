package platform

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// cursorAdapter implements Adapter for Cursor.
//
// Hook paradigm: JSON stdin/stdout
// Hook names: preToolUse, postToolUse (lowerCamel), sessionStart (buggy)
// Blocking: { "permission": "deny", "user_message": "..." }
// Arg modification: not natively supported in v1
// Output modification: not supported in v1
// Session context: { "additional_context": "..." }
// Session ID: conversation_id > CURSOR_TRACE_ID > ppid
// MCP tool naming: MCP:<tool> in hook payloads
type cursorAdapter struct{}

func (a *cursorAdapter) ID() PlatformID     { return PlatformCursor }
func (a *cursorAdapter) Name() string       { return "Cursor" }
func (a *cursorAdapter) Paradigm() Paradigm { return ParadigmJSONStdio }

func (a *cursorAdapter) Capabilities() Capabilities {
	return Capabilities{
		PreToolUse:              true,
		PostToolUse:             true,
		PreCompact:              false, // not shipped in v1
		SessionStart:            false, // documented but buggy, treat as unsupported
		CanModifyArgs:           false,
		CanModifyOutput:         false,
		CanInjectSessionContext: false,
	}
}

func (a *cursorAdapter) ValidateHooks() []string {
	return validateHooksCommon(a.SettingsPath(), a.Capabilities(), map[string]string{
		"PreToolUse":  "preToolUse",
		"PostToolUse": "postToolUse",
	})
}

func (a *cursorAdapter) ParsePreToolUse(raw json.RawMessage) NormalizedEvent {
	m := parseRawToMap(raw)

	return NormalizedEvent{
		ToolName:   extractString(m, "tool_name"),
		ToolInput:  extractMap(m, "tool_input"),
		SessionID:  a.extractSessionID(m),
		ProjectDir: a.projectDir(m),
		Raw:        raw,
	}
}

func (a *cursorAdapter) ParsePostToolUse(raw json.RawMessage) NormalizedEvent {
	m := parseRawToMap(raw)

	return NormalizedEvent{
		ToolName:   extractString(m, "tool_name"),
		ToolInput:  extractMap(m, "tool_input"),
		ToolOutput: extractString(m, "tool_output"),
		IsError:    extractBool(m, "is_error"),
		SessionID:  a.extractSessionID(m),
		ProjectDir: a.projectDir(m),
		Raw:        raw,
	}
}

func (a *cursorAdapter) ParsePreCompact(raw json.RawMessage) NormalizedEvent {
	// Cursor does not support PreCompact.
	m := parseRawToMap(raw)

	return NormalizedEvent{
		SessionID:  a.extractSessionID(m),
		ProjectDir: a.projectDir(m),
		Raw:        raw,
	}
}

func (a *cursorAdapter) ParseSessionStart(raw json.RawMessage) NormalizedEvent {
	m := parseRawToMap(raw)

	return NormalizedEvent{
		SessionID:  a.extractSessionID(m),
		Source:     "startup",
		ProjectDir: a.projectDir(m),
		Raw:        raw,
	}
}

func (a *cursorAdapter) FormatPreToolUse(resp HookResponse) json.RawMessage {
	switch resp.Decision {
	case "deny":
		return marshalJSON(map[string]any{
			"permission":   "deny",
			"user_message": resp.Reason,
		})
	case "context":
		return marshalJSON(map[string]any{
			"additional_context": resp.AdditionalContext,
		})
	default:
		return emptyJSON()
	}
}

func (a *cursorAdapter) FormatPostToolUse(resp HookResponse) json.RawMessage {
	if resp.AdditionalContext != "" {
		return marshalJSON(map[string]any{
			"additional_context": resp.AdditionalContext,
		})
	}

	return emptyJSON()
}

func (a *cursorAdapter) FormatPreCompact(resp HookResponse) json.RawMessage {
	return emptyJSON()
}

func (a *cursorAdapter) FormatSessionStart(resp HookResponse) json.RawMessage {
	if resp.Context != "" {
		return marshalJSON(map[string]any{
			"additional_context": resp.Context,
		})
	}

	return emptyJSON()
}

func (a *cursorAdapter) SettingsPath() string {
	return filepath.Join(homeDir(), ".cursor", "hooks.json")
}

func (a *cursorAdapter) SessionDir() string {
	return sessionDirFor(".cursor", "thimble", "sessions")
}

// extractSessionID: conversation_id > CURSOR_TRACE_ID > ppid.
func (a *cursorAdapter) extractSessionID(m map[string]any) string {
	if cid := extractString(m, "conversation_id"); cid != "" {
		return cid
	}

	if env := os.Getenv("CURSOR_TRACE_ID"); env != "" {
		return env
	}

	return fmt.Sprintf("pid-%d", os.Getppid())
}

// projectDir extracts from workspace_roots in stdin or env.
func (a *cursorAdapter) projectDir(m map[string]any) string {
	// Cursor provides workspace_roots in stdin payload.
	if roots, ok := m["workspace_roots"]; ok {
		if arr, ok := roots.([]any); ok && len(arr) > 0 {
			if s, ok := arr[0].(string); ok {
				return s
			}
		}
	}

	return ""
}
