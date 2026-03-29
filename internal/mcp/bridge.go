// Package mcp implements the MCP server bridge — stdio transport that calls
// service layers directly (single-binary, no daemon).
package mcp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/inovacc/thimble/internal/delegate"
	"github.com/inovacc/thimble/internal/executor"
	"github.com/inovacc/thimble/internal/hooklog"
	"github.com/inovacc/thimble/internal/model"
	"github.com/inovacc/thimble/internal/paths"
	"github.com/inovacc/thimble/internal/platform"
	"github.com/inovacc/thimble/internal/plugin"
	"github.com/inovacc/thimble/internal/routing"
	"github.com/inovacc/thimble/internal/security"
	"github.com/inovacc/thimble/internal/session"
	"github.com/inovacc/thimble/internal/store"
	"github.com/inovacc/thimble/internal/tracing"
)

// sessionStats tracks per-tool call counts and byte volumes.
type sessionStats struct {
	mu             sync.Mutex
	calls          map[string]int
	bytesReturned  map[string]int
	bytesIndexed   int
	bytesSandboxed int
	sessionStart   time.Time
}

func newSessionStats() *sessionStats {
	return &sessionStats{
		calls:         make(map[string]int),
		bytesReturned: make(map[string]int),
		sessionStart:  time.Now(),
	}
}

func (s *sessionStats) snapshot() map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()

	return map[string]any{
		"calls":          s.calls,
		"bytesReturned":  s.bytesReturned,
		"bytesIndexed":   s.bytesIndexed,
		"bytesSandboxed": s.bytesSandboxed,
		"uptimeSeconds":  int(time.Since(s.sessionStart).Seconds()),
	}
}

const (
	serverName    = "thimble"
	serverVersion = "4.0.0"
)

// Bridge holds the MCP server and direct service references.
type Bridge struct {
	server           *mcp.Server
	content          *store.ContentStore
	sharedStore      *store.ContentStore // lazy-initialized cross-session shared store
	sharedOnce       sync.Once           // ensures shared store opened at most once
	session          *session.SessionDB
	executor         *executor.PolyglotExecutor
	delegate         *delegate.Manager
	hookLog          *hooklog.Logger
	rateLimiter      *RateLimiter
	throttler        *searchThrottler
	stats            *sessionStats
	progress         ProgressReporter
	logger           *slog.Logger
	projectDir       string              // per-project content store isolation
	workspace        *session.Workspace  // detected multi-project workspace
	sessionID        string              // deterministic hash(app + projectDir)
	detectedPlatform platform.PlatformID // platform detected from MCP clientInfo
	tracingShutdown  func(context.Context) error
	pluginConflicts  []plugin.PluginConflict // detected plugin tool conflicts
	goalMu           sync.RWMutex
	activeGoal       string // current goal tag for session events
}

// getSharedStore returns the shared ContentStore, initializing it lazily on first access.
// If sharedStore was pre-set (e.g. in tests), it is returned directly without opening a new one.
func (b *Bridge) getSharedStore() (*store.ContentStore, error) {
	var initErr error

	b.sharedOnce.Do(func() {
		// If already injected (e.g. by tests), skip initialization.
		if b.sharedStore != nil {
			return
		}

		cs, err := store.OpenShared()
		if err != nil {
			initErr = err
			return
		}

		b.sharedStore = cs
	})

	if initErr != nil {
		return nil, initErr
	}

	if b.sharedStore == nil {
		return nil, fmt.Errorf("shared store not initialized")
	}

	return b.sharedStore, nil
}

// SessionID returns the deterministic session identifier.
func (b *Bridge) SessionID() string { return b.sessionID }

// ActiveGoal returns the current active goal tag (empty if none).
func (b *Bridge) ActiveGoal() string {
	b.goalMu.RLock()
	defer b.goalMu.RUnlock()

	return b.activeGoal
}

// SetActiveGoal sets the current active goal tag.
func (b *Bridge) SetActiveGoal(goal string) {
	b.goalMu.Lock()
	defer b.goalMu.Unlock()

	b.activeGoal = goal
}

// SetProgressReporter replaces the progress reporter. Primarily used in tests.
func (b *Bridge) SetProgressReporter(r ProgressReporter) {
	if r == nil {
		r = NoopReporter{}
	}

	b.progress = r
}

// progressCtxFromRequest extracts the server session and progress token from
// an MCP CallToolRequest and stores them in the context for downstream use
// by the ProgressReporter.
func progressCtxFromRequest(ctx context.Context, req *mcp.CallToolRequest) context.Context {
	if req == nil || req.Session == nil {
		return ctx
	}

	token := req.Params.GetProgressToken()

	return withProgress(ctx, req.Session, token)
}

// sessionHash produces a deterministic 16-char hex ID from the executable path and project dir.
func sessionHash(appPath, projectDir string) string {
	h := sha256.Sum256([]byte(appPath + "\x00" + projectDir))
	return hex.EncodeToString(h[:8])
}

// New creates a new MCP bridge with direct service references.
// It opens the content store, session DB, executor, and hook log directly.
func New() (*Bridge, error) {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	projectDir, _ := os.Getwd()
	exe, _ := os.Executable()

	// One-time migration: projects/ → sessions/.
	paths.MigrateProjectsToSessions()

	// Detect multi-project workspace (monorepo / multi-root).
	ws, err := session.DetectWorkspace(projectDir)
	if err != nil {
		logger.Warn("workspace detection failed, using single project", "error", err)

		ws = &session.Workspace{
			RootDir:  projectDir,
			Projects: []string{projectDir},
			Type:     session.WorkspaceSingle,
		}
	}

	// Compute session directory using workspace key for multi-project support.
	wsKey := session.ProjectKey(ws)

	sessionDir := filepath.Join(paths.PluginDataDir(), "sessions", wsKey)
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		return nil, fmt.Errorf("create session dir: %w", err)
	}

	// Open content store.
	cs, err := store.New(filepath.Join(sessionDir, "content.db"))
	if err != nil {
		return nil, fmt.Errorf("open content store: %w", err)
	}

	if ep := store.NewEmbeddingProvider(); ep != nil {
		cs.SetEmbeddingProvider(ep)

		logger.Info("embedding provider configured from env", "info", store.DetectedProviderInfo(ep))
	} else if ep := store.DetectEmbeddingProvider(context.Background()); ep != nil {
		cs.SetEmbeddingProvider(ep)

		logger.Info("embedding provider auto-detected", "info", store.DetectedProviderInfo(ep))
	} else {
		logger.Debug("no embedding provider available (env or auto-detect)")
	}

	// Open session DB.
	sessDB, err := session.New(filepath.Join(sessionDir, "session.db"))
	if err != nil {
		cs.Close()
		return nil, fmt.Errorf("open session db: %w", err)
	}

	// Create executor.
	exec := executor.New(&executor.Options{Runtimes: executor.DetectRuntimes()})

	// Open hook log.
	hl, _ := hooklog.New(paths.DataDir())

	// Create delegate manager.
	delegateMgr := delegate.NewManager(exec, logger)

	// Initialize tracing (opt-in via THIMBLE_TRACING=1).
	var tracingShutdown func(context.Context) error

	if tracing.Enabled() {
		shutdown, err := tracing.Init(context.Background(), serverName, serverVersion)
		if err != nil {
			logger.Warn("tracing init failed, continuing without tracing", "error", err)
		} else {
			tracingShutdown = shutdown

			logger.Info("tracing enabled")
		}
	}

	b := &Bridge{
		content:          cs,
		session:          sessDB,
		executor:         exec,
		delegate:         delegateMgr,
		hookLog:          hl,
		rateLimiter:      RateLimitFromEnv(),
		throttler:        newThrottler(),
		stats:            newSessionStats(),
		progress:         &MCPReporter{Logger: logger},
		logger:           logger,
		projectDir:       projectDir,
		workspace:        ws,
		sessionID:        sessionHash(exe, projectDir),
		detectedPlatform: platform.PlatformClaudeCode,
		tracingShutdown:  tracingShutdown,
	}

	srv := mcp.NewServer(
		&mcp.Implementation{
			Name:    serverName,
			Version: serverVersion,
		},
		&mcp.ServerOptions{
			Logger: logger,
			InitializedHandler: func(_ context.Context, req *mcp.InitializedRequest) {
				if req.Session == nil {
					return
				}

				params := req.Session.InitializeParams()
				if params == nil || params.ClientInfo == nil {
					return
				}

				b.SetPlatform(DetectPlatformFromClientInfo(params.ClientInfo.Name))
				logger.Info("platform detected from clientInfo",
					"clientName", params.ClientInfo.Name,
					"platform", b.detectedPlatform)
			},
		},
	)

	b.server = srv
	b.registerTools()
	b.registerReportTools()
	b.registerGitTools()
	b.registerGhTools()
	b.registerLintTools()
	b.registerGitHubTools()
	b.registerPluginTools()
	b.registerSharedTools()
	b.registerResources()

	return b, nil
}

// NewForTest creates a Bridge with injected dependencies for testing.
func NewForTest(cs *store.ContentStore, sessDB *session.SessionDB, exec *executor.PolyglotExecutor) *Bridge {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	b := &Bridge{
		content:          cs,
		session:          sessDB,
		executor:         exec,
		throttler:        newThrottler(),
		stats:            newSessionStats(),
		progress:         NoopReporter{},
		logger:           logger,
		detectedPlatform: platform.PlatformClaudeCode,
	}

	srv := mcp.NewServer(
		&mcp.Implementation{
			Name:    serverName,
			Version: serverVersion,
		},
		&mcp.ServerOptions{
			Logger: logger,
		},
	)

	b.server = srv
	b.registerTools()
	b.registerReportTools()
	b.registerGitTools()
	b.registerGhTools()
	b.registerLintTools()
	b.registerGitHubTools()
	b.registerPluginTools()
	b.registerSharedTools()
	b.registerResources()

	return b
}

// SetPlatform overrides the detected platform for the bridge.
func (b *Bridge) SetPlatform(id platform.PlatformID) {
	b.detectedPlatform = id
}

// recordToolCall captures an MCP tool invocation into the session for audit/tracking.
func (b *Bridge) recordToolCall(_ context.Context, toolName string, isQuery bool) {
	if b.sessionID == "" {
		return
	}

	// Update in-memory stats.
	b.stats.mu.Lock()
	b.stats.calls[toolName]++
	b.stats.mu.Unlock()

	// Record as session event directly.
	_ = b.session.EnsureSession("default", b.projectDir)

	data := fmt.Sprintf(`{"tool":"%s","session":"%s","is_query":%v}`, toolName, b.sessionID, isQuery)

	if goal := b.ActiveGoal(); goal != "" {
		data = fmt.Sprintf(`{"tool":"%s","session":"%s","is_query":%v,"goal":"%s"}`, toolName, b.sessionID, isQuery, goal)
	}

	ev := model.SessionEvent{
		Type:     "mcp_tool_call",
		Category: toolName,
		Data:     data,
		Priority: 2,
	}

	_ = b.session.InsertEvent("default", ev, "mcp")
}

// Run starts the MCP server on stdio and blocks until the context is cancelled.
// Background goroutines monitor parent process and watch plugins.
func (b *Bridge) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go b.monitorParent(ctx, cancel)
	go b.handleSignals(ctx, cancel)
	go b.watchPlugins(ctx)

	// For MCP-only platforms without SessionStart hooks, write routing instructions.
	b.writeRoutingIfNeeded()

	return b.server.Run(ctx, &mcp.StdioTransport{})
}

// parentCheckInterval is how often to check if the parent process is still alive.
const parentCheckInterval = 30 * time.Second

// monitorParent checks periodically whether the parent process has changed (died).
// If the parent PID changes or stdin is closed, it triggers a clean shutdown.
func (b *Bridge) monitorParent(ctx context.Context, cancel context.CancelFunc) {
	originalPPID := os.Getppid()

	// On Windows, PPID 0 means the parent is the System process (orphaned).
	if runtime.GOOS == "windows" && (originalPPID == 0 || originalPPID == 1) {
		b.logger.Warn("parent process is System (orphaned), shutting down", "ppid", originalPPID)
		cancel()

		return
	}

	ticker := time.NewTicker(parentCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if os.Getppid() != originalPPID {
				b.logger.Warn("parent process changed, shutting down",
					"original_ppid", originalPPID,
					"current_ppid", os.Getppid())
				cancel()

				return
			}
		}
	}
}

// Server returns the underlying MCP server for testing with in-memory transports.
func (b *Bridge) Server() *mcp.Server {
	return b.server
}

// checkCommandDeny evaluates a command against deny-only security policies.
// Returns an error if the command is denied.
func (b *Bridge) checkCommandDeny(command string) error {
	policies := security.ReadBashPolicies(b.projectDir, "")
	decision := security.EvaluateCommandDenyOnly(command, policies)

	if decision.Decision == security.Deny {
		return fmt.Errorf("command denied by policy %q", decision.MatchedPattern)
	}

	return nil
}

// checkFilePathDeny evaluates a file path against deny-only security policies.
// Returns an error if the path is denied.
func (b *Bridge) checkFilePathDeny(filePath string) error {
	denyGlobs := security.ReadToolDenyPatterns("Edit", b.projectDir, "")
	decision := security.EvaluateFilePath(filePath, denyGlobs)

	if decision.Denied {
		return fmt.Errorf("file path denied by policy %q", decision.MatchedPattern)
	}

	return nil
}

// writeRoutingIfNeeded writes routing instructions for platforms that don't have
// SessionStart hook capability (MCP-only platforms).
func (b *Bridge) writeRoutingIfNeeded() {
	if b.projectDir == "" {
		return
	}

	adapter, err := platform.Get(b.detectedPlatform)
	if err != nil {
		return
	}

	// Only write routing if the platform doesn't have SessionStart hooks.
	if adapter.Capabilities().SessionStart {
		return
	}

	_, _ = routing.WriteInstructions(b.projectDir, b.detectedPlatform)
}

// Close releases all owned resources (stores, session DB, hook log, delegate manager, tracing).
func (b *Bridge) Close() {
	if b.tracingShutdown != nil {
		_ = b.tracingShutdown(context.Background())
	}

	if b.delegate != nil {
		b.delegate.Shutdown()
	}

	if b.content != nil {
		b.content.Close()
	}

	if b.sharedStore != nil {
		b.sharedStore.Close()
	}

	if b.session != nil {
		b.session.Close()
	}

	if b.hookLog != nil {
		_ = b.hookLog.Close()
	}
}
