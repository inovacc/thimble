// Package ghcli wraps the GitHub CLI (gh) as a subprocess.
package ghcli

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// ExecResult holds the output of a gh CLI invocation.
type ExecResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
	TimedOut bool
}

// Exec runs the gh CLI with the given arguments in the specified directory.
// If jsonFields is non-empty, "--json <fields>" is appended automatically.
// A zero or negative timeout defaults to 30 seconds.
func Exec(ctx context.Context, dir string, args []string, jsonFields []string, timeoutMs int64) (*ExecResult, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("args required (e.g. [\"pr\", \"list\"])")
	}

	if len(jsonFields) > 0 {
		args = append(args, "--json", strings.Join(jsonFields, ","))
	}

	timeout := time.Duration(timeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(execCtx, "gh", args...)
	if dir != "" {
		cmd.Dir = dir
	}

	var stdout, stderr strings.Builder

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	resp := &ExecResult{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}

	if execCtx.Err() == context.DeadlineExceeded {
		resp.TimedOut = true
		resp.ExitCode = 124

		return resp, nil
	}

	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			resp.Stderr = "gh CLI not found in PATH. Install from https://cli.github.com/ or set GH_TOKEN for API-only access."
			resp.ExitCode = 1

			return resp, nil
		}

		var ee *exec.ExitError
		if errors.As(err, &ee) {
			resp.ExitCode = ee.ExitCode()
		} else {
			return nil, fmt.Errorf("gh exec: %w", err)
		}
	}

	return resp, nil
}
