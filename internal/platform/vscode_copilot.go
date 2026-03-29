package platform

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// vscodeCopilotAdapter implements Adapter for VS Code Copilot.
//
// Hook paradigm: JSON stdin/stdout
// Hook names: PreToolUse, PostToolUse, PreCompact, SessionStart (PascalCase)
// Blocking: permissionDecision: "deny" (same as Claude Code)
// Arg modification: hookSpecificOutput > updatedInput (NOT flat like Claude)
// Session ID: sessionId (camelCase)
// MCP tool naming: f1e_ prefix
type vscodeCopilotAdapter struct{}

func (a *vscodeCopilotAdapter) ID() PlatformID     { return PlatformVSCodeCopilot }
func (a *vscodeCopilotAdapter) Name() string       { return "VS Code Copilot" }
func (a *vscodeCopilotAdapter) Paradigm() Paradigm { return ParadigmJSONStdio }

func (a *vscodeCopilotAdapter) Capabilities() Capabilities {
	return Capabilities{
		PreToolUse:              true,
		PostToolUse:             true,
		PreCompact:              true,
		SessionStart:            true,
		CanModifyArgs:           false,
		CanModifyOutput:         false,
		CanInjectSessionContext: false,
	}
}

func (a *vscodeCopilotAdapter) ValidateHooks() []string {
	return validateHooksCommon(a.SettingsPath(), a.Capabilities(), map[string]string{
		"PreToolUse":   "PreToolUse",
		"PostToolUse":  "PostToolUse",
		"PreCompact":   "PreCompact",
		"SessionStart": "SessionStart",
	})
}

func (a *vscodeCopilotAdapter) ParsePreToolUse(raw json.RawMessage) NormalizedEvent {
	m := parseRawToMap(raw)

	return NormalizedEvent{
		ToolName:   extractString(m, "tool_name"),
		ToolInput:  extractMap(m, "tool_input"),
		SessionID:  extractString(m, "sessionId"),
		ProjectDir: a.projectDir(),
		Raw:        raw,
	}
}

func (a *vscodeCopilotAdapter) ParsePostToolUse(raw json.RawMessage) NormalizedEvent {
	m := parseRawToMap(raw)

	return NormalizedEvent{
		ToolName:   extractString(m, "tool_name"),
		ToolInput:  extractMap(m, "tool_input"),
		ToolOutput: extractString(m, "tool_output"),
		IsError:    extractBool(m, "is_error"),
		SessionID:  extractString(m, "sessionId"),
		ProjectDir: a.projectDir(),
		Raw:        raw,
	}
}

func (a *vscodeCopilotAdapter) ParsePreCompact(raw json.RawMessage) NormalizedEvent {
	m := parseRawToMap(raw)

	return NormalizedEvent{
		SessionID:  extractString(m, "sessionId"),
		ProjectDir: a.projectDir(),
		Raw:        raw,
	}
}

func (a *vscodeCopilotAdapter) ParseSessionStart(raw json.RawMessage) NormalizedEvent {
	m := parseRawToMap(raw)

	source := extractString(m, "source")
	if source == "" {
		source = "startup"
	}

	return NormalizedEvent{
		SessionID:  extractString(m, "sessionId"),
		Source:     source,
		ProjectDir: a.projectDir(),
		Raw:        raw,
	}
}

func (a *vscodeCopilotAdapter) FormatPreToolUse(resp HookResponse) json.RawMessage {
	switch resp.Decision {
	case "deny":
		return marshalJSON(map[string]any{
			"permissionDecision": "deny",
			"reason":             resp.Reason,
		})
	case "modify":
		return marshalJSON(map[string]any{
			"hookSpecificOutput": map[string]any{
				"hookEventName": "PreToolUse",
				"updatedInput":  resp.UpdatedInput,
			},
		})
	case "context":
		return marshalJSON(map[string]any{
			"hookSpecificOutput": map[string]any{
				"hookEventName":     "PreToolUse",
				"additionalContext": resp.AdditionalContext,
			},
		})
	default:
		return emptyJSON()
	}
}

func (a *vscodeCopilotAdapter) FormatPostToolUse(resp HookResponse) json.RawMessage {
	if resp.AdditionalContext != "" {
		return marshalJSON(map[string]any{
			"hookSpecificOutput": map[string]any{
				"hookEventName":     "PostToolUse",
				"additionalContext": resp.AdditionalContext,
			},
		})
	}

	return emptyJSON()
}

func (a *vscodeCopilotAdapter) FormatPreCompact(resp HookResponse) json.RawMessage {
	if resp.Context != "" {
		return marshalJSON(resp.Context)
	}

	return emptyJSON()
}

func (a *vscodeCopilotAdapter) FormatSessionStart(resp HookResponse) json.RawMessage {
	if resp.Context != "" {
		return marshalJSON(resp.Context)
	}

	return emptyJSON()
}

func (a *vscodeCopilotAdapter) SettingsPath() string {
	return filepath.Join(".github", "hooks")
}

func (a *vscodeCopilotAdapter) SessionDir() string {
	return sessionDirFor(".vscode", "thimble", "sessions")
}

func (a *vscodeCopilotAdapter) projectDir() string {
	// VS Code Copilot also uses CLAUDE_PROJECT_DIR.
	return os.Getenv("CLAUDE_PROJECT_DIR")
}
