package plugin

import (
	"fmt"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"
)

// SandboxConfig is the JSON-serializable sandbox configuration embedded in plugin definitions.
type SandboxConfig struct {
	AllowCommands []string `json:"allow_commands,omitempty"` // glob patterns like "git *", "echo *"
	DenyPaths     []string `json:"deny_paths,omitempty"`     // file path deny patterns
	MaxTimeoutSec int      `json:"max_timeout_sec,omitempty"`
}

// PluginSandbox is the runtime sandbox enforcement structure derived from SandboxConfig.
type PluginSandbox struct {
	AllowedCommands []string      // glob patterns for allowed commands
	DeniedPaths     []string      // file path deny patterns
	MaxTimeout      time.Duration // maximum execution timeout
}

// defaultAllowedCommands is the restrictive default command allowlist.
var defaultAllowedCommands = []string{
	"echo *",
	"cat *",
	"git *",
	"go *",
	"python *",
	"python3 *",
	"node *",
}

const defaultMaxTimeout = 30 * time.Second

// DefaultSandbox returns a restrictive default sandbox configuration.
func DefaultSandbox() *PluginSandbox {
	return &PluginSandbox{
		AllowedCommands: defaultAllowedCommands,
		DeniedPaths:     nil,
		MaxTimeout:      defaultMaxTimeout,
	}
}

// SandboxFromPlugin extracts a PluginSandbox from a PluginDef.
// If the plugin has no sandbox config, DefaultSandbox is returned.
func SandboxFromPlugin(def *PluginDef) *PluginSandbox {
	if def.Sandbox == nil {
		return DefaultSandbox()
	}

	sb := &PluginSandbox{
		AllowedCommands: def.Sandbox.AllowCommands,
		DeniedPaths:     def.Sandbox.DenyPaths,
		MaxTimeout:      defaultMaxTimeout,
	}

	// Fall back to default allowlist if none specified.
	if len(sb.AllowedCommands) == 0 {
		sb.AllowedCommands = defaultAllowedCommands
	}

	if def.Sandbox.MaxTimeoutSec > 0 {
		sb.MaxTimeout = time.Duration(def.Sandbox.MaxTimeoutSec) * time.Second
	}

	return sb
}

// ValidateCommand checks whether a command is allowed by the sandbox.
// It extracts the base command (first token) and matches it against
// the allowlist using glob patterns. Returns nil if allowed.
func ValidateCommand(sandbox *PluginSandbox, cmd string) error {
	if sandbox == nil {
		sandbox = DefaultSandbox()
	}

	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return fmt.Errorf("empty command")
	}

	// Strip inline env var assignments (e.g., "FOO=bar cmd args").
	effective := stripEnvPrefix(cmd)

	for _, pattern := range sandbox.AllowedCommands {
		if matchCommandGlob(pattern, effective) {
			return nil
		}
	}

	// Extract first token for the error message.
	base := strings.Fields(effective)[0]

	return fmt.Errorf("command %q not allowed by plugin sandbox (base: %s)", cmd, base)
}

// ValidatePath checks whether a file path is denied by the sandbox.
// Returns nil if the path is allowed.
func ValidatePath(sandbox *PluginSandbox, path string) error {
	if sandbox == nil || len(sandbox.DeniedPaths) == 0 {
		return nil
	}

	normalized := strings.ReplaceAll(path, "\\", "/")
	caseInsensitive := runtime.GOOS == "windows"

	for _, pattern := range sandbox.DeniedPaths {
		pat := pattern
		candidate := normalized

		if caseInsensitive {
			pat = strings.ToLower(pat)
			candidate = strings.ToLower(candidate)
		}

		if matched, _ := filepath.Match(pat, candidate); matched {
			return fmt.Errorf("path %q denied by sandbox pattern %q", path, pattern)
		}

		// Also check if the pattern matches as a suffix (for partial paths).
		if strings.Contains(candidate, pat) {
			return fmt.Errorf("path %q denied by sandbox pattern %q", path, pattern)
		}
	}

	return nil
}

// networkPatterns matches commands that perform network access.
var networkPatterns = []string{"curl", "wget", "fetch"}

// ValidateToolPermissions checks whether a command is allowed by the tool's
// per-tool permission scopes. If perms is nil, restrictive defaults apply
// (all false). Returns nil if the command passes all checks.
func ValidateToolPermissions(sandbox *PluginSandbox, perms *ToolPermissions, cmd string) error {
	if perms == nil {
		perms = &ToolPermissions{} // all-false defaults
	}

	trimmed := strings.TrimSpace(cmd)
	if trimmed == "" {
		return nil // empty commands are caught elsewhere
	}

	effective := stripEnvPrefix(trimmed)

	// Network check: deny curl/wget/fetch when AllowNetwork is false.
	if !perms.AllowNetwork {
		base := strings.Fields(effective)[0]
		if slices.Contains(networkPatterns, base) {
			return fmt.Errorf("tool permission denied: network command %q requires allow_network", base)
		}
	}

	// File-write check: deny output redirection (>, >>) when AllowFileWrite is false.
	if !perms.AllowFileWrite {
		if strings.Contains(effective, ">>") || redirectPattern(effective) {
			return fmt.Errorf("tool permission denied: output redirection requires allow_file_write")
		}
	}

	// Subprocess check: deny pipes (|) and command substitution ($()) when AllowSubprocess is false.
	if !perms.AllowSubprocess {
		if strings.Contains(effective, "|") {
			return fmt.Errorf("tool permission denied: pipe (|) requires allow_subprocess")
		}

		if strings.Contains(effective, "$(") {
			return fmt.Errorf("tool permission denied: command substitution $() requires allow_subprocess")
		}

		if strings.Contains(effective, "`") {
			return fmt.Errorf("tool permission denied: backtick substitution requires allow_subprocess")
		}
	}

	return nil
}

// EffectiveTimeout returns the per-tool timeout if set, otherwise the sandbox max.
func EffectiveTimeout(sandbox *PluginSandbox, perms *ToolPermissions) time.Duration {
	if perms != nil && perms.MaxTimeout > 0 {
		return time.Duration(perms.MaxTimeout) * time.Second
	}

	if sandbox != nil {
		return sandbox.MaxTimeout
	}

	return defaultMaxTimeout
}

// redirectPattern detects single > redirection that is not inside >> (already caught).
// It looks for ">" preceded by a space or start-of-token, not part of ">>".
func redirectPattern(cmd string) bool {
	for i, c := range cmd {
		if c == '>' {
			// Skip >> (handled separately).
			if i+1 < len(cmd) && cmd[i+1] == '>' {
				return true // >> also counts
			}

			return true
		}
	}

	return false
}

// stripEnvPrefix removes leading VAR=VALUE tokens from a command string.
func stripEnvPrefix(cmd string) string {
	parts := strings.Fields(cmd)

	i := 0
	for i < len(parts) && strings.Contains(parts[i], "=") && !strings.HasPrefix(parts[i], "-") {
		// Ensure it looks like KEY=VALUE (key must be alphanumeric/underscore).
		eqIdx := strings.Index(parts[i], "=")
		if eqIdx < 0 {
			break
		}

		key := parts[i][:eqIdx]

		if isEnvVarName(key) {
			i++
		} else {
			break
		}
	}

	if i >= len(parts) {
		return cmd // all tokens were env vars — return original
	}

	return strings.Join(parts[i:], " ")
}

// isEnvVarName checks if s looks like a valid environment variable name.
func isEnvVarName(s string) bool {
	if s == "" {
		return false
	}

	for j, c := range s {
		if c == '_' || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') {
			continue
		}

		if j > 0 && c >= '0' && c <= '9' {
			continue
		}

		return false
	}

	return true
}

// matchCommandGlob matches a command string against a glob pattern.
// The pattern format is "base *" where base is the command name and * matches anything.
func matchCommandGlob(pattern, cmd string) bool {
	caseInsensitive := runtime.GOOS == "windows"

	p := pattern
	c := cmd

	if caseInsensitive {
		p = strings.ToLower(p)
		c = strings.ToLower(c)
	}

	// Simple glob: "cmd *" matches "cmd" alone and "cmd anything..."
	if base, ok := strings.CutSuffix(p, " *"); ok {
		cmdBase := strings.Fields(c)[0]

		return cmdBase == base
	}

	// Exact match.
	if p == c {
		return true
	}

	// filepath.Match for more complex patterns.
	matched, _ := filepath.Match(p, c)

	return matched
}
