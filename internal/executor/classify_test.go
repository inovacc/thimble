package executor

import (
	"strings"
	"testing"
)

func TestClassifyNonZeroExit(t *testing.T) {
	tests := []struct {
		name     string
		language string
		exitCode int
		stdout   string
		stderr   string
		wantErr  bool
	}{
		{
			name:     "shell exit 1 with stdout is soft fail",
			language: "shell",
			exitCode: 1,
			stdout:   "some output\n",
			stderr:   "",
			wantErr:  false,
		},
		{
			name:     "shell exit 1 with no stdout is real error",
			language: "shell",
			exitCode: 1,
			stdout:   "",
			stderr:   "command not found",
			wantErr:  true,
		},
		{
			name:     "shell exit 1 with whitespace-only stdout is real error",
			language: "shell",
			exitCode: 1,
			stdout:   "   \n\t  ",
			stderr:   "",
			wantErr:  true,
		},
		{
			name:     "shell exit 2 is real error regardless",
			language: "shell",
			exitCode: 2,
			stdout:   "some output",
			stderr:   "bad usage",
			wantErr:  true,
		},
		{
			name:     "python exit 1 is real error not shell",
			language: "python",
			exitCode: 1,
			stdout:   "some output",
			stderr:   "traceback...",
			wantErr:  true,
		},
		{
			name:     "shell exit 0 should not happen but returns error format",
			language: "shell",
			exitCode: 0,
			stdout:   "output",
			stderr:   "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyNonZeroExit(tt.language, tt.exitCode, tt.stdout, tt.stderr)

			if got.IsError != tt.wantErr {
				t.Errorf("IsError = %v, want %v", got.IsError, tt.wantErr)
			}

			if !tt.wantErr {
				// Soft fail: output should be just stdout
				if got.Output != tt.stdout {
					t.Errorf("Output = %q, want %q", got.Output, tt.stdout)
				}
			} else {
				// Real error: output should contain exit code
				if got.Output == "" {
					t.Error("Output should not be empty for real errors")
				}
			}
		})
	}
}

func TestClassifyError(t *testing.T) {
	tests := []struct {
		name       string
		stderr     string
		stdout     string
		exitCode   int
		wantPrefix string
	}{
		{
			name:       "permission denied in stderr",
			stderr:     "bash: /etc/shadow: Permission denied",
			stdout:     "",
			exitCode:   1,
			wantPrefix: "PERMISSION_ERROR",
		},
		{
			name:       "permission denied lowercase",
			stderr:     "open: permission denied",
			stdout:     "",
			exitCode:   1,
			wantPrefix: "PERMISSION_ERROR",
		},
		{
			name:       "file not found",
			stderr:     "cat: /tmp/missing: No such file or directory",
			stdout:     "",
			exitCode:   1,
			wantPrefix: "NOT_FOUND",
		},
		{
			name:       "command not found in stderr",
			stderr:     "foobar: not found",
			stdout:     "",
			exitCode:   127,
			wantPrefix: "NOT_FOUND",
		},
		{
			name:       "syntax error python",
			stderr:     "SyntaxError: invalid syntax",
			stdout:     "",
			exitCode:   1,
			wantPrefix: "SYNTAX_ERROR",
		},
		{
			name:       "syntax error shell",
			stderr:     "bash: syntax error near unexpected token",
			stdout:     "",
			exitCode:   2,
			wantPrefix: "SYNTAX_ERROR",
		},
		{
			name:       "import error python",
			stderr:     "ModuleNotFoundError: No module named 'pandas'\nimport pandas",
			stdout:     "",
			exitCode:   1,
			wantPrefix: "IMPORT_ERROR",
		},
		{
			name:       "import error go",
			stderr:     "cannot find package \"github.com/foo/bar\"\nimport \"github.com/foo/bar\"",
			stdout:     "",
			exitCode:   1,
			wantPrefix: "IMPORT_ERROR",
		},
		{
			name:       "timeout error",
			stderr:     "context deadline exceeded: timeout",
			stdout:     "",
			exitCode:   1,
			wantPrefix: "TIMEOUT",
		},
		{
			name:       "out of memory",
			stderr:     "fatal: out of memory",
			stdout:     "",
			exitCode:   1,
			wantPrefix: "MEMORY_ERROR",
		},
		{
			name:       "OOM killed",
			stderr:     "OOM killer invoked",
			stdout:     "",
			exitCode:   137,
			wantPrefix: "MEMORY_ERROR",
		},
		{
			name:       "generic exit 1 no pattern match",
			stderr:     "something went wrong",
			stdout:     "",
			exitCode:   1,
			wantPrefix: "GENERAL_ERROR",
		},
		{
			name:       "exit code 2 with syntax error pattern",
			stderr:     "syntax error: unexpected flag",
			stdout:     "",
			exitCode:   2,
			wantPrefix: "SYNTAX_ERROR",
		},
		{
			name:       "exit code 2 no pattern",
			stderr:     "failed",
			stdout:     "",
			exitCode:   2,
			wantPrefix: "USAGE_ERROR",
		},
		{
			name:       "exit code 126 not executable",
			stderr:     "some error",
			stdout:     "",
			exitCode:   126,
			wantPrefix: "NOT_EXECUTABLE",
		},
		{
			name:       "exit code 127 command not found",
			stderr:     "",
			stdout:     "",
			exitCode:   127,
			wantPrefix: "COMMAND_NOT_FOUND",
		},
		{
			name:       "exit code 137 killed",
			stderr:     "",
			stdout:     "",
			exitCode:   137,
			wantPrefix: "KILLED",
		},
		{
			name:       "exit code 139 segfault",
			stderr:     "",
			stdout:     "",
			exitCode:   139,
			wantPrefix: "SEGFAULT",
		},
		{
			name:       "unknown exit code",
			stderr:     "",
			stdout:     "",
			exitCode:   42,
			wantPrefix: "EXIT_42",
		},
		{
			name:       "pattern in stdout not stderr",
			stderr:     "",
			stdout:     "Permission denied for file /etc/passwd",
			exitCode:   1,
			wantPrefix: "PERMISSION_ERROR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyError(tt.stderr, tt.stdout, tt.exitCode)

			if !strings.HasPrefix(got, tt.wantPrefix) {
				t.Errorf("ClassifyError() = %q, want prefix %q", got, tt.wantPrefix)
			}
		})
	}
}
