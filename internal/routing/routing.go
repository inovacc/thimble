// Package routing generates platform-specific routing instruction files.
// These files (CLAUDE.md, GEMINI.md, AGENTS.md, copilot-instructions.md)
// teach the LLM to use thimble MCP tools instead of raw shell commands.
package routing

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/inovacc/thimble/internal/platform"
)

// InstructionsConfig describes a routing instructions file for a platform.
type InstructionsConfig struct {
	FileName            string
	GlobalPath          string
	ProjectRelativePath string
}

// GetConfig returns the routing instructions config for a platform.
func GetConfig(id platform.PlatformID) InstructionsConfig {
	home, _ := os.UserHomeDir()

	switch id {
	case platform.PlatformClaudeCode:
		return InstructionsConfig{
			FileName:            "CLAUDE.md",
			GlobalPath:          filepath.Join(home, ".claude", "CLAUDE.md"),
			ProjectRelativePath: "CLAUDE.md",
		}
	case platform.PlatformGeminiCLI:
		return InstructionsConfig{
			FileName:            "GEMINI.md",
			GlobalPath:          filepath.Join(home, ".gemini", "GEMINI.md"),
			ProjectRelativePath: "GEMINI.md",
		}
	case platform.PlatformVSCodeCopilot:
		return InstructionsConfig{
			FileName:            "copilot-instructions.md",
			GlobalPath:          filepath.Join(home, ".github", "copilot-instructions.md"),
			ProjectRelativePath: filepath.Join(".github", "copilot-instructions.md"),
		}
	case platform.PlatformCursor:
		return InstructionsConfig{
			FileName:            ".cursorrules",
			GlobalPath:          filepath.Join(home, ".cursor", "rules"),
			ProjectRelativePath: ".cursorrules",
		}
	case platform.PlatformOpenCode:
		return InstructionsConfig{
			FileName:            "AGENTS.md",
			GlobalPath:          "",
			ProjectRelativePath: "AGENTS.md",
		}
	case platform.PlatformKiro:
		return InstructionsConfig{
			FileName:            "KIRO.md",
			GlobalPath:          filepath.Join(home, ".kiro", "KIRO.md"),
			ProjectRelativePath: "KIRO.md",
		}
	case platform.PlatformOpenClaw:
		return InstructionsConfig{
			FileName:            "AGENTS.md",
			GlobalPath:          "",
			ProjectRelativePath: "AGENTS.md",
		}
	case platform.PlatformAntigravity:
		return InstructionsConfig{
			FileName:            "GEMINI.md",
			GlobalPath:          filepath.Join(home, ".antigravity", "GEMINI.md"),
			ProjectRelativePath: "GEMINI.md",
		}
	case platform.PlatformCodex, platform.PlatformUnknown:
		return InstructionsConfig{
			FileName:            "CLAUDE.md",
			GlobalPath:          filepath.Join(home, ".claude", "CLAUDE.md"),
			ProjectRelativePath: "CLAUDE.md",
		}
	default:
		return InstructionsConfig{
			FileName:            "CLAUDE.md",
			GlobalPath:          filepath.Join(home, ".claude", "CLAUDE.md"),
			ProjectRelativePath: "CLAUDE.md",
		}
	}
}

// toolPrefix returns the fully-qualified MCP tool name prefix for a platform.
func toolPrefix(id platform.PlatformID) string {
	switch id { //nolint:exhaustive // default covers all other platforms
	case platform.PlatformClaudeCode, platform.PlatformCodex:
		return "mcp__thimble__"
	case platform.PlatformCursor:
		return "MCP:"
	default:
		return ""
	}
}

// GenerateInstructions returns the routing instructions content for a platform.
func GenerateInstructions(id platform.PlatformID) string {
	var sb strings.Builder

	prefix := toolPrefix(id)

	sb.WriteString("<context_window_protection>\n")
	sb.WriteString("  <priority_instructions>\n")
	sb.WriteString("    Raw tool output floods your context window. You MUST use thimble MCP tools to keep raw data in the sandbox.\n")
	sb.WriteString("  </priority_instructions>\n\n")

	sb.WriteString("  <tool_selection_hierarchy>\n")
	_, _ = fmt.Fprintf(&sb, "    1. GATHER: %sctx_batch_execute(commands, queries)\n", prefix)
	sb.WriteString("       - Primary tool for research. Runs all commands, auto-indexes, and searches.\n")
	sb.WriteString("       - ONE call replaces many individual steps.\n")
	_, _ = fmt.Fprintf(&sb, "    2. FOLLOW-UP: %sctx_search(queries: [\"q1\", \"q2\", ...])\n", prefix)
	sb.WriteString("       - Use for all follow-up questions. ONE call, many queries.\n")
	_, _ = fmt.Fprintf(&sb, "    3. PROCESSING: %sctx_execute(language, code) | %sctx_execute_file(path, language, code)\n", prefix, prefix)
	sb.WriteString("       - Use for API calls, log analysis, and data processing.\n")
	sb.WriteString("  </tool_selection_hierarchy>\n\n")

	sb.WriteString("  <forbidden_actions>\n")
	sb.WriteString("    - DO NOT use Bash for commands producing >20 lines of output.\n")
	sb.WriteString("    - DO NOT use Read for analysis (use execute_file). Read IS correct for files you intend to Edit.\n")
	_, _ = fmt.Fprintf(&sb, "    - DO NOT use WebFetch (use %sctx_fetch_and_index instead).\n", prefix)
	sb.WriteString("    - Bash is ONLY for git/mkdir/rm/mv/navigation.\n")
	sb.WriteString("  </forbidden_actions>\n\n")

	sb.WriteString("  <output_constraints>\n")
	sb.WriteString("    <word_limit>Keep your final response under 500 words.</word_limit>\n")
	sb.WriteString("    <artifact_policy>\n")
	sb.WriteString("      Write artifacts (code, configs, PRDs) to FILES. NEVER return them as inline text.\n")
	sb.WriteString("      Return only: file path + 1-line description.\n")
	sb.WriteString("    </artifact_policy>\n")
	sb.WriteString("  </output_constraints>\n\n")

	sb.WriteString("  <ctx_commands>\n")

	switch id { //nolint:exhaustive // default covers all other platforms
	case platform.PlatformClaudeCode:
		sb.WriteString("    When the user says \"ctx stats\" → Call the stats MCP tool and display output.\n")
		sb.WriteString("    When the user says \"ctx doctor\" → Call the doctor MCP tool.\n")
		sb.WriteString("    When the user says \"ctx upgrade\" → Call the upgrade MCP tool.\n")
	case platform.PlatformGeminiCLI:
		sb.WriteString("    When the user says \"ctx stats\" → Call the stats MCP tool.\n")
		sb.WriteString("    When the user says \"ctx doctor\" → Call the doctor MCP tool.\n")
	default:
		sb.WriteString("    Use ctx_stats, ctx_doctor tools for diagnostics.\n")
	}

	sb.WriteString("  </ctx_commands>\n")

	sb.WriteString("</context_window_protection>\n")

	return sb.String()
}

// WriteInstructions writes routing instructions to a project directory.
// Returns the path written, or empty string if the file already contains context-mode instructions.
func WriteInstructions(projectDir string, id platform.PlatformID) (string, error) {
	config := GetConfig(id)
	targetPath := filepath.Join(projectDir, config.ProjectRelativePath)

	content := GenerateInstructions(id)

	// Check if file already has thimble routing instructions.
	existing, err := os.ReadFile(targetPath)
	if err == nil {
		if strings.Contains(string(existing), "context_window_protection") {
			return "", nil // Already present.
		}
		// Append to existing file.
		content = string(existing) + "\n\n" + content
	}

	// Ensure directory exists.
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return "", fmt.Errorf("create dir: %w", err)
	}

	if err := os.WriteFile(targetPath, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("write file: %w", err)
	}

	return targetPath, nil
}
