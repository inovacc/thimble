package platform

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// geminiCLIAdapter implements Adapter for Gemini CLI.
//
// Hook paradigm: JSON stdin/stdout
// Hook names: BeforeTool, AfterTool, PreCompress, SessionStart
// Blocking: decision: "deny" (NOT permissionDecision)
// Arg modification: hookSpecificOutput.tool_input (merged)
// Session ID: session_id field
type geminiCLIAdapter struct{}

func (a *geminiCLIAdapter) ID() PlatformID     { return PlatformGeminiCLI }
func (a *geminiCLIAdapter) Name() string       { return "Gemini CLI" }
func (a *geminiCLIAdapter) Paradigm() Paradigm { return ParadigmJSONStdio }

func (a *geminiCLIAdapter) Capabilities() Capabilities {
	return Capabilities{
		PreToolUse:              true,
		PostToolUse:             true,
		PreCompact:              true, // advisory only, async
		SessionStart:            true,
		CanModifyArgs:           false,
		CanModifyOutput:         false,
		CanInjectSessionContext: true,
	}
}

func (a *geminiCLIAdapter) ValidateHooks() []string {
	return validateHooksCommon(a.SettingsPath(), a.Capabilities(), map[string]string{
		"PreToolUse":   "BeforeTool",
		"PostToolUse":  "AfterTool",
		"PreCompact":   "PreCompress",
		"SessionStart": "SessionStart",
	})
}

func (a *geminiCLIAdapter) ParsePreToolUse(raw json.RawMessage) NormalizedEvent {
	m := parseRawToMap(raw)

	return NormalizedEvent{
		ToolName:   extractString(m, "tool_name"),
		ToolInput:  extractMap(m, "tool_input"),
		SessionID:  extractString(m, "session_id"),
		ProjectDir: a.projectDir(),
		Raw:        raw,
	}
}

func (a *geminiCLIAdapter) ParsePostToolUse(raw json.RawMessage) NormalizedEvent {
	m := parseRawToMap(raw)

	return NormalizedEvent{
		ToolName:   extractString(m, "tool_name"),
		ToolInput:  extractMap(m, "tool_input"),
		ToolOutput: extractString(m, "tool_output"),
		IsError:    extractBool(m, "is_error"),
		SessionID:  extractString(m, "session_id"),
		ProjectDir: a.projectDir(),
		Raw:        raw,
	}
}

func (a *geminiCLIAdapter) ParsePreCompact(raw json.RawMessage) NormalizedEvent {
	m := parseRawToMap(raw)

	return NormalizedEvent{
		SessionID:  extractString(m, "session_id"),
		ProjectDir: a.projectDir(),
		Raw:        raw,
	}
}

func (a *geminiCLIAdapter) ParseSessionStart(raw json.RawMessage) NormalizedEvent {
	m := parseRawToMap(raw)

	source := extractString(m, "source")
	if source == "" {
		source = "startup"
	}

	return NormalizedEvent{
		SessionID:  extractString(m, "session_id"),
		Source:     source,
		ProjectDir: a.projectDir(),
		Raw:        raw,
	}
}

func (a *geminiCLIAdapter) FormatPreToolUse(resp HookResponse) json.RawMessage {
	switch resp.Decision {
	case "deny":
		return marshalJSON(map[string]any{
			"decision": "deny",
			"reason":   resp.Reason,
		})
	case "modify":
		return marshalJSON(map[string]any{
			"hookSpecificOutput": map[string]any{
				"tool_input": resp.UpdatedInput,
			},
		})
	case "context":
		return marshalJSON(map[string]any{
			"hookSpecificOutput": map[string]any{
				"additionalContext": resp.AdditionalContext,
			},
		})
	default:
		return emptyJSON()
	}
}

func (a *geminiCLIAdapter) FormatPostToolUse(resp HookResponse) json.RawMessage {
	if resp.AdditionalContext != "" {
		return marshalJSON(map[string]any{
			"hookSpecificOutput": map[string]any{
				"additionalContext": resp.AdditionalContext,
			},
		})
	}

	return emptyJSON()
}

func (a *geminiCLIAdapter) FormatPreCompact(resp HookResponse) json.RawMessage {
	if resp.Context != "" {
		return marshalJSON(resp.Context)
	}

	return emptyJSON()
}

func (a *geminiCLIAdapter) FormatSessionStart(resp HookResponse) json.RawMessage {
	if resp.Context != "" {
		return marshalJSON(resp.Context)
	}

	return emptyJSON()
}

func (a *geminiCLIAdapter) SettingsPath() string {
	return filepath.Join(homeDir(), ".gemini", "settings.json")
}

func (a *geminiCLIAdapter) SessionDir() string {
	return sessionDirFor(".gemini", "thimble", "sessions")
}

func (a *geminiCLIAdapter) projectDir() string {
	if d := os.Getenv("GEMINI_PROJECT_DIR"); d != "" {
		return d
	}
	// Gemini CLI also sets CLAUDE_PROJECT_DIR as alias.
	return os.Getenv("CLAUDE_PROJECT_DIR")
}
