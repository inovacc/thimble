package platform

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

// claudeCodeAdapter implements Adapter for Claude Code.
//
// Hook paradigm: JSON stdin/stdout
// Session ID: transcript_path UUID > session_id > CLAUDE_SESSION_ID > ppid
// Blocking: permissionDecision: "deny"
// Arg modification: updatedInput at top level
// Output modification: updatedMCPToolOutput / additionalContext
type claudeCodeAdapter struct{}

var uuidFromTranscriptRe = regexp.MustCompile(`([a-f0-9-]{36})\.jsonl$`)

func (a *claudeCodeAdapter) ID() PlatformID     { return PlatformClaudeCode }
func (a *claudeCodeAdapter) Name() string       { return "Claude Code" }
func (a *claudeCodeAdapter) Paradigm() Paradigm { return ParadigmJSONStdio }

func (a *claudeCodeAdapter) Capabilities() Capabilities {
	return Capabilities{
		PreToolUse:              true,
		PostToolUse:             true,
		PreCompact:              true,
		SessionStart:            true,
		CanModifyArgs:           false, // Claude Code doesn't let hooks modify tool inputs
		CanModifyOutput:         true,
		CanInjectSessionContext: true,
	}
}

func (a *claudeCodeAdapter) ValidateHooks() []string {
	return validateHooksCommon(a.SettingsPath(), a.Capabilities(), map[string]string{
		"PreToolUse":   "PreToolUse",
		"PostToolUse":  "PostToolUse",
		"PreCompact":   "PreCompact",
		"SessionStart": "SessionStart",
	})
}

func (a *claudeCodeAdapter) ParsePreToolUse(raw json.RawMessage) NormalizedEvent {
	m := parseRawToMap(raw)

	return NormalizedEvent{
		ToolName:   extractString(m, "tool_name"),
		ToolInput:  extractMap(m, "tool_input"),
		SessionID:  a.extractSessionID(m),
		ProjectDir: a.projectDir(),
		Raw:        raw,
	}
}

func (a *claudeCodeAdapter) ParsePostToolUse(raw json.RawMessage) NormalizedEvent {
	m := parseRawToMap(raw)

	return NormalizedEvent{
		ToolName:   extractString(m, "tool_name"),
		ToolInput:  extractMap(m, "tool_input"),
		ToolOutput: extractString(m, "tool_output"),
		IsError:    extractBool(m, "is_error"),
		SessionID:  a.extractSessionID(m),
		ProjectDir: a.projectDir(),
		Raw:        raw,
	}
}

func (a *claudeCodeAdapter) ParsePreCompact(raw json.RawMessage) NormalizedEvent {
	m := parseRawToMap(raw)

	return NormalizedEvent{
		SessionID:  a.extractSessionID(m),
		ProjectDir: a.projectDir(),
		Raw:        raw,
	}
}

func (a *claudeCodeAdapter) ParseSessionStart(raw json.RawMessage) NormalizedEvent {
	m := parseRawToMap(raw)

	source := extractString(m, "source")
	if source == "" {
		source = "startup"
	}

	return NormalizedEvent{
		SessionID:  a.extractSessionID(m),
		Source:     source,
		ProjectDir: a.projectDir(),
		Raw:        raw,
	}
}

func (a *claudeCodeAdapter) FormatPreToolUse(resp HookResponse) json.RawMessage {
	var inner map[string]any

	switch resp.Decision {
	case "deny":
		inner = map[string]any{
			"permissionDecision": "deny",
			"reason":             resp.Reason,
		}
	case "modify":
		inner = map[string]any{
			"updatedInput": resp.UpdatedInput,
		}
	case "context":
		inner = map[string]any{
			"additionalContext": resp.AdditionalContext,
		}
	case "ask":
		inner = map[string]any{
			"permissionDecision": "ask",
		}
	default:
		return nil // passthrough — no stdout output
	}

	inner["hookEventName"] = "PreToolUse"

	return marshalJSON(map[string]any{"hookSpecificOutput": inner})
}

func (a *claudeCodeAdapter) FormatPostToolUse(resp HookResponse) json.RawMessage {
	inner := make(map[string]any)
	if resp.AdditionalContext != "" {
		inner["additionalContext"] = resp.AdditionalContext
	}

	if resp.UpdatedOutput != "" {
		inner["updatedMCPToolOutput"] = resp.UpdatedOutput
	}

	if len(inner) == 0 {
		return nil // passthrough — no stdout output
	}

	inner["hookEventName"] = "PostToolUse"

	return marshalJSON(map[string]any{"hookSpecificOutput": inner})
}

func (a *claudeCodeAdapter) FormatPreCompact(resp HookResponse) json.RawMessage {
	if resp.Context != "" {
		return marshalJSON(map[string]any{
			"hookSpecificOutput": map[string]any{
				"hookEventName":     "PreCompact",
				"additionalContext": resp.Context,
			},
		})
	}

	return nil // passthrough — no stdout output
}

func (a *claudeCodeAdapter) FormatSessionStart(resp HookResponse) json.RawMessage {
	if resp.Context != "" {
		return marshalJSON(map[string]any{
			"hookSpecificOutput": map[string]any{
				"hookEventName":     "SessionStart",
				"additionalContext": resp.Context,
			},
		})
	}

	return nil // passthrough — no stdout output
}

func (a *claudeCodeAdapter) SettingsPath() string {
	return filepath.Join(homeDir(), ".claude", "settings.json")
}

func (a *claudeCodeAdapter) SessionDir() string {
	return sessionDirFor(".claude", "thimble", "sessions")
}

// extractSessionID extracts session ID using Claude Code's priority chain:
// transcript_path UUID > session_id > CLAUDE_SESSION_ID env > ppid.
func (a *claudeCodeAdapter) extractSessionID(m map[string]any) string {
	if tp := extractString(m, "transcript_path"); tp != "" {
		if matches := uuidFromTranscriptRe.FindStringSubmatch(tp); len(matches) > 1 {
			return matches[1]
		}
	}

	if sid := extractString(m, "session_id"); sid != "" {
		return sid
	}

	if env := os.Getenv("CLAUDE_SESSION_ID"); env != "" {
		return env
	}

	return fmt.Sprintf("pid-%d", os.Getppid())
}

func (a *claudeCodeAdapter) projectDir() string {
	return os.Getenv("CLAUDE_PROJECT_DIR")
}
