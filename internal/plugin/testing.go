package plugin

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// TestResult holds the outcome of a single plugin test check.
type TestResult struct {
	ToolName string `json:"tool_name,omitempty"`
	Check    string `json:"check"`
	Status   string `json:"status"` // "pass" or "fail"
	Error    string `json:"error,omitempty"`
}

// defaultDryRunTimeout is the timeout for dry-run command execution.
const defaultDryRunTimeout = 5 * time.Second

// lookPathFunc is replaceable for testing (avoids requiring real binaries on PATH).
var lookPathFunc = exec.LookPath

// TestPlugin validates and tests a plugin definition, returning a result per check.
// Checks performed:
//   - Schema validation (name, version, tools non-empty)
//   - Tool name prefix (ctx_ required)
//   - Command existence (first word of each tool command must be on PATH)
//   - Dry-run each tool command with a timeout
//   - Dependency check (all declared dependencies exist in the given registry)
func TestPlugin(def *PluginDef, registry []RegistryEntry) []TestResult {
	results := make([]TestResult, 0, 5) //nolint:mnd // 5 check categories below

	results = append(results, checkSchema(def)...)
	results = append(results, checkToolPrefixes(def)...)
	results = append(results, checkCommandExistence(def)...)
	results = append(results, checkDryRun(def)...)
	results = append(results, checkDependencies(def, registry)...)

	return results
}

// checkSchema validates required fields: name, version, and at least one tool.
func checkSchema(def *PluginDef) []TestResult {
	var results []TestResult

	if def.Name == "" {
		results = append(results, TestResult{
			Check:  "schema:name",
			Status: "fail",
			Error:  "plugin name is required",
		})
	} else {
		results = append(results, TestResult{
			Check:  "schema:name",
			Status: "pass",
		})
	}

	if def.Version == "" {
		results = append(results, TestResult{
			Check:  "schema:version",
			Status: "fail",
			Error:  "plugin version is required",
		})
	} else {
		results = append(results, TestResult{
			Check:  "schema:version",
			Status: "pass",
		})
	}

	if len(def.Tools) == 0 {
		results = append(results, TestResult{
			Check:  "schema:tools",
			Status: "fail",
			Error:  "plugin must have at least one tool",
		})
	} else {
		results = append(results, TestResult{
			Check:  "schema:tools",
			Status: "pass",
		})
	}

	return results
}

// checkToolPrefixes verifies each tool name starts with "ctx_".
func checkToolPrefixes(def *PluginDef) []TestResult {
	var results []TestResult

	for _, t := range def.Tools {
		if strings.HasPrefix(t.Name, "ctx_") {
			results = append(results, TestResult{
				ToolName: t.Name,
				Check:    "prefix:ctx_",
				Status:   "pass",
			})
		} else {
			results = append(results, TestResult{
				ToolName: t.Name,
				Check:    "prefix:ctx_",
				Status:   "fail",
				Error:    fmt.Sprintf("tool %q must have ctx_ prefix", t.Name),
			})
		}
	}

	return results
}

// checkCommandExistence verifies the first word of each tool command exists on PATH.
func checkCommandExistence(def *PluginDef) []TestResult {
	var results []TestResult

	for _, t := range def.Tools {
		if t.Command == "" {
			results = append(results, TestResult{
				ToolName: t.Name,
				Check:    "command:exists",
				Status:   "fail",
				Error:    "tool has no command",
			})

			continue
		}

		// Extract the first word (the binary name).
		bin := extractBinary(t.Command)

		_, err := lookPathFunc(bin)
		if err != nil {
			results = append(results, TestResult{
				ToolName: t.Name,
				Check:    "command:exists",
				Status:   "fail",
				Error:    fmt.Sprintf("command %q not found on PATH", bin),
			})
		} else {
			results = append(results, TestResult{
				ToolName: t.Name,
				Check:    "command:exists",
				Status:   "pass",
			})
		}
	}

	return results
}

// extractBinary returns the first whitespace-delimited token from a command string.
// It handles common shell patterns like env vars at the start.
func extractBinary(command string) string {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return ""
	}

	// Skip leading env var assignments (e.g., "FOO=bar cmd").
	for _, p := range parts {
		if !strings.Contains(p, "=") || strings.HasPrefix(p, "-") {
			return p
		}
	}

	return parts[0]
}

// checkDryRun executes each tool command with a 5-second timeout and checks exit code.
func checkDryRun(def *PluginDef) []TestResult {
	var results []TestResult

	for _, t := range def.Tools {
		if t.Command == "" {
			continue
		}

		results = append(results, dryRunTool(t))
	}

	return results
}

// dryRunTool runs a single tool command with a timeout and returns the result.
func dryRunTool(t ToolDef) TestResult {
	ctx, cancel := context.WithTimeout(context.Background(), defaultDryRunTimeout)
	defer cancel()

	// Use shell to handle pipes, templates, etc. The command may have
	// template placeholders ({{.input}}) so we just run it as-is; if it
	// fails due to missing input that's expected and we note a non-zero exit.
	cmd := exec.CommandContext(ctx, "sh", "-c", t.Command)

	output, err := cmd.CombinedOutput()

	if ctx.Err() == context.DeadlineExceeded {
		return TestResult{
			ToolName: t.Name,
			Check:    "dryrun:execute",
			Status:   "fail",
			Error:    "command timed out after 5s",
		}
	}

	if err != nil {
		return TestResult{
			ToolName: t.Name,
			Check:    "dryrun:execute",
			Status:   "fail",
			Error:    fmt.Sprintf("exit error: %v (output: %s)", err, truncate(string(output), 200)),
		}
	}

	return TestResult{
		ToolName: t.Name,
		Check:    "dryrun:execute",
		Status:   "pass",
	}
}

// checkDependencies verifies all declared dependencies exist in the registry.
func checkDependencies(def *PluginDef, registry []RegistryEntry) []TestResult {
	if len(def.Dependencies) == 0 {
		return nil
	}

	regMap := make(map[string]RegistryEntry, len(registry))
	for _, e := range registry {
		regMap[e.Name] = e
	}

	var results []TestResult

	for _, dep := range def.Dependencies {
		entry, ok := regMap[dep.Name]
		if !ok {
			if dep.Optional {
				results = append(results, TestResult{
					Check:  fmt.Sprintf("dependency:%s", dep.Name),
					Status: "pass",
					Error:  "optional dependency not found in registry (skipped)",
				})
			} else {
				results = append(results, TestResult{
					Check:  fmt.Sprintf("dependency:%s", dep.Name),
					Status: "fail",
					Error:  fmt.Sprintf("required dependency %q not found in registry", dep.Name),
				})
			}

			continue
		}

		// Check version constraint if specified.
		if dep.Version != "" && !SatisfiesConstraint(entry.Version, dep.Version) {
			results = append(results, TestResult{
				Check:  fmt.Sprintf("dependency:%s", dep.Name),
				Status: "fail",
				Error:  fmt.Sprintf("registry version %q does not satisfy constraint %q", entry.Version, dep.Version),
			})
		} else {
			results = append(results, TestResult{
				Check:  fmt.Sprintf("dependency:%s", dep.Name),
				Status: "pass",
			})
		}
	}

	return results
}

// truncate shortens a string to maxLen, appending "..." if truncated.
func truncate(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	if len(s) <= maxLen {
		return s
	}

	return s[:maxLen] + "..."
}

// HasFailures returns true if any TestResult has Status "fail".
func HasFailures(results []TestResult) bool {
	for _, r := range results {
		if r.Status == "fail" {
			return true
		}
	}

	return false
}
