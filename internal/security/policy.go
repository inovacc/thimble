// Package security implements permission enforcement — parsing policies,
// evaluating commands, splitting chained commands, and glob matching.
package security

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

// PermissionDecision is the result of evaluating a command or path.
type PermissionDecision string

const (
	Allow PermissionDecision = "allow"
	Deny  PermissionDecision = "deny"
	Ask   PermissionDecision = "ask"
)

// SecurityPolicy contains allow/deny/ask pattern lists from a settings file.
type SecurityPolicy struct {
	Allow []string `json:"allow"`
	Deny  []string `json:"deny"`
	Ask   []string `json:"ask"`

	// DangerousGitOverrides, when non-empty, replaces the default DangerousGitSubcommands list.
	DangerousGitOverrides []string `json:"dangerous_git_overrides,omitempty"`

	// DangerousGhOverrides, when non-empty, replaces the default DangerousGhSubcommands list.
	DangerousGhOverrides []string `json:"dangerous_gh_overrides,omitempty"`
}

// CommandDecision is the result of evaluating a command.
type CommandDecision struct {
	Decision       PermissionDecision
	MatchedPattern string
}

// FileDecision is the result of evaluating a file path.
type FileDecision struct {
	Denied         bool
	MatchedPattern string
}

// ── Pattern Parsing ──

var bashPatternRe = regexp.MustCompile(`^Bash\((.+)\)$`)
var toolPatternRe = regexp.MustCompile(`^(\w+)\((.+)\)$`)

// ParseBashPattern extracts the glob from "Bash(glob)". Returns "" if not a Bash pattern.
func ParseBashPattern(pattern string) string {
	m := bashPatternRe.FindStringSubmatch(pattern)
	if m == nil {
		return ""
	}

	return m[1]
}

// ToolPattern is a parsed "ToolName(glob)" pattern.
type ToolPattern struct {
	Tool string
	Glob string
}

// ParseToolPattern parses "ToolName(glob)" patterns. Returns nil if invalid.
func ParseToolPattern(pattern string) *ToolPattern {
	m := toolPatternRe.FindStringSubmatch(pattern)
	if m == nil {
		return nil
	}

	return &ToolPattern{Tool: m[1], Glob: m[2]}
}

// ── Glob-to-Regex Conversion ──

// escapeRegex escapes all regex special characters.
func escapeRegex(s string) string {
	special := regexp.MustCompile(`[.*+?^${}()|[\]\\/\-]`)

	return special.ReplaceAllStringFunc(s, func(ch string) string {
		return `\` + ch
	})
}

// convertGlobPart escapes regex specials except *, then converts * to .*
func convertGlobPart(glob string) string {
	special := regexp.MustCompile(`[.+?^${}()|[\]\\/\-]`)
	escaped := special.ReplaceAllStringFunc(glob, func(ch string) string {
		return `\` + ch
	})

	return strings.ReplaceAll(escaped, "*", ".*")
}

// GlobToRegex converts a Bash permission glob to a regex.
// Colon format: "tree:*" → ^tree(\s.*)?$
// Space format: "sudo *" → ^sudo .*$
func GlobToRegex(glob string, caseInsensitive bool) *regexp.Regexp {
	var regexStr string

	before, after, ok := strings.Cut(glob, ":")
	if ok {
		command := before
		argsGlob := after
		regexStr = `^` + escapeRegex(command) + `(\s` + convertGlobPart(argsGlob) + `)?$`
	} else {
		regexStr = `^` + convertGlobPart(glob) + `$`
	}

	if caseInsensitive {
		regexStr = "(?i)" + regexStr
	}

	re, err := regexp.Compile(regexStr)
	if err != nil {
		return regexp.MustCompile(`^$`) // fallback: matches nothing useful
	}

	return re
}

// FileGlobToRegex converts a file path glob to a regex.
// ** matches any path segments, * matches anything except /, ? matches single non-/.
func FileGlobToRegex(glob string, caseInsensitive bool) *regexp.Regexp {
	var b strings.Builder

	i := 0
	for i < len(glob) {
		if i+1 < len(glob) && glob[i] == '*' && glob[i+1] == '*' { //nolint:gocritic // if-else chain is clearer for glob parsing
			if i+2 < len(glob) && glob[i+2] == '/' {
				b.WriteString("(.*/)?")

				i += 3
			} else {
				b.WriteString(".*")

				i += 2
			}
		} else if glob[i] == '*' {
			b.WriteString("[^/]*")

			i++
		} else if glob[i] == '?' {
			b.WriteString("[^/]")

			i++
		} else {
			ch := string(glob[i])
			special := regexp.MustCompile(`[.+^${}()|[\]\\/\-]`)
			b.WriteString(special.ReplaceAllStringFunc(ch, func(c string) string {
				return `\` + c
			}))

			i++
		}
	}

	regexStr := `^` + b.String() + `$`
	if caseInsensitive {
		regexStr = "(?i)" + regexStr
	}

	re, err := regexp.Compile(regexStr)
	if err != nil {
		return regexp.MustCompile(`^$`)
	}

	return re
}

// MatchesAnyPattern checks if a command matches any Bash pattern in the list.
func MatchesAnyPattern(command string, patterns []string, caseInsensitive bool) string {
	for _, pattern := range patterns {
		glob := ParseBashPattern(pattern)
		if glob == "" {
			continue
		}

		if GlobToRegex(glob, caseInsensitive).MatchString(command) {
			return pattern
		}
	}

	return ""
}

// ── Chained Command Splitting ──

// SplitChainedCommands splits a shell command on chain operators (&&, ||, ;, |)
// while respecting quotes and backticks.
func SplitChainedCommands(command string) []string {
	var (
		parts   []string
		current strings.Builder
	)

	inSingle, inDouble, inBacktick := false, false, false

	for i := 0; i < len(command); i++ {
		ch := command[i]

		prev := byte(0)
		if i > 0 {
			prev = command[i-1]
		}

		switch {
		case ch == '\'' && !inDouble && !inBacktick && prev != '\\':
			inSingle = !inSingle

			current.WriteByte(ch)
		case ch == '"' && !inSingle && !inBacktick && prev != '\\':
			inDouble = !inDouble

			current.WriteByte(ch)
		case ch == '`' && !inSingle && !inDouble && prev != '\\':
			inBacktick = !inBacktick

			current.WriteByte(ch)
		case !inSingle && !inDouble && !inBacktick:
			if ch == ';' { //nolint:gocritic // if-else chain is clearer for shell parsing
				if s := strings.TrimSpace(current.String()); s != "" {
					parts = append(parts, s)
				}

				current.Reset()
			} else if ch == '|' && i+1 < len(command) && command[i+1] == '|' {
				if s := strings.TrimSpace(current.String()); s != "" {
					parts = append(parts, s)
				}

				current.Reset()

				i++ // skip second |
			} else if ch == '&' && i+1 < len(command) && command[i+1] == '&' {
				if s := strings.TrimSpace(current.String()); s != "" {
					parts = append(parts, s)
				}

				current.Reset()

				i++ // skip second &
			} else if ch == '|' {
				if s := strings.TrimSpace(current.String()); s != "" {
					parts = append(parts, s)
				}

				current.Reset()
			} else {
				current.WriteByte(ch)
			}
		default:
			current.WriteByte(ch)
		}
	}

	if s := strings.TrimSpace(current.String()); s != "" {
		parts = append(parts, s)
	}

	return parts
}

// ── Settings Reader ──

type settingsFile struct {
	Permissions struct {
		Allow []string `json:"allow"`
		Deny  []string `json:"deny"`
		Ask   []string `json:"ask"`
	} `json:"permissions"`
	DangerousGitOverrides []string `json:"dangerous_git_overrides,omitempty"`
	DangerousGhOverrides  []string `json:"dangerous_gh_overrides,omitempty"`
}

func readSingleSettings(path string) *SecurityPolicy {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var s settingsFile
	if err := json.Unmarshal(data, &s); err != nil {
		return nil
	}

	filterBash := func(arr []string) []string {
		var result []string

		for _, p := range arr {
			if ParseBashPattern(p) != "" {
				result = append(result, p)
			}
		}

		return result
	}

	return &SecurityPolicy{
		Allow:                 filterBash(s.Permissions.Allow),
		Deny:                  filterBash(s.Permissions.Deny),
		Ask:                   filterBash(s.Permissions.Ask),
		DangerousGitOverrides: s.DangerousGitOverrides,
		DangerousGhOverrides:  s.DangerousGhOverrides,
	}
}

// ReadBashPolicies reads Bash permission policies from up to 3 settings files.
// Returns policies in precedence order (most local first):
// 1. .claude/settings.local.json (project-local)
// 2. .claude/settings.json (project-shared)
// 3. ~/.claude/settings.json (global)
func ReadBashPolicies(projectDir, globalSettingsPath string) []SecurityPolicy {
	var policies []SecurityPolicy

	if projectDir != "" {
		if p := readSingleSettings(filepath.Join(projectDir, ".claude", "settings.local.json")); p != nil {
			policies = append(policies, *p)
		}

		if p := readSingleSettings(filepath.Join(projectDir, ".claude", "settings.json")); p != nil {
			policies = append(policies, *p)
		}
	}

	if globalSettingsPath == "" {
		home, _ := os.UserHomeDir()
		globalSettingsPath = filepath.Join(home, ".claude", "settings.json")
	}

	if p := readSingleSettings(globalSettingsPath); p != nil {
		policies = append(policies, *p)
	}

	return policies
}

// ReadToolDenyPatterns reads deny globs for a specific tool from settings files.
func ReadToolDenyPatterns(toolName, projectDir, globalSettingsPath string) [][]string {
	var result [][]string

	extractGlobs := func(path string) []string {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		var s settingsFile
		if err := json.Unmarshal(data, &s); err != nil {
			return nil
		}

		var globs []string

		for _, entry := range s.Permissions.Deny {
			if tp := ParseToolPattern(entry); tp != nil && tp.Tool == toolName {
				globs = append(globs, tp.Glob)
			}
		}

		return globs
	}

	if projectDir != "" {
		if g := extractGlobs(filepath.Join(projectDir, ".claude", "settings.local.json")); g != nil {
			result = append(result, g)
		}

		if g := extractGlobs(filepath.Join(projectDir, ".claude", "settings.json")); g != nil {
			result = append(result, g)
		}
	}

	if globalSettingsPath == "" {
		home, _ := os.UserHomeDir()
		globalSettingsPath = filepath.Join(home, ".claude", "settings.json")
	}

	if g := extractGlobs(globalSettingsPath); g != nil {
		result = append(result, g)
	}

	return result
}

// ── Decision Engine ──

// EvaluateCommand evaluates a command against policies in precedence order.
// Splits chained commands and checks each segment against deny patterns.
// Within each policy: deny > ask > allow. Default: "ask".
func EvaluateCommand(command string, policies []SecurityPolicy) CommandDecision {
	caseInsensitive := runtime.GOOS == "windows"

	segments := SplitChainedCommands(command)
	for _, segment := range segments {
		for _, policy := range policies {
			if match := MatchesAnyPattern(segment, policy.Deny, caseInsensitive); match != "" {
				return CommandDecision{Decision: Deny, MatchedPattern: match}
			}
		}
	}

	for _, policy := range policies {
		if match := MatchesAnyPattern(command, policy.Ask, caseInsensitive); match != "" {
			return CommandDecision{Decision: Ask, MatchedPattern: match}
		}

		if match := MatchesAnyPattern(command, policy.Allow, caseInsensitive); match != "" {
			return CommandDecision{Decision: Allow, MatchedPattern: match}
		}
	}

	return CommandDecision{Decision: Ask}
}

// EvaluateCommandDenyOnly is the server-side variant: only enforce deny patterns.
// Returns Deny if any deny pattern matches, otherwise Allow.
func EvaluateCommandDenyOnly(command string, policies []SecurityPolicy) CommandDecision {
	caseInsensitive := runtime.GOOS == "windows"

	segments := SplitChainedCommands(command)
	for _, segment := range segments {
		for _, policy := range policies {
			if match := MatchesAnyPattern(segment, policy.Deny, caseInsensitive); match != "" {
				return CommandDecision{Decision: Deny, MatchedPattern: match}
			}
		}
	}

	return CommandDecision{Decision: Allow}
}

// EvaluateFilePath checks if a file path should be denied based on deny globs.
func EvaluateFilePath(filePath string, denyGlobs [][]string) FileDecision {
	caseInsensitive := runtime.GOOS == "windows"
	normalized := strings.ReplaceAll(filePath, "\\", "/")

	for _, globs := range denyGlobs {
		for _, glob := range globs {
			if FileGlobToRegex(glob, caseInsensitive).MatchString(normalized) {
				return FileDecision{Denied: true, MatchedPattern: glob}
			}
		}
	}

	return FileDecision{Denied: false}
}
