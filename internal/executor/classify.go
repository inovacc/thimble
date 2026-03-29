package executor

import (
	"fmt"
	"strings"
)

// ExitClassification holds the result of classifying a non-zero exit code.
type ExitClassification struct {
	IsError bool
	Output  string
}

// ClassifyNonZeroExit determines whether a non-zero exit code is a real error.
// Shell commands like `grep` exit 1 for "no matches" — not a real error.
// Exit code 1 is treated as a soft failure when:
//   - language is "shell"
//   - exit code is exactly 1
//   - stdout has non-whitespace content
func ClassifyNonZeroExit(language string, exitCode int, stdout, stderr string) ExitClassification {
	isSoftFail := language == "shell" && exitCode == 1 && strings.TrimSpace(stdout) != ""

	if isSoftFail {
		return ExitClassification{IsError: false, Output: stdout}
	}

	return ExitClassification{
		IsError: true,
		Output:  fmt.Sprintf("Exit code: %d\n\nstdout:\n%s\n\nstderr:\n%s", exitCode, stdout, stderr),
	}
}

// ClassifyError returns a human-readable error classification based on stderr/stdout
// patterns and exit code. This is used by the MCP bridge when explain_errors is enabled.
func ClassifyError(stderr, stdout string, exitCode int) string {
	combined := stderr + stdout

	switch {
	case strings.Contains(combined, "permission denied") || strings.Contains(combined, "Permission denied"):
		return "PERMISSION_ERROR: Access denied. Check file permissions or run with appropriate privileges."
	case strings.Contains(combined, "No such file") || strings.Contains(combined, "not found"):
		return "NOT_FOUND: File or command not found. Verify the path exists."
	case strings.Contains(combined, "syntax error") || strings.Contains(combined, "SyntaxError"):
		return "SYNTAX_ERROR: Code has a syntax error. Check the highlighted line."
	case strings.Contains(combined, "import") && (strings.Contains(combined, "cannot find") || strings.Contains(combined, "ModuleNotFoundError")):
		return "IMPORT_ERROR: Missing dependency. Install the required package."
	case strings.Contains(combined, "timeout") || strings.Contains(combined, "Timeout"):
		return "TIMEOUT: Execution timed out. Consider increasing timeout or optimizing the code."
	case strings.Contains(combined, "out of memory") || strings.Contains(combined, "OOM"):
		return "MEMORY_ERROR: Out of memory. Reduce data size or increase memory limits."
	case exitCode == 1:
		return "GENERAL_ERROR: Process exited with code 1. Check stderr for details."
	case exitCode == 2:
		return "USAGE_ERROR: Invalid arguments or usage. Check command syntax."
	case exitCode == 126:
		return "NOT_EXECUTABLE: File exists but is not executable."
	case exitCode == 127:
		return "COMMAND_NOT_FOUND: Command not found in PATH."
	case exitCode == 137:
		return "KILLED: Process was killed (likely OOM or signal 9)."
	case exitCode == 139:
		return "SEGFAULT: Segmentation fault. Likely a bug in native code."
	default:
		return fmt.Sprintf("EXIT_%d: Process exited with code %d.", exitCode, exitCode)
	}
}
