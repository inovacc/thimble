package security

import (
	"regexp"
	"strings"
)

// NetworkCommandResult describes the outcome of network command detection.
type NetworkCommandResult struct {
	IsCurlWget   bool
	IsInlineHTTP bool
	Advisory     string
}

// stripQuotedStrings removes single-quoted, double-quoted, and backtick-quoted
// strings to prevent false positives on embedded text (e.g. gh issue edit --body "text with curl").
func stripQuotedStrings(cmd string) string {
	// Remove heredocs (<<EOF...EOF, <<'EOF'...EOF, <<"EOF"...EOF).
	cmd = stripHeredocs(cmd)
	// Remove double-quoted strings.
	cmd = regexp.MustCompile(`"[^"]*"`).ReplaceAllString(cmd, "")
	// Remove single-quoted strings.
	cmd = regexp.MustCompile(`'[^']*'`).ReplaceAllString(cmd, "")
	// Remove backtick-quoted strings.
	cmd = regexp.MustCompile("`[^`]*`").ReplaceAllString(cmd, "")

	return cmd
}

// heredocStartRe matches the start of a shell heredoc: <<EOF, <<'EOF', <<"EOF", <<-EOF
var heredocStartRe = regexp.MustCompile(`<<-?\s*['"]?(\w+)['"]?`)

// stripHeredocs removes shell heredoc bodies to prevent false positives.
func stripHeredocs(cmd string) string {
	for {
		loc := heredocStartRe.FindStringSubmatchIndex(cmd)
		if loc == nil {
			break
		}

		delimiter := cmd[loc[2]:loc[3]]
		// Find the newline after the heredoc marker.
		startAfterMarker := loc[1]

		nlIdx := strings.Index(cmd[startAfterMarker:], "\n")
		if nlIdx < 0 {
			break
		}

		bodyStart := startAfterMarker + nlIdx + 1
		// Find the closing delimiter on its own line.
		endDelimRe := regexp.MustCompile(`(?m)^\s*` + regexp.QuoteMeta(delimiter) + `\b`)

		endLoc := endDelimRe.FindStringIndex(cmd[bodyStart:])
		if endLoc == nil {
			break
		}
		// Remove from the << marker through the end of the closing delimiter line.
		endAbs := bodyStart + endLoc[1]
		cmd = cmd[:loc[0]] + cmd[endAbs:]
	}

	return cmd
}

var curlWgetRe = regexp.MustCompile(`\b(curl|wget)\b`)

var inlineHTTPPatterns = []*regexp.Regexp{
	regexp.MustCompile(`\bfetch\s*\(`),
	regexp.MustCompile(`\brequests\.get\s*\(`),
	regexp.MustCompile(`\bhttp\.get\s*\(`),
	regexp.MustCompile(`\burllib\.request\b`),
	regexp.MustCompile(`\brequests\.post\s*\(`),
	regexp.MustCompile(`\brequests\.put\s*\(`),
	regexp.MustCompile(`\bhttp\.request\s*\(`),
}

// DetectNetworkCommand checks whether a command string contains curl/wget calls
// or inline HTTP calls (fetch, requests.get, http.get).
func DetectNetworkCommand(cmd string) NetworkCommandResult {
	stripped := stripQuotedStrings(cmd)

	if curlWgetRe.MatchString(stripped) {
		return NetworkCommandResult{
			IsCurlWget: true,
			Advisory:   "Consider using ctx_fetch_and_index instead of curl/wget for better context management.",
		}
	}

	for _, pat := range inlineHTTPPatterns {
		if pat.MatchString(stripped) {
			return NetworkCommandResult{
				IsInlineHTTP: true,
				Advisory:     "Inline HTTP calls detected. Use ctx_fetch_and_index to keep fetched data in the knowledge base.",
			}
		}
	}

	return NetworkCommandResult{}
}

var buildToolRe = regexp.MustCompile(`\b(gradle|gradlew|mvn|mvnw)\b`)

// DetectBuildTool checks whether a command invokes a build tool (gradle, mvn)
// that should be routed through ctx_execute for output management.
func DetectBuildTool(cmd string) string {
	stripped := stripQuotedStrings(cmd)

	m := buildToolRe.FindString(stripped)
	if m != "" {
		return m
	}

	return ""
}

// IsWebFetchBlocked returns true if the tool name is WebFetch,
// which should be blocked in favor of ctx_fetch_and_index.
func IsWebFetchBlocked(toolName string) bool {
	return toolName == "WebFetch"
}

// IsAgentOrTask returns true if the tool name is one that should have
// routing instructions injected via AdditionalContext.
func IsAgentOrTask(toolName string) bool {
	switch toolName {
	case "Agent", "Task":
		return true
	}

	return false
}

// toolAliasMap maps platform-specific tool names (lowercase) to canonical names.
// Covers Gemini CLI, OpenCode, Codex, Cursor, VS Code Copilot, and Kiro.
var toolAliasMap = map[string]string{
	// Gemini CLI
	"run_shell_command":   "Bash",
	"read_file":           "Read",
	"read_many_files":     "Read",
	"grep_search":         "Grep",
	"search_file_content": "Grep",
	"web_fetch":           "WebFetch",

	// OpenCode
	"bash":  "Bash",
	"view":  "Read",
	"grep":  "Grep",
	"fetch": "WebFetch",
	"agent": "Agent",

	// Codex
	"shell":          "Bash",
	"shell_command":  "Bash",
	"exec_command":   "Bash",
	"container.exec": "Bash",
	"local_shell":    "Bash",
	"grep_files":     "Grep",

	// Cursor
	"mcp_web_fetch":  "WebFetch",
	"mcp_fetch_tool": "WebFetch",

	// VS Code Copilot
	"run_in_terminal": "Bash",

	// Kiro
	"fs_read":      "Read",
	"fs_write":     "Write",
	"execute_bash": "Bash",
}

// NormalizeToolName maps platform-specific tool names to canonical names.
// The lookup is case-insensitive. Unknown tool names pass through unchanged.
func NormalizeToolName(toolName string) string {
	toolName = strings.TrimSpace(toolName)
	if canonical, ok := toolAliasMap[strings.ToLower(toolName)]; ok {
		return canonical
	}

	return toolName
}
