package executor

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/inovacc/thimble/internal/model"
)

// OutputChunk represents a piece of subprocess output.
type OutputChunk struct {
	Data      string
	Stream    string // "stdout" or "stderr"
	Timestamp time.Time
}

// ExecuteStream runs code and sends output chunks via callback.
// The callback must be safe for concurrent use (called from two goroutines).
func (pe *PolyglotExecutor) ExecuteStream(
	ctx context.Context,
	language, code string,
	timeout time.Duration,
	onChunk func(OutputChunk) error,
) (model.ExecResult, error) {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}

	tmpDir, err := os.MkdirTemp("", "thimble-exec-")
	if err != nil {
		return model.ExecResult{}, fmt.Errorf("mkdtemp: %w", err)
	}

	defer func() { _ = os.RemoveAll(tmpDir) }()

	filePath, err := pe.writeScript(tmpDir, code, language)
	if err != nil {
		return model.ExecResult{}, err
	}

	cmd, err := BuildCommand(pe.runtimes, language, filePath)
	if err != nil {
		return model.ExecResult{}, err
	}

	// Rust: compile then run with streaming.
	if cmd[0] == "__rust_compile_run__" {
		return pe.compileAndRunStream(ctx, filePath, tmpDir, timeout, onChunk)
	}

	// Shell runs in project dir; other languages run in temp dir.
	cwd := tmpDir
	if language == "shell" {
		cwd = pe.projectRoot
	}

	return pe.spawnStream(ctx, cmd, cwd, timeout, onChunk)
}

// ExecuteFileStream runs code with file content injected, streaming output via callback.
func (pe *PolyglotExecutor) ExecuteFileStream(
	ctx context.Context,
	path, language, code string,
	timeout time.Duration,
	onChunk func(OutputChunk) error,
) (model.ExecResult, error) {
	absolutePath := path
	if !filepath.IsAbs(path) {
		absolutePath = filepath.Join(pe.projectRoot, path)
	}

	wrappedCode := wrapWithFileContent(absolutePath, language, code)

	return pe.ExecuteStream(ctx, language, wrappedCode, timeout, onChunk)
}

func (pe *PolyglotExecutor) compileAndRunStream(ctx context.Context, srcPath, cwd string, timeout time.Duration, onChunk func(OutputChunk) error) (model.ExecResult, error) {
	binSuffix := ""
	if runtime.GOOS == "windows" {
		binSuffix = ".exe"
	}

	binPath := strings.TrimSuffix(srcPath, ".rs") + binSuffix

	compileCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	compileCmd := exec.CommandContext(compileCtx, "rustc", srcPath, "-o", binPath)
	compileCmd.Dir = cwd

	var stderrBuf strings.Builder

	compileCmd.Stderr = &stderrBuf

	if err := compileCmd.Run(); err != nil {
		errMsg := "Compilation failed:\n" + stderrBuf.String()
		_ = onChunk(OutputChunk{
			Data:      errMsg,
			Stream:    "stderr",
			Timestamp: time.Now(),
		})

		return model.ExecResult{
			Stderr:   errMsg,
			ExitCode: 1,
		}, nil
	}

	return pe.spawnStream(ctx, []string{binPath}, cwd, timeout, onChunk)
}

func (pe *PolyglotExecutor) spawnStream(
	ctx context.Context,
	cmd []string,
	cwd string,
	timeout time.Duration,
	onChunk func(OutputChunk) error,
) (model.ExecResult, error) {
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	c := exec.CommandContext(execCtx, cmd[0], cmd[1:]...)
	c.Dir = cwd
	c.Env = pe.buildSafeEnv(cwd)

	setProcGroup(c)
	c.Cancel = func() error { killProcessGroup(c); return nil }
	c.WaitDelay = 2 * time.Second

	stdoutPipe, err := c.StdoutPipe()
	if err != nil {
		return model.ExecResult{}, fmt.Errorf("stdout pipe: %w", err)
	}

	stderrPipe, err := c.StderrPipe()
	if err != nil {
		return model.ExecResult{}, fmt.Errorf("stderr pipe: %w", err)
	}

	if err := c.Start(); err != nil {
		return model.ExecResult{}, fmt.Errorf("start: %w", err)
	}

	var (
		stdoutBuf  strings.Builder
		stderrBuf  strings.Builder
		totalBytes atomic.Int64
		wg         sync.WaitGroup
	)

	// Scanner goroutine for a pipe.
	scanPipe := func(scanner *bufio.Scanner, stream string, buf *strings.Builder) {
		defer wg.Done()

		for scanner.Scan() {
			line := scanner.Text()
			now := time.Now()

			buf.WriteString(line)
			buf.WriteString("\n")

			newTotal := totalBytes.Add(int64(len(line) + 1))
			if int(newTotal) > pe.hardCapBytes {
				// Hard cap exceeded — kill the process group.
				killProcessGroup(c)

				return
			}

			_ = onChunk(OutputChunk{
				Data:      line,
				Stream:    stream,
				Timestamp: now,
			})
		}
	}

	wg.Add(2)

	go scanPipe(bufio.NewScanner(stdoutPipe), "stdout", &stdoutBuf)
	go scanPipe(bufio.NewScanner(stderrPipe), "stderr", &stderrBuf)

	// Wait for both scanners to drain remaining data first.
	// Scanners finish when the process exits and closes the write end of
	// the pipe.  We must drain before c.Wait() because Wait closes the
	// read end (via closeAfterWait), which would truncate unread data.
	wg.Wait()

	// Now wait for the process to fully exit and collect its status.
	waitErr := c.Wait()

	timedOut := execCtx.Err() == context.DeadlineExceeded

	exitCode := 0

	if waitErr != nil {
		exitErr := &exec.ExitError{}
		if errors.As(waitErr, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}

	if timedOut {
		exitCode = 1
	}

	return model.ExecResult{
		Stdout:   smartTruncate(stdoutBuf.String(), pe.maxOutputBytes),
		Stderr:   smartTruncate(stderrBuf.String(), pe.maxOutputBytes),
		ExitCode: exitCode,
		TimedOut: timedOut,
	}, nil
}
