// Package platform implements platform adapters for IDE integrations.
// Supports: claude-code, gemini-cli, vscode-copilot, cursor, opencode, codex.
//
// The MCP server layer is 100% portable and needs no adapter.
// Only the hook layer requires platform-specific adapters.
package platform

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Paradigm describes the hook I/O mechanism a platform uses.
type Paradigm string

const (
	ParadigmJSONStdio Paradigm = "json-stdio"
	ParadigmTSPlugin  Paradigm = "ts-plugin"
	ParadigmMCPOnly   Paradigm = "mcp-only"
)

// PlatformID identifies a supported platform.
type PlatformID string

const (
	PlatformClaudeCode    PlatformID = "claude-code"
	PlatformGeminiCLI     PlatformID = "gemini-cli"
	PlatformVSCodeCopilot PlatformID = "vscode-copilot"
	PlatformCursor        PlatformID = "cursor"
	PlatformOpenCode      PlatformID = "opencode"
	PlatformCodex         PlatformID = "codex"
	PlatformKiro          PlatformID = "kiro"
	PlatformOpenClaw      PlatformID = "openclaw"
	PlatformAntigravity   PlatformID = "antigravity"
	PlatformUnknown       PlatformID = "unknown"
)

// Capabilities describes what hooks a platform supports.
type Capabilities struct {
	PreToolUse              bool
	PostToolUse             bool
	PreCompact              bool
	SessionStart            bool
	CanModifyArgs           bool
	CanModifyOutput         bool
	CanInjectSessionContext bool
}

// NormalizedEvent is the platform-agnostic representation of a hook event.
type NormalizedEvent struct {
	ToolName   string          `json:"toolName,omitempty"`
	ToolInput  map[string]any  `json:"toolInput,omitempty"`
	ToolOutput string          `json:"toolOutput,omitempty"`
	IsError    bool            `json:"isError,omitempty"`
	SessionID  string          `json:"sessionId"`
	ProjectDir string          `json:"projectDir,omitempty"`
	Source     string          `json:"source,omitempty"` // startup, compact, resume, clear
	Raw        json.RawMessage `json:"raw,omitempty"`
}

// HookResponse is the platform-agnostic hook response.
type HookResponse struct {
	Decision          string         `json:"decision"` // allow, deny, modify, context
	Reason            string         `json:"reason,omitempty"`
	UpdatedInput      map[string]any `json:"updatedInput,omitempty"`
	AdditionalContext string         `json:"additionalContext,omitempty"`
	UpdatedOutput     string         `json:"updatedOutput,omitempty"`
	Context           string         `json:"context,omitempty"` // for PreCompact/SessionStart
}

// Adapter is the contract for platform-specific hook implementations.
type Adapter interface { //nolint:interfacebloat // platform adapters require all hook lifecycle methods
	// ID returns the platform identifier.
	ID() PlatformID

	// Name returns the human-readable platform name.
	Name() string

	// Paradigm returns the hook I/O paradigm.
	Paradigm() Paradigm

	// Capabilities returns what this platform supports.
	Capabilities() Capabilities

	// ValidateHooks checks whether the platform's settings file exists and
	// has the expected hook entries configured. Returns a slice of diagnostic
	// strings such as "settings file exists: OK" or "PreToolUse hook configured: MISSING".
	ValidateHooks() []string

	// ParsePreToolUse parses raw stdin JSON into a NormalizedEvent.
	ParsePreToolUse(raw json.RawMessage) NormalizedEvent

	// ParsePostToolUse parses raw stdin JSON into a NormalizedEvent.
	ParsePostToolUse(raw json.RawMessage) NormalizedEvent

	// ParsePreCompact parses raw stdin JSON into a NormalizedEvent.
	ParsePreCompact(raw json.RawMessage) NormalizedEvent

	// ParseSessionStart parses raw stdin JSON into a NormalizedEvent.
	ParseSessionStart(raw json.RawMessage) NormalizedEvent

	// FormatPreToolUse formats a HookResponse into platform-specific JSON output.
	FormatPreToolUse(resp HookResponse) json.RawMessage

	// FormatPostToolUse formats a HookResponse into platform-specific JSON output.
	FormatPostToolUse(resp HookResponse) json.RawMessage

	// FormatPreCompact formats a HookResponse into platform-specific output.
	FormatPreCompact(resp HookResponse) json.RawMessage

	// FormatSessionStart formats a HookResponse into platform-specific output.
	FormatSessionStart(resp HookResponse) json.RawMessage

	// SettingsPath returns the path to the platform's settings file.
	SettingsPath() string

	// SessionDir returns the directory where session data is stored.
	SessionDir() string
}

// validateHooksCommon provides shared hook validation logic for adapters that
// use JSON-based settings files. It checks that the settings file exists and
// that expected hook keys are present in the file content.
func validateHooksCommon(settingsPath string, caps Capabilities, hookKeys map[string]string) []string {
	var diags []string

	// Check settings file existence.
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		diags = append(diags, "settings file exists: MISSING")
		// If the file doesn't exist, all hooks are missing.
		for _, label := range sortedHookLabels(caps, hookKeys) {
			diags = append(diags, label+" hook configured: MISSING")
		}

		return diags
	}

	diags = append(diags, "settings file exists: OK")

	content := string(data)

	for _, label := range sortedHookLabels(caps, hookKeys) {
		key := hookKeys[label]
		if key != "" && contains(content, key) {
			diags = append(diags, label+" hook configured: OK")
		} else {
			diags = append(diags, label+" hook configured: MISSING")
		}
	}

	return diags
}

// sortedHookLabels returns hook labels in a deterministic order, filtered by capabilities.
func sortedHookLabels(caps Capabilities, hookKeys map[string]string) []string {
	order := []struct {
		label   string
		enabled bool
	}{
		{"PreToolUse", caps.PreToolUse},
		{"PostToolUse", caps.PostToolUse},
		{"PreCompact", caps.PreCompact},
		{"SessionStart", caps.SessionStart},
	}

	var labels []string

	for _, o := range order {
		if o.enabled {
			if _, ok := hookKeys[o.label]; ok {
				labels = append(labels, o.label)
			}
		}
	}

	return labels
}

// contains checks if s contains substr (simple string search).
func contains(s, substr string) bool {
	return len(substr) > 0 && len(s) >= len(substr) && stringContains(s, substr)
}

// stringContains is a simple substring search.
func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}

	return false
}

// Get returns the adapter for a given platform ID.
func Get(id PlatformID) (Adapter, error) {
	switch id {
	case PlatformClaudeCode:
		return &claudeCodeAdapter{}, nil
	case PlatformGeminiCLI:
		return &geminiCLIAdapter{}, nil
	case PlatformVSCodeCopilot:
		return &vscodeCopilotAdapter{}, nil
	case PlatformCursor:
		return &cursorAdapter{}, nil
	case PlatformOpenCode:
		return &openCodeAdapter{}, nil
	case PlatformCodex:
		return &codexAdapter{}, nil
	case PlatformKiro:
		return &kiroAdapter{}, nil
	case PlatformOpenClaw:
		return &openClawAdapter{}, nil
	case PlatformAntigravity:
		return &antigravityAdapter{}, nil
	case PlatformUnknown:
		return nil, fmt.Errorf("unknown platform: %s", id)
	default:
		return nil, fmt.Errorf("unknown platform: %s", id)
	}
}

// AllPlatformIDs returns all supported platform identifiers.
func AllPlatformIDs() []PlatformID {
	return []PlatformID{
		PlatformClaudeCode,
		PlatformGeminiCLI,
		PlatformVSCodeCopilot,
		PlatformCursor,
		PlatformOpenCode,
		PlatformCodex,
		PlatformKiro,
		PlatformOpenClaw,
		PlatformAntigravity,
	}
}

// homeDir returns the user's home directory.
func homeDir() string {
	h, _ := os.UserHomeDir()
	return h
}

// marshalJSON is a helper that marshals v and returns RawMessage.
func marshalJSON(v any) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		return []byte("{}")
	}

	return data
}

// emptyJSON returns an empty JSON object.
func emptyJSON() json.RawMessage {
	return json.RawMessage(`{}`)
}

// ensureDir creates a directory if it doesn't exist.
func ensureDir(dir string) string {
	_ = os.MkdirAll(dir, 0o755)
	return dir
}

// extractString safely extracts a string from a map.
func extractString(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}

	return ""
}

// extractMap safely extracts a map from a map.
func extractMap(m map[string]any, key string) map[string]any {
	if v, ok := m[key]; ok {
		if sub, ok := v.(map[string]any); ok {
			return sub
		}
	}

	return nil
}

// extractBool safely extracts a bool from a map.
func extractBool(m map[string]any, key string) bool {
	if v, ok := m[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}

	return false
}

// parseRawToMap parses raw JSON into a map.
func parseRawToMap(raw json.RawMessage) map[string]any {
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return make(map[string]any)
	}

	return m
}

// sessionDirFor computes a platform's session directory.
func sessionDirFor(parts ...string) string {
	return ensureDir(filepath.Join(append([]string{homeDir()}, parts...)...))
}
