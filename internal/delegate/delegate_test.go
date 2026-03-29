package delegate

import (
	"log/slog"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/inovacc/thimble/internal/executor"
)

// newTestManager creates a Manager with a real PolyglotExecutor suitable for tests.
func newTestManager(t *testing.T) *Manager {
	t.Helper()

	exec := executor.New(&executor.Options{
		MaxOutputBytes: 1024,
		HardCapBytes:   1024 * 1024,
	})

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	return NewManager(exec, logger)
}


func TestNewManager(t *testing.T) {
	t.Parallel()

	m := newTestManager(t)
	if m == nil {
		t.Fatal("expected non-nil manager")
	}

	if m.maxTasks != DefaultMaxTasks {
		t.Errorf("expected maxTasks=%d, got %d", DefaultMaxTasks, m.maxTasks)
	}

	if m.exec == nil {
		t.Fatal("expected non-nil executor")
	}
}

func TestStartTask(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		language  string
		code      string
		desc      string
		wantErr   bool
		errSubstr string
	}{
		{
			name:     "success case returns ID",
			language: "shell",
			code:     "echo hello",
			desc:     "test echo",
		},
		{
			name:      "empty code errors",
			language:  "shell",
			code:      "",
			desc:      "empty code",
			wantErr:   true,
			errSubstr: "code is required",
		},
		{
			name:      "empty language errors",
			language:  "",
			code:      "echo hello",
			desc:      "empty language",
			wantErr:   true,
			errSubstr: "language is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			m := newTestManager(t)
			defer m.Shutdown()

			result, err := m.StartTask(tt.language, tt.code, tt.desc, 5000)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}

				if tt.errSubstr != "" && !strings.Contains(err.Error(), tt.errSubstr) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errSubstr)
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.TaskID == "" {
				t.Error("expected non-empty task ID")
			}

			if result.Status != "running" {
				t.Errorf("expected status=running, got %s", result.Status)
			}
		})
	}
}

func TestStartTaskMaxConcurrent(t *testing.T) {
	t.Parallel()

	m := newTestManager(t)
	defer m.Shutdown()

	// Use python sleep which terminates cleanly on cancel.
	lang, code := "python", "import time; time.sleep(30)"

	// Fill all 5 slots with long-running tasks.
	for range DefaultMaxTasks {
		_, err := m.StartTask(lang, code, "filler", 60000)
		if err != nil {
			t.Fatalf("failed to start task: %v", err)
		}
	}

	// Give goroutines a moment to register as running.
	time.Sleep(500 * time.Millisecond)

	// The 6th task should be rejected.
	_, err := m.StartTask(lang, code, "overflow", 60000)
	if err == nil {
		t.Fatal("expected error when exceeding max concurrent tasks")
	}

	if !strings.Contains(err.Error(), "max concurrent tasks") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestGetTaskStatus(t *testing.T) {
	t.Parallel()

	t.Run("returns status for valid task", func(t *testing.T) {
		t.Parallel()

		m := newTestManager(t)
		defer m.Shutdown()

		result, err := m.StartTask("shell", "echo hello", "echo test", 5000)
		if err != nil {
			t.Fatalf("start task: %v", err)
		}

		status, err := m.GetTaskStatus(result.TaskID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if status.TaskID != result.TaskID {
			t.Errorf("expected task ID %q, got %q", result.TaskID, status.TaskID)
		}

		if status.Description != "echo test" {
			t.Errorf("expected description 'echo test', got %q", status.Description)
		}
	})

	t.Run("returns error for unknown task ID", func(t *testing.T) {
		t.Parallel()

		m := newTestManager(t)
		defer m.Shutdown()

		_, err := m.GetTaskStatus("nonexistent-task-id")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestGetTaskStatusCompleted(t *testing.T) {
	t.Parallel()

	m := newTestManager(t)
	defer m.Shutdown()

	result, err := m.StartTask("shell", "echo done", "quick task", 5000)
	if err != nil {
		t.Fatalf("start task: %v", err)
	}

	// Poll until the task completes (max 10 seconds).
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		status, err := m.GetTaskStatus(result.TaskID)
		if err != nil {
			t.Fatalf("get status: %v", err)
		}

		if status.Status == "completed" {
			if status.ProgressPct != 100 {
				t.Errorf("expected progress 100, got %d", status.ProgressPct)
			}

			return
		}

		if status.Status == "failed" {
			t.Fatalf("task failed: exit=%d stderr=%s err=%s", status.ExitCode, status.Stderr, status.ErrorMsg)
		}

		time.Sleep(100 * time.Millisecond)
	}

	t.Fatal("task did not complete within deadline")
}

func TestCancelTask(t *testing.T) {
	t.Parallel()

	t.Run("cancel running task returns true", func(t *testing.T) {
		t.Parallel()

		m := newTestManager(t)
		defer m.Shutdown()

		// Use python sleep — terminates cleanly on context cancel.
		result, err := m.StartTask("python", "import time; time.sleep(30)", "long task", 60000)
		if err != nil {
			t.Fatalf("start task: %v", err)
		}

		// Give the goroutine a moment to start.
		time.Sleep(500 * time.Millisecond)

		ok, reason := m.CancelTask(result.TaskID)
		if !ok {
			t.Errorf("expected cancel to succeed, got reason=%q", reason)
		}

		status, err := m.GetTaskStatus(result.TaskID)
		if err != nil {
			t.Fatalf("get status after cancel: %v", err)
		}

		if status.Status != "cancelled" {
			t.Errorf("expected status=cancelled, got %s", status.Status)
		}
	})

	t.Run("cancel non-existent task returns false", func(t *testing.T) {
		t.Parallel()

		m := newTestManager(t)
		defer m.Shutdown()

		ok, reason := m.CancelTask("does-not-exist")
		if ok {
			t.Error("expected cancel to fail for non-existent task")
		}

		if !strings.Contains(reason, "task not found") {
			t.Errorf("unexpected reason: %q", reason)
		}
	})

	t.Run("cancel already-completed task returns false", func(t *testing.T) {
		t.Parallel()

		m := newTestManager(t)
		defer m.Shutdown()

		result, err := m.StartTask("shell", "echo quick", "fast task", 5000)
		if err != nil {
			t.Fatalf("start task: %v", err)
		}

		// Wait for completion.
		deadline := time.Now().Add(10 * time.Second)
		for time.Now().Before(deadline) {
			status, _ := m.GetTaskStatus(result.TaskID)
			if status != nil && status.Status != "running" {
				break
			}

			time.Sleep(100 * time.Millisecond)
		}

		ok, reason := m.CancelTask(result.TaskID)
		if ok {
			t.Error("expected cancel to fail for completed task")
		}

		if !strings.Contains(reason, "already") {
			t.Errorf("unexpected reason: %q", reason)
		}
	})
}

func TestListTasks(t *testing.T) {
	t.Parallel()

	t.Run("list all tasks", func(t *testing.T) {
		t.Parallel()

		m := newTestManager(t)
		defer m.Shutdown()

		// Use echo — fast tasks are fine, we just need them registered.
		_, err := m.StartTask("shell", "echo task-a", "task-a", 5000)
		if err != nil {
			t.Fatalf("start task: %v", err)
		}

		_, err = m.StartTask("shell", "echo task-b", "task-b", 5000)
		if err != nil {
			t.Fatalf("start task: %v", err)
		}

		tasks := m.ListTasks("")
		if len(tasks) != 2 {
			t.Errorf("expected 2 tasks, got %d", len(tasks))
		}
	})

	t.Run("filter by status", func(t *testing.T) {
		t.Parallel()

		m := newTestManager(t)
		defer m.Shutdown()

		// Start a long-running task.
		_, err := m.StartTask("python", "import time; time.sleep(30)", "slow", 60000)
		if err != nil {
			t.Fatalf("start task: %v", err)
		}

		// Start a quick task and wait for it to finish.
		quickResult, err := m.StartTask("shell", "echo fast", "fast", 5000)
		if err != nil {
			t.Fatalf("start task: %v", err)
		}

		deadline := time.Now().Add(10 * time.Second)
		for time.Now().Before(deadline) {
			status, _ := m.GetTaskStatus(quickResult.TaskID)
			if status != nil && status.Status != "running" {
				break
			}

			time.Sleep(100 * time.Millisecond)
		}

		// Filter for running tasks only.
		running := m.ListTasks("running")
		for _, task := range running {
			if task.Status != "running" {
				t.Errorf("expected all filtered tasks to be running, got %s", task.Status)
			}
		}

		// Filter for completed tasks only.
		completed := m.ListTasks("completed")
		for _, task := range completed {
			if task.Status != "completed" {
				t.Errorf("expected all filtered tasks to be completed, got %s", task.Status)
			}
		}
	})

	t.Run("empty list", func(t *testing.T) {
		t.Parallel()

		m := newTestManager(t)
		defer m.Shutdown()

		tasks := m.ListTasks("")
		if len(tasks) != 0 {
			t.Errorf("expected 0 tasks on fresh manager, got %d", len(tasks))
		}
	})
}

func TestShutdown(t *testing.T) {
	t.Parallel()

	m := newTestManager(t)

	// Use python for cancellable long tasks.
	lang, code := "python", "import time; time.sleep(30)"

	taskIDs := make([]string, 0, 3)

	for range 3 {
		result, err := m.StartTask(lang, code, "shutdown-test", 60000)
		if err != nil {
			t.Fatalf("start task: %v", err)
		}

		taskIDs = append(taskIDs, result.TaskID)
	}

	// Give goroutines a moment to start.
	time.Sleep(500 * time.Millisecond)

	// Shutdown should cancel all running tasks and return.
	done := make(chan struct{})

	go func() {
		m.Shutdown()
		close(done)
	}()

	select {
	case <-done:
		// Shutdown completed.
	case <-time.After(15 * time.Second):
		t.Fatal("Shutdown did not complete within 15 seconds")
	}

	// Verify all tasks are cancelled.
	for _, id := range taskIDs {
		status, err := m.GetTaskStatus(id)
		if err != nil {
			t.Fatalf("get status: %v", err)
		}

		if status.Status != "cancelled" {
			t.Errorf("task %s: expected status=cancelled, got %s", id, status.Status)
		}
	}
}

func TestConcurrentStartTask(t *testing.T) {
	t.Parallel()

	m := newTestManager(t)
	defer m.Shutdown()

	var wg sync.WaitGroup

	results := make(chan *StartTaskResult, DefaultMaxTasks)
	errs := make(chan error, DefaultMaxTasks)

	for range DefaultMaxTasks {
		wg.Go(func() {
			result, err := m.StartTask("shell", "echo concurrent", "concurrent", 5000)
			if err != nil {
				errs <- err
				return
			}

			results <- result
		})
	}

	wg.Wait()
	close(results)
	close(errs)

	// Collect unique task IDs to verify no duplicates.
	seen := make(map[string]bool)

	for r := range results {
		if seen[r.TaskID] {
			t.Errorf("duplicate task ID: %s", r.TaskID)
		}

		seen[r.TaskID] = true
	}

	for err := range errs {
		t.Logf("concurrent start error (may be expected): %v", err)
	}
}

func TestTaskOutputCapture(t *testing.T) {
	t.Parallel()

	m := newTestManager(t)
	defer m.Shutdown()

	result, err := m.StartTask("shell", "echo captured-output", "output test", 5000)
	if err != nil {
		t.Fatalf("start task: %v", err)
	}

	// Wait for completion.
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		status, err := m.GetTaskStatus(result.TaskID)
		if err != nil {
			t.Fatalf("get status: %v", err)
		}

		if status.Status == "completed" {
			if !strings.Contains(status.Stdout, "captured-output") {
				t.Errorf("expected stdout to contain 'captured-output', got %q", status.Stdout)
			}

			return
		}

		if status.Status == "failed" {
			t.Fatalf("task failed: exit=%d stderr=%s err=%s", status.ExitCode, status.Stderr, status.ErrorMsg)
		}

		time.Sleep(100 * time.Millisecond)
	}

	t.Fatal("task did not complete within deadline")
}

func TestGenerateTaskID(t *testing.T) {
	t.Parallel()

	ids := make(map[string]bool)

	for range 100 {
		id := generateTaskID()
		if id == "" {
			t.Fatal("generated empty task ID")
		}

		if len(id) != 32 { // 16 bytes = 32 hex chars
			t.Errorf("expected 32 char hex ID, got len=%d id=%s", len(id), id)
		}

		if ids[id] {
			t.Errorf("duplicate ID generated: %s", id)
		}

		ids[id] = true
	}
}
