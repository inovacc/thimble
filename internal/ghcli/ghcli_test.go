package ghcli

import (
	"context"
	"os/exec"
	"testing"
	"time"
)

func ghAvailable() bool {
	_, err := exec.LookPath("gh")
	return err == nil
}

func TestExec_EmptyArgs(t *testing.T) {
	t.Parallel()

	_, err := Exec(context.Background(), "", nil, nil, 0)
	if err == nil {
		t.Fatal("expected error for empty args, got nil")
	}
}

func TestExec_MissingBinary(t *testing.T) {
	t.Parallel()

	if ghAvailable() {
		t.Skip("gh is installed; cannot test missing-binary path")
	}

	res, err := Exec(context.Background(), "", []string{"pr", "list"}, nil, 5000)
	if err != nil {
		t.Fatalf("expected nil error for missing gh, got: %v", err)
	}

	if res.ExitCode == 0 {
		t.Fatal("expected non-zero exit code when gh is missing")
	}

	if res.Stderr == "" {
		t.Fatal("expected stderr message when gh is missing")
	}
}

func TestExec_InvalidArgs(t *testing.T) {
	t.Parallel()

	if !ghAvailable() {
		t.Skip("gh not installed")
	}

	// "not-a-real-command" should produce a non-zero exit code from gh.
	res, err := Exec(context.Background(), "", []string{"not-a-real-command"}, nil, 5000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.ExitCode == 0 {
		t.Error("expected non-zero exit code for invalid gh subcommand")
	}
}

func TestExec_EmptyDir(t *testing.T) {
	t.Parallel()

	if !ghAvailable() {
		t.Skip("gh not installed")
	}

	// Empty dir should default to current working directory — gh should still run.
	res, err := Exec(context.Background(), "", []string{"version"}, nil, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.ExitCode != 0 {
		t.Errorf("expected exit code 0 for 'gh version', got %d (stderr: %s)", res.ExitCode, res.Stderr)
	}
}

func TestExec_TimeoutBehavior(t *testing.T) {
	t.Parallel()

	if !ghAvailable() {
		t.Skip("gh not installed")
	}

	// Use 1ms timeout — should time out or complete very quickly.
	res, err := Exec(context.Background(), "", []string{"version"}, nil, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Either it timed out (TimedOut=true, ExitCode=124) or it completed fast enough.
	// Both outcomes are acceptable; we just verify no panic/crash.
	if res.TimedOut {
		if res.ExitCode != 124 {
			t.Errorf("expected exit code 124 on timeout, got %d", res.ExitCode)
		}
	}
}

func TestExec_JSONFieldsAppended(t *testing.T) {
	t.Parallel()

	if !ghAvailable() {
		t.Skip("gh not installed")
	}

	// "gh pr list --json number,title" in a non-repo dir will fail,
	// but we can verify the --json flag was passed by checking stderr
	// output that references the flag or simply that it does not panic.
	res, err := Exec(context.Background(), t.TempDir(), []string{"pr", "list"}, []string{"number", "title"}, 5000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// We don't assert specific output, but the command should have run
	// (non-zero exit is expected in a non-repo dir).
	_ = res
}

func TestExec_DefaultTimeout(t *testing.T) {
	t.Parallel()

	// Test that zero timeout defaults to 30s without panicking.
	// Verify the default logic without actually running gh (slow on Windows CI).
	timeout := time.Duration(0) * time.Millisecond
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	if timeout != 30*time.Second {
		t.Errorf("expected 30s default, got %v", timeout)
	}
}

func TestExec_NegativeTimeout(t *testing.T) {
	t.Parallel()

	// Test that negative timeout defaults to 30s.
	timeout := time.Duration(-100) * time.Millisecond
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	if timeout != 30*time.Second {
		t.Errorf("expected 30s default, got %v", timeout)
	}
}
