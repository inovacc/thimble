// Package delegate provides background task management for delegated execution.
package delegate

import (
	"context"
	"crypto/rand"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/inovacc/thimble/internal/executor"
)

const (
	// DefaultMaxTasks is the maximum number of concurrent delegated tasks.
	DefaultMaxTasks = 5
	// MaxOutputBytes caps stdout/stderr per task to prevent memory exhaustion.
	MaxOutputBytes = 1024 * 1024 // 1MB
)

// TaskEntry holds the state of a single delegated task.
type TaskEntry struct {
	mu          sync.Mutex
	ID          string
	Description string
	Status      string // "running", "completed", "failed", "cancelled"
	ProgressPct int
	Stdout      strings.Builder
	Stderr      strings.Builder
	ExitCode    int
	TimedOut    bool
	ErrorMsg    string
	StartedAt   time.Time
	FinishedAt  time.Time
	cancel      context.CancelFunc
}

// TaskSummary is a lightweight view of a task for listing.
type TaskSummary struct {
	ID          string
	Description string
	Status      string
	ProgressPct int
	StartedAt   time.Time
}

// StartTaskResult is returned when a new task is submitted.
type StartTaskResult struct {
	TaskID string
	Status string
}

// TaskStatus is returned when checking a task's progress.
type TaskStatus struct {
	TaskID      string
	Description string
	Status      string
	ProgressPct int
	Stdout      string
	Stderr      string
	ExitCode    int
	TimedOut    bool
	ErrorMsg    string
	StartedAt   time.Time
	FinishedAt  time.Time
}

// Manager manages background tasks using a polyglot executor.
type Manager struct {
	exec     *executor.PolyglotExecutor
	tasks    sync.Map // map[string]*TaskEntry
	wg       sync.WaitGroup
	maxTasks int
	logger   *slog.Logger
}

// NewManager creates a new task manager.
func NewManager(exec *executor.PolyglotExecutor, logger *slog.Logger) *Manager {
	return &Manager{
		exec:     exec,
		maxTasks: DefaultMaxTasks,
		logger:   logger,
	}
}

func generateTaskID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)

	return fmt.Sprintf("%x", b)
}

func (m *Manager) activeCount() int {
	count := 0

	m.tasks.Range(func(_, v any) bool {
		e := v.(*TaskEntry)
		e.mu.Lock()
		if e.Status == "running" {
			count++
		}
		e.mu.Unlock()

		return true
	})

	return count
}

// StartTask creates a new background task and returns its ID immediately.
func (m *Manager) StartTask(language, code, description string, timeoutMs int64) (*StartTaskResult, error) {
	if code == "" {
		return nil, fmt.Errorf("code is required")
	}

	if language == "" {
		return nil, fmt.Errorf("language is required")
	}

	if m.activeCount() >= m.maxTasks {
		return nil, fmt.Errorf("max concurrent tasks reached (%d)", m.maxTasks)
	}

	taskID := generateTaskID()

	entry := &TaskEntry{
		ID:          taskID,
		Description: description,
		Status:      "running",
		StartedAt:   time.Now(),
	}

	m.tasks.Store(taskID, entry)

	timeout := time.Duration(timeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = executor.DefaultTimeout
	}

	m.wg.Add(1)

	go m.runTask(entry, language, code, timeout)

	m.logger.Info("task started", "task_id", taskID, "language", language, "description", description)

	return &StartTaskResult{TaskID: taskID, Status: "running"}, nil
}

// GetTaskStatus returns the current state of a task.
func (m *Manager) GetTaskStatus(taskID string) (*TaskStatus, error) {
	v, ok := m.tasks.Load(taskID)
	if !ok {
		return nil, fmt.Errorf("task not found")
	}

	e := v.(*TaskEntry)
	e.mu.Lock()
	defer e.mu.Unlock()

	return &TaskStatus{
		TaskID:      e.ID,
		Description: e.Description,
		Status:      e.Status,
		ProgressPct: e.ProgressPct,
		Stdout:      e.Stdout.String(),
		Stderr:      e.Stderr.String(),
		ExitCode:    e.ExitCode,
		TimedOut:    e.TimedOut,
		ErrorMsg:    e.ErrorMsg,
		StartedAt:   e.StartedAt,
		FinishedAt:  e.FinishedAt,
	}, nil
}

// CancelTask cancels a running task. Returns true if cancelled.
func (m *Manager) CancelTask(taskID string) (bool, string) {
	v, ok := m.tasks.Load(taskID)
	if !ok {
		return false, "task not found"
	}

	e := v.(*TaskEntry)
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.Status != "running" {
		return false, fmt.Sprintf("task is already %s", e.Status)
	}

	if e.cancel != nil {
		e.cancel()
	}

	e.Status = "cancelled"
	e.FinishedAt = time.Now()
	e.ProgressPct = 100

	return true, ""
}

// ListTasks returns summaries of all tasks, optionally filtered by status.
func (m *Manager) ListTasks(statusFilter string) []TaskSummary {
	var tasks []TaskSummary

	m.tasks.Range(func(_, v any) bool {
		e := v.(*TaskEntry)
		e.mu.Lock()
		defer e.mu.Unlock()

		if statusFilter != "" && e.Status != statusFilter {
			return true
		}

		tasks = append(tasks, TaskSummary{
			ID:          e.ID,
			Description: e.Description,
			Status:      e.Status,
			ProgressPct: e.ProgressPct,
			StartedAt:   e.StartedAt,
		})

		return true
	})

	return tasks
}

// Shutdown cancels all running tasks and waits for goroutines to finish.
func (m *Manager) Shutdown() {
	m.tasks.Range(func(_, v any) bool {
		e := v.(*TaskEntry)
		e.mu.Lock()
		if e.Status == "running" && e.cancel != nil {
			e.cancel()
			e.Status = "cancelled"
			e.FinishedAt = time.Now()
		}
		e.mu.Unlock()

		return true
	})

	m.wg.Wait()
}

func (m *Manager) runTask(entry *TaskEntry, language, code string, timeout time.Duration) {
	defer m.wg.Done()

	ctx, cancel := context.WithCancel(context.Background())

	entry.mu.Lock()
	entry.cancel = cancel
	entry.mu.Unlock()

	defer cancel()

	result, err := m.exec.ExecuteStream(ctx, language, code, timeout, func(chunk executor.OutputChunk) error {
		entry.mu.Lock()
		defer entry.mu.Unlock()

		switch chunk.Stream {
		case "stdout":
			if entry.Stdout.Len() < MaxOutputBytes {
				entry.Stdout.WriteString(chunk.Data)
				entry.Stdout.WriteString("\n")
			}
		case "stderr":
			if entry.Stderr.Len() < MaxOutputBytes {
				entry.Stderr.WriteString(chunk.Data)
				entry.Stderr.WriteString("\n")
			}
		}

		return nil
	})

	entry.mu.Lock()
	defer entry.mu.Unlock()

	entry.FinishedAt = time.Now()
	entry.ExitCode = result.ExitCode
	entry.TimedOut = result.TimedOut

	if entry.Status == "cancelled" {
		return
	}

	switch {
	case err != nil:
		entry.Status = "failed"
		entry.ErrorMsg = err.Error()
	case result.ExitCode != 0:
		entry.Status = "failed"
	default:
		entry.Status = "completed"
	}

	entry.ProgressPct = 100

	m.logger.Info("task finished", "task_id", entry.ID, "status", entry.Status, "exit_code", entry.ExitCode)
}
