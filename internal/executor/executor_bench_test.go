package executor

import (
	"context"
	"testing"
	"time"
)

// BenchmarkExecuteShell benchmarks executing a simple echo command.
func BenchmarkExecuteShell(b *testing.B) {
	pe := New(&Options{
		MaxOutputBytes: DefaultMaxOutputBytes,
		HardCapBytes:   DefaultHardCapBytes,
	})

	ctx := context.Background()

	// Warm up: verify shell is available.
	result, err := pe.Execute(ctx, "shell", "echo hello", 10*time.Second, false)
	if err != nil {
		b.Skipf("shell not available: %v", err)
	}

	if result.ExitCode != 0 {
		b.Skipf("shell echo failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}

	b.ResetTimer()

	for range b.N {
		_, _ = pe.Execute(ctx, "shell", "echo bench", 10*time.Second, false)
	}
}

// BenchmarkClassifyLanguage benchmarks the ClassifyNonZeroExit and ClassifyError
// functions which perform language/error classification on execution results.
func BenchmarkClassifyLanguage(b *testing.B) {
	b.Run("ClassifyNonZeroExit", func(b *testing.B) {
		stdout := "some output from the command\nline two\nline three\n"
		stderr := "warning: something happened\n"

		b.ResetTimer()

		for range b.N {
			_ = ClassifyNonZeroExit("shell", 1, stdout, stderr)
			_ = ClassifyNonZeroExit("python", 1, "", stderr)
			_ = ClassifyNonZeroExit("shell", 2, stdout, stderr)
		}
	})

	b.Run("ClassifyError", func(b *testing.B) {
		cases := []struct {
			stderr   string
			stdout   string
			exitCode int
		}{
			{"permission denied", "", 1},
			{"No such file or directory", "", 127},
			{"SyntaxError: invalid syntax", "", 1},
			{"ModuleNotFoundError: No module named 'foo'", "", 1},
			{"", "normal output", 0},
			{"timeout exceeded", "", 1},
			{"", "", 137},
			{"", "", 139},
		}

		b.ResetTimer()

		for range b.N {
			for _, c := range cases {
				_ = ClassifyError(c.stderr, c.stdout, c.exitCode)
			}
		}
	})
}
