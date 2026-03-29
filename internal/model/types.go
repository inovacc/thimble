// Package model contains domain types shared across thimble packages.
package model

// ToolCall represents a tool invocation during event extraction.
type ToolCall struct {
	ToolName     string         `json:"toolName"`
	ToolInput    map[string]any `json:"toolInput"`
	ToolResponse string         `json:"toolResponse,omitempty"`
	IsError      bool           `json:"isError,omitempty"`
}

// UserMessage represents a user message during event extraction.
type UserMessage struct {
	Content   string `json:"content"`
	Timestamp string `json:"timestamp,omitempty"`
}

// SessionEvent as stored in SessionDB.
type SessionEvent struct {
	Type     string `json:"type"`
	Category string `json:"category"`
	Data     string `json:"data"`
	Priority int    `json:"priority"`
	DataHash string `json:"data_hash,omitempty"`
}

// StoredEvent is a full row from the session_events table.
type StoredEvent struct {
	ID         int64  `json:"id"`
	SessionID  string `json:"session_id"`
	Type       string `json:"type"`
	Category   string `json:"category"`
	Priority   int    `json:"priority"`
	Data       string `json:"data"`
	SourceHook string `json:"source_hook"`
	CreatedAt  string `json:"created_at"`
	DataHash   string `json:"data_hash"`
}

// SessionMeta is a row from the session_meta table.
type SessionMeta struct {
	SessionID    string `json:"session_id"`
	ProjectDir   string `json:"project_dir"`
	StartedAt    string `json:"started_at"`
	LastEventAt  string `json:"last_event_at"`
	EventCount   int    `json:"event_count"`
	CompactCount int    `json:"compact_count"`
}

// SessionActivity tracks when a session was last used for eviction.
type SessionActivity struct {
	SessionID  string `json:"session_id"`
	AppName    string `json:"app_name"`
	ProjectDir string `json:"project_dir"`
	LastUsedAt string `json:"last_used_at"`
	ToolCalls  int    `json:"tool_calls"`
	Queries    int    `json:"queries"`
	CreatedAt  string `json:"created_at"`
}

// ToolCount holds a tool name and its invocation count.
type ToolCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// ResumeRow is a row from the session_resume table.
type ResumeRow struct {
	Snapshot   string `json:"snapshot"`
	EventCount int    `json:"event_count"`
	Consumed   int    `json:"consumed"`
}

// ExecResult returned by PolyglotExecutor after running a code snippet.
type ExecResult struct {
	Stdout       string `json:"stdout"`
	Stderr       string `json:"stderr"`
	ExitCode     int    `json:"exitCode"`
	TimedOut     bool   `json:"timedOut"`
	Backgrounded bool   `json:"backgrounded,omitempty"`
}

// IndexResult returned after indexing content into the knowledge base.
type IndexResult struct {
	SourceID    int64  `json:"sourceId"`
	Label       string `json:"label"`
	TotalChunks int    `json:"totalChunks"`
	CodeChunks  int    `json:"codeChunks"`
}

// SearchResult returned from FTS5 BM25-ranked lookup.
type SearchResult struct {
	Title       string  `json:"title"`
	Content     string  `json:"content"`
	Source      string  `json:"source"`
	Rank        float64 `json:"rank"`
	ContentType string  `json:"contentType"` // "code" or "prose"
	MatchLayer  string  `json:"matchLayer,omitempty"`
	Highlighted string  `json:"highlighted,omitempty"`
}

// StoreStats — aggregate statistics for a ContentStore instance.
type StoreStats struct {
	Sources    int `json:"sources"`
	Chunks     int `json:"chunks"`
	CodeChunks int `json:"codeChunks"`
}

// ResumeSnapshot for injecting into a new conversation as context.
type ResumeSnapshot struct {
	GeneratedAt string         `json:"generatedAt"`
	Summary     string         `json:"summary"`
	Events      []SessionEvent `json:"events"`
}

// EventPriority levels.
const (
	PriorityLow      = 1
	PriorityNormal   = 2
	PriorityHigh     = 3
	PriorityCritical = 4
)

// HookInput is the JSON payload received by hook dispatchers.
type HookInput struct {
	Platform   string          `json:"platform"`
	Event      string          `json:"event"`
	ToolCall   *ToolCall       `json:"toolCall,omitempty"`
	Message    *UserMessage    `json:"message,omitempty"`
	SessionID  string          `json:"sessionId,omitempty"`
	ProjectDir string          `json:"projectDir,omitempty"`
	Extra      map[string]any  `json:"extra,omitempty"`
}

// HookOutput is the JSON response returned by hook dispatchers.
type HookOutput struct {
	Result  any    `json:"result,omitempty"`
	Blocked bool   `json:"blocked,omitempty"`
	Reason  string `json:"reason,omitempty"`
}
