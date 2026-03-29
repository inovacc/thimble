// Package hooklog provides append-only JSONL logging for hook interactions.
package hooklog

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const logFileName = "hooklog.jsonl"

// Entry represents a single hook interaction log record.
type Entry struct {
	Timestamp   time.Time `json:"timestamp"`
	Platform    string    `json:"platform"`
	Event       string    `json:"event"`
	SessionID   string    `json:"session_id,omitempty"`
	ProjectDir  string    `json:"project_dir,omitempty"`
	ToolName    string    `json:"tool_name,omitempty"`
	ToolInput   string    `json:"tool_input,omitempty"`
	Blocked     bool      `json:"blocked,omitempty"`
	Reason      string    `json:"reason,omitempty"`
	HasContext  bool      `json:"has_context,omitempty"`
	DurationMs  int64     `json:"duration_ms"`
	Error       string    `json:"error,omitempty"`
	Response    string    `json:"response,omitempty"`
	GRPCPayload string    `json:"grpc_payload,omitempty"`
}

// ReadOptions controls filtering when reading log entries.
type ReadOptions struct {
	Limit       int
	Platform    string
	Event       string
	ToolName    string
	BlockedOnly bool
	ProjectDir  string
}

// Logger is an append-only JSONL hook logger.
type Logger struct {
	mu    sync.Mutex
	path  string
	debug bool
}

// New creates a Logger that writes to hooklog.jsonl in the given data directory.
func New(dataDir string) (*Logger, error) {
	p := filepath.Join(dataDir, logFileName)
	return &Logger{path: p}, nil
}

// SetDebug enables or disables debug mode (full payload logging).
func (l *Logger) SetDebug(v bool) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.debug = v
}

// Debug returns whether debug mode is enabled.
func (l *Logger) Debug() bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	return l.debug
}

// Path returns the log file path.
func (l *Logger) Path() string {
	return l.path
}

// Log appends an entry to the log file.
func (l *Logger) Log(e Entry) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now()
	}

	// Strip large payloads in non-debug mode.
	if !l.debug {
		e.GRPCPayload = ""
		e.Response = ""
		e.ToolInput = ""
	}

	data, err := json.Marshal(e)
	if err != nil {
		return
	}

	f, err := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}

	defer func() { _ = f.Close() }()

	_, _ = fmt.Fprintf(f, "%s\n", data)
}

// Read returns log entries matching the given options.
func (l *Logger) Read(opts *ReadOptions) ([]Entry, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	data, err := os.ReadFile(l.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("read hook log: %w", err)
	}

	var all []Entry

	for _, line := range splitLines(data) {
		if len(line) == 0 {
			continue
		}

		var e Entry
		if err := json.Unmarshal(line, &e); err != nil {
			continue
		}

		if opts != nil {
			if opts.Platform != "" && e.Platform != opts.Platform {
				continue
			}

			if opts.Event != "" && e.Event != opts.Event {
				continue
			}

			if opts.ToolName != "" && e.ToolName != opts.ToolName {
				continue
			}

			if opts.BlockedOnly && !e.Blocked {
				continue
			}

			if opts.ProjectDir != "" && e.ProjectDir != opts.ProjectDir {
				continue
			}
		}

		all = append(all, e)
	}

	// Apply limit (return last N entries).
	limit := 100
	if opts != nil && opts.Limit > 0 {
		limit = opts.Limit
	}

	if len(all) > limit {
		all = all[len(all)-limit:]
	}

	return all, nil
}

// Stats returns aggregate counts: total, allowed, blocked, errored.
func (l *Logger) Stats() (total, allowed, blocked, errored int) {
	l.mu.Lock()
	defer l.mu.Unlock()

	data, err := os.ReadFile(l.path)
	if err != nil {
		return 0, 0, 0, 0
	}

	for _, line := range splitLines(data) {
		if len(line) == 0 {
			continue
		}

		var e Entry
		if err := json.Unmarshal(line, &e); err != nil {
			continue
		}

		total++

		switch {
		case e.Blocked:
			blocked++
		case e.Error != "":
			errored++
		default:
			allowed++
		}
	}

	return total, allowed, blocked, errored
}

// Clear removes all log entries.
func (l *Logger) Clear() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	return os.Remove(l.path)
}

// Close is a no-op — the logger writes are append-only and don't hold open handles.
func (l *Logger) Close() error {
	return nil
}

// splitLines splits byte data into individual lines.
func splitLines(data []byte) [][]byte {
	var lines [][]byte

	start := 0

	for i, b := range data {
		if b == '\n' {
			lines = append(lines, data[start:i])
			start = i + 1
		}
	}

	if start < len(data) {
		lines = append(lines, data[start:])
	}

	return lines
}
