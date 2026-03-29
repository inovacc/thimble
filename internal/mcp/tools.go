package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/inovacc/thimble/internal/analysis"
	"github.com/inovacc/thimble/internal/executor"
	"github.com/inovacc/thimble/internal/fetch"
	"github.com/inovacc/thimble/internal/model"
	"github.com/inovacc/thimble/internal/session"
)

const (
	// SearchTotalOutputCap is the maximum total bytes of search output before
	// remaining queries are skipped.
	SearchTotalOutputCap = 40 * 1024

	// BatchTimeoutMs is the cumulative timeout for batch execute operations.
	BatchTimeoutMs = 120_000

	// IntentSearchThreshold is the minimum output size (bytes) before
	// intent-based filtering kicks in. Below this threshold, raw output is returned.
	IntentSearchThreshold = 5120
)

// ── Tool Input Types ──

type executeInput struct {
	Language      string `json:"language" jsonschema:"the programming language (shell python javascript typescript go rust ruby php perl r elixir)"`
	Code          string `json:"code" jsonschema:"the code to execute"`
	TimeoutMs     int    `json:"timeout_ms,omitempty" jsonschema:"execution timeout in milliseconds (default 30000)"`
	Background    bool   `json:"background,omitempty" jsonschema:"run in background without waiting for completion"`
	ExplainErrors bool   `json:"explain_errors,omitempty" jsonschema:"auto-classify error output when exit code is non-zero"`
	Intent        string `json:"intent,omitempty" jsonschema:"describe what you are looking for — large outputs will be intent-filtered via BM25 preview"`
}

type executeFileInput struct {
	Language      string `json:"language" jsonschema:"the programming language"`
	FilePath      string `json:"file_path" jsonschema:"path to the file to process"`
	Code          string `json:"code" jsonschema:"code that processes FILE_CONTENT variable"`
	TimeoutMs     int    `json:"timeout_ms,omitempty" jsonschema:"execution timeout in milliseconds"`
	ExplainErrors bool   `json:"explain_errors,omitempty" jsonschema:"auto-classify error output when exit code is non-zero"`
	Intent        string `json:"intent,omitempty" jsonschema:"describe what you are looking for — large outputs will be intent-filtered via BM25 preview"`
}

type indexInput struct {
	Content     string `json:"content" jsonschema:"the content to index"`
	Label       string `json:"label" jsonschema:"source label for later reference"`
	ContentType string `json:"content_type,omitempty" jsonschema:"content type: markdown or plain or json (default markdown)"`
}

type searchInput struct {
	Query   string   `json:"query,omitempty" jsonschema:"single search query (convenience alias for queries)"`
	Queries []string `json:"queries,omitempty" jsonschema:"search queries to run against the knowledge base"`
	Limit   int      `json:"limit,omitempty" jsonschema:"max results per query (default 5)"`
	Intent  string   `json:"intent,omitempty" jsonschema:"describe what you are looking for to get better snippets"`
	Source  string   `json:"source,omitempty" jsonschema:"filter results to a specific source label"`
}

type fetchAndIndexInput struct {
	URL   string `json:"url" jsonschema:"the URL to fetch and index"`
	Label string `json:"label,omitempty" jsonschema:"optional label (defaults to URL)"`
}

type batchExecuteInput struct {
	Commands      []batchCommand `json:"commands,omitempty" jsonschema:"shell commands to execute and auto-index"`
	Queries       []string       `json:"queries,omitempty" jsonschema:"search queries to run after commands complete"`
	ExplainErrors bool           `json:"explain_errors,omitempty" jsonschema:"auto-classify error output when exit code is non-zero"`
}

type batchCommand struct {
	Command string `json:"command" jsonschema:"shell command to execute"`
	Label   string `json:"label,omitempty" jsonschema:"label for indexing output"`
}

// UnmarshalJSON allows batchCommand to accept either a plain string or a JSON object.
// A plain string is treated as {"command": "<string>"}.
func (bc *batchCommand) UnmarshalJSON(data []byte) error {
	// Try as string first.
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		bc.Command = s
		return nil
	}

	// Fall back to object.
	type alias batchCommand

	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}

	*bc = batchCommand(a)

	return nil
}

type analyzeInput struct {
	RootDir     string   `json:"root_dir,omitempty" jsonschema:"project root directory to analyze (default: current directory)"`
	Paths       []string `json:"paths,omitempty" jsonschema:"specific files or directories to analyze (empty = full project)"`
	Incremental bool     `json:"incremental,omitempty" jsonschema:"only analyze files changed since last git commit"`
}

type symbolsInput struct {
	Query        string `json:"query,omitempty" jsonschema:"name substring to search for"`
	Kind         string `json:"kind,omitempty" jsonschema:"filter by kind: function method type struct interface constant variable"`
	Package      string `json:"package,omitempty" jsonschema:"filter by package name"`
	ExportedOnly bool   `json:"exported_only,omitempty" jsonschema:"only return exported symbols"`
	Limit        int    `json:"limit,omitempty" jsonschema:"max results (default 50)"`
}

type delegateInput struct {
	Description string `json:"description" jsonschema:"human-readable description of the task"`
	Language    string `json:"language" jsonschema:"programming language (shell python javascript typescript go rust ruby php perl r elixir)"`
	Code        string `json:"code" jsonschema:"the code to execute in the background"`
	TimeoutMs   int    `json:"timeout_ms,omitempty" jsonschema:"execution timeout in milliseconds (default 30000)"`
}

type delegateStatusInput struct {
	TaskID string `json:"task_id" jsonschema:"the task ID returned by ctx_delegate"`
}

type delegateCancelInput struct {
	TaskID string `json:"task_id" jsonschema:"the task ID to cancel"`
}

type delegateListInput struct {
	StatusFilter string `json:"status_filter,omitempty" jsonschema:"filter by status: running completed failed cancelled (empty = all)"`
}

type upgradeInput struct{}

type statsInput struct{}

type doctorInput struct{}

type setGoalInput struct {
	Goal string `json:"goal" jsonschema:"the goal to tag subsequent events with (e.g. 'implement auth module')"`
}

type clearGoalInput struct{}

type sessionInsightsInput struct {
	SessionID string `json:"session_id,omitempty" jsonschema:"session ID to analyze (default: current session's 'default')"`
}

type workspaceInfoInput struct{}

// coerceJSONArray handles double-serialized JSON arrays — when a string value
// starts with "[", try to unmarshal it as []string. Returns the original value
// if it's already a []string, or the parsed result if it was a JSON string.
func coerceJSONArray(v any) []string {
	// Already a []string.
	if arr, ok := v.([]string); ok {
		return arr
	}
	// Already a []any (common from JSON unmarshal).
	if arr, ok := v.([]any); ok {
		result := make([]string, 0, len(arr))
		for _, item := range arr {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}

		return result
	}
	// Double-serialized: string starting with "[".
	if s, ok := v.(string); ok && len(s) > 0 && s[0] == '[' {
		var arr []string
		if err := json.Unmarshal([]byte(s), &arr); err == nil {
			return arr
		}
	}

	return nil
}

// registerTools registers all MCP tools on the server.
func (b *Bridge) registerTools() {
	mcpsdk.AddTool(b.server, &mcpsdk.Tool{
		Name:        "ctx_execute",
		Description: "Execute code in a sandboxed environment. Supports 11 languages. Output is auto-indexed into the knowledge base.",
	}, withRateLimit(b, "ctx_execute", b.handleExecute))

	mcpsdk.AddTool(b.server, &mcpsdk.Tool{
		Name:        "ctx_execute_file",
		Description: "Execute code with access to a file's content via FILE_CONTENT variable. Output is auto-indexed.",
	}, withRateLimit(b, "ctx_execute_file", b.handleExecuteFile))

	mcpsdk.AddTool(b.server, &mcpsdk.Tool{
		Name:        "ctx_index",
		Description: "Index content into the knowledge base for later search. Supports markdown, plain text, and JSON.",
	}, withRateLimit(b, "ctx_index", b.handleIndex))

	mcpsdk.AddTool(b.server, &mcpsdk.Tool{
		Name:        "ctx_search",
		Description: "Search the knowledge base using FTS5 with 4-layer fallback (Porter AND → OR → Trigram AND → OR → Fuzzy).",
	}, withRateLimit(b, "ctx_search", b.handleSearch))

	mcpsdk.AddTool(b.server, &mcpsdk.Tool{
		Name:        "ctx_fetch_and_index",
		Description: "Fetch a URL, convert HTML to Markdown, and index it into the knowledge base.",
	}, withRateLimit(b, "ctx_fetch_and_index", b.handleFetchAndIndex))

	mcpsdk.AddTool(b.server, &mcpsdk.Tool{
		Name:        "ctx_batch_execute",
		Description: "Execute multiple commands and search queries in one call. Primary research tool — runs commands, auto-indexes output, and searches.",
	}, withRateLimit(b, "ctx_batch_execute", b.handleBatchExecute))

	mcpsdk.AddTool(b.server, &mcpsdk.Tool{
		Name:        "ctx_stats",
		Description: "Show knowledge base statistics: sources, chunks, code chunks.",
	}, withRateLimit(b, "ctx_stats", b.handleStats))

	mcpsdk.AddTool(b.server, &mcpsdk.Tool{
		Name:        "ctx_doctor",
		Description: "Health check: reports server status, knowledge base stats, throttle state, and runtime info.",
	}, withRateLimit(b, "ctx_doctor", b.handleDoctor))

	mcpsdk.AddTool(b.server, &mcpsdk.Tool{
		Name:        "ctx_analyze",
		Description: "Analyze codebase structure: parse source files, extract symbols (functions, types, interfaces), and index into knowledge base.",
	}, withRateLimit(b, "ctx_analyze", b.handleAnalyze))

	mcpsdk.AddTool(b.server, &mcpsdk.Tool{
		Name:        "ctx_symbols",
		Description: "Query extracted code symbols by name, kind, or package. Returns signatures, locations, and documentation.",
	}, withRateLimit(b, "ctx_symbols", b.handleSymbols))

	mcpsdk.AddTool(b.server, &mcpsdk.Tool{
		Name:        "ctx_upgrade",
		Description: "Show the install/upgrade command for thimble and the current version.",
	}, withRateLimit(b, "ctx_upgrade", b.handleUpgrade))

	mcpsdk.AddTool(b.server, &mcpsdk.Tool{
		Name:        "ctx_delegate",
		Description: "Start a background task. Returns a task_id immediately. Use ctx_delegate_status to poll for completion.",
	}, withRateLimit(b, "ctx_delegate", b.handleDelegate))

	mcpsdk.AddTool(b.server, &mcpsdk.Tool{
		Name:        "ctx_delegate_status",
		Description: "Get the status and output of a delegated background task.",
	}, withRateLimit(b, "ctx_delegate_status", b.handleDelegateStatus))

	mcpsdk.AddTool(b.server, &mcpsdk.Tool{
		Name:        "ctx_delegate_cancel",
		Description: "Cancel a running delegated background task.",
	}, withRateLimit(b, "ctx_delegate_cancel", b.handleDelegateCancel))

	mcpsdk.AddTool(b.server, &mcpsdk.Tool{
		Name:        "ctx_delegate_list",
		Description: "List all delegated tasks, optionally filtered by status.",
	}, withRateLimit(b, "ctx_delegate_list", b.handleDelegateList))

	mcpsdk.AddTool(b.server, &mcpsdk.Tool{
		Name:        "ctx_set_goal",
		Description: "Set the active goal tag. All subsequent tool events will be tagged with this goal for better resume context.",
	}, withRateLimit(b, "ctx_set_goal", b.handleSetGoal))

	mcpsdk.AddTool(b.server, &mcpsdk.Tool{
		Name:        "ctx_clear_goal",
		Description: "Clear the active goal tag. Subsequent events will no longer be tagged with a goal.",
	}, withRateLimit(b, "ctx_clear_goal", b.handleClearGoal))

	mcpsdk.AddTool(b.server, &mcpsdk.Tool{
		Name:        "ctx_session_insights",
		Description: "Session analytics: event counts by type, top tools, error rate, session duration, active goals, and compact count.",
	}, withRateLimit(b, "ctx_session_insights", b.handleSessionInsights))

	mcpsdk.AddTool(b.server, &mcpsdk.Tool{
		Name:        "ctx_workspace_info",
		Description: "Show multi-project workspace info: root directory, detected projects, workspace type (pnpm, go-work, vscode, single).",
	}, withRateLimit(b, "ctx_workspace_info", b.handleWorkspaceInfo))
}

// ── Tool Handlers (direct service calls) ──

func (b *Bridge) handleExecute(ctx context.Context, req *mcpsdk.CallToolRequest, input executeInput) (*mcpsdk.CallToolResult, struct{}, error) {
	ctx = progressCtxFromRequest(ctx, req)

	ctx, finish := spanTool(ctx, "ctx_execute")
	defer finish(nil)

	b.recordToolCall(ctx, "ctx_execute", false)
	b.progress.Report(ctx, "ctx_execute", 0, 2, "starting execution")

	// Security check for shell language.
	if input.Language == "shell" {
		if err := b.checkCommandDeny(input.Code); err != nil {
			return errorResult("denied: " + err.Error()), struct{}{}, nil
		}
	}

	// Prepend network tracking preamble for JS/TS.
	code := input.Code
	if input.Language == "javascript" || input.Language == "typescript" {
		code = executor.JSNetPreamble + code
	}

	// For background JS/TS execution, append a keepalive interval to prevent
	// the process from exiting before background work completes.
	if input.Background && (input.Language == "javascript" || input.Language == "typescript") {
		code += "\nsetInterval(()=>{},2147483647);\n"
	}

	timeout := time.Duration(input.TimeoutMs) * time.Millisecond

	// For background execution, use the non-streaming Execute.
	if input.Background {
		result, err := b.executor.Execute(ctx, input.Language, code, timeout, true)
		if err != nil {
			return errorResult("execution failed: " + err.Error()), struct{}{}, nil
		}

		output := formatExecOutput(result.Stdout, result.Stderr, result.ExitCode, result.TimedOut, result.Backgrounded)

		return filterResult(output), struct{}{}, nil
	}

	// Use streaming for non-background execution.
	var stdoutBuf, stderrBuf strings.Builder

	result, err := b.executor.ExecuteStream(ctx, input.Language, code, timeout, func(chunk executor.OutputChunk) error {
		switch chunk.Stream {
		case "stdout":
			stdoutBuf.WriteString(chunk.Data)
			stdoutBuf.WriteString("\n")
		case "stderr":
			stderrBuf.WriteString(chunk.Data)
			stderrBuf.WriteString("\n")
		}

		return nil
	})
	if err != nil {
		return errorResult("execution failed: " + err.Error()), struct{}{}, nil
	}

	stdout := stdoutBuf.String()
	stderr := stderrBuf.String()
	exitCode := result.ExitCode
	timedOut := result.TimedOut

	// Track network bytes from instrumented JS/TS and clean markers from stderr.
	if netStats := executor.ParseNetMarkers(stderr); netStats != nil {
		b.stats.mu.Lock()
		b.stats.bytesSandboxed += netStats.TotalBytes
		b.stats.mu.Unlock()

		stderr = executor.CleanNetMarkers(stderr)
	}

	// Soft fail: shell exit 1 with stdout is not a real error (e.g., grep no matches).
	if isSoftFail(input.Language, exitCode, stdout) {
		exitCode = 0
	}

	output := formatExecOutput(stdout, stderr, exitCode, timedOut, false)

	if input.ExplainErrors && exitCode != 0 {
		output += "\n\n[classification] " + classifyError(stderr, stdout, exitCode)
	}

	b.progress.Report(ctx, "ctx_execute", 1, 2, "execution complete, indexing output")

	// Auto-index output.
	label := ""

	var sourceID int64

	if len(output) > 0 {
		label = fmt.Sprintf("exec:%s:%d", input.Language, time.Now().Unix())

		indexRes, indexErr := b.content.IndexPlainText(output, label, 20)
		if indexErr == nil {
			sourceID = indexRes.SourceID
		}
	}

	// Intent-based filtering: for large outputs with an intent, search the
	// just-indexed content and return a BM25 preview instead of raw output.
	if input.Intent != "" && len(output) > IntentSearchThreshold && label != "" {
		preview := b.intentPreview(input.Intent, label, output, sourceID)
		if preview != "" {
			b.progress.Report(ctx, "ctx_execute", 2, 2, "done")

			return filterResult(preview), struct{}{}, nil
		}
	}

	b.progress.Report(ctx, "ctx_execute", 2, 2, "done")

	return filterResult(output), struct{}{}, nil
}

func (b *Bridge) handleExecuteFile(ctx context.Context, _ *mcpsdk.CallToolRequest, input executeFileInput) (*mcpsdk.CallToolResult, struct{}, error) {
	ctx, finish := spanTool(ctx, "ctx_execute_file")
	defer finish(nil)

	// Security check for file path.
	if err := b.checkFilePathDeny(input.FilePath); err != nil {
		return errorResult("denied: " + err.Error()), struct{}{}, nil
	}

	timeout := time.Duration(input.TimeoutMs) * time.Millisecond

	// Use streaming for file execution.
	var stdoutBuf, stderrBuf strings.Builder

	result, err := b.executor.ExecuteFileStream(ctx, input.FilePath, input.Language, input.Code, timeout, func(chunk executor.OutputChunk) error {
		switch chunk.Stream {
		case "stdout":
			stdoutBuf.WriteString(chunk.Data)
			stdoutBuf.WriteString("\n")
		case "stderr":
			stderrBuf.WriteString(chunk.Data)
			stderrBuf.WriteString("\n")
		}

		return nil
	})
	if err != nil {
		return errorResult("execution failed: " + err.Error()), struct{}{}, nil
	}

	stdout := stdoutBuf.String()
	stderr := stderrBuf.String()
	exitCode := result.ExitCode
	timedOut := result.TimedOut

	output := formatExecOutput(stdout, stderr, exitCode, timedOut, false)

	if input.ExplainErrors && exitCode != 0 {
		output += "\n\n[classification] " + classifyError(stderr, stdout, exitCode)
	}

	label := ""

	var sourceID int64

	if len(output) > 0 {
		label = fmt.Sprintf("exec_file:%s:%d", input.Language, time.Now().Unix())

		indexRes, indexErr := b.content.IndexPlainText(output, label, 20)
		if indexErr == nil {
			sourceID = indexRes.SourceID
		}
	}

	// Intent-based filtering for file execution.
	if input.Intent != "" && len(output) > IntentSearchThreshold && label != "" {
		preview := b.intentPreview(input.Intent, label, output, sourceID)
		if preview != "" {
			return filterResult(preview), struct{}{}, nil
		}
	}

	return filterResult(output), struct{}{}, nil
}

func (b *Bridge) handleIndex(ctx context.Context, _ *mcpsdk.CallToolRequest, input indexInput) (*mcpsdk.CallToolResult, struct{}, error) {
	ctx, finish := spanTool(ctx, "ctx_index")
	defer finish(nil)

	b.recordToolCall(ctx, "ctx_index", false)

	ct := input.ContentType
	if ct == "" {
		ct = "markdown"
	}

	var (
		res model.IndexResult
		err error
	)

	switch ct {
	case "plain":
		res, err = b.content.IndexPlainText(input.Content, input.Label, 20)
	case "json":
		res, err = b.content.IndexJSON(input.Content, input.Label)
	default: // "markdown"
		res, err = b.content.Index(input.Content, input.Label)
	}

	if err != nil {
		return errorResult("index failed: " + err.Error()), struct{}{}, nil
	}

	return textResult(fmt.Sprintf("Indexed as %q", res.Label)), struct{}{}, nil
}

func (b *Bridge) handleSearch(ctx context.Context, _ *mcpsdk.CallToolRequest, input searchInput) (*mcpsdk.CallToolResult, struct{}, error) {
	ctx, finish := spanTool(ctx, "ctx_search")
	defer finish(nil)

	b.recordToolCall(ctx, "ctx_search", true)

	// Normalize: if Query is set and Queries is empty, use Query.
	if input.Query != "" && len(input.Queries) == 0 {
		input.Queries = []string{input.Query}
	}

	level := b.throttler.check()
	if level == ThrottleBlocked {
		return errorResult(fmt.Sprintf(
			"STOP — search budget exhausted. %d/%d searches used in last %s. "+
				"Do NOT retry; wait for the window to reset or use already-indexed content.",
			MaxSearchesPerWindow, MaxSearchesPerWindow, ThrottleWindow)), struct{}{}, nil
	}

	limit := effectiveLimit(level, input.Limit)

	var sb strings.Builder

	totalBytes := 0

	// Execute queries one at a time.
	for i, q := range input.Queries {
		if totalBytes >= SearchTotalOutputCap {
			fmt.Fprintf(&sb, "## Query %d: %s — SKIPPED (output cap reached)\n", i+1, q)

			continue
		}

		results, err := b.content.SearchWithFallback(q, limit, input.Source)
		if err != nil {
			fmt.Fprintf(&sb, "## Query %d: %s\nError: %s\n\n", i+1, q, err)

			continue
		}

		fmt.Fprintf(&sb, "## Query %d: %s (%d results)\n", i+1, q, len(results))

		for _, r := range results {
			content := r.Content
			if input.Intent != "" {
				content = extractSnippetHL(content, input.Intent, 500, r.Highlighted)
			}

			fmt.Fprintf(&sb, "### %s [%s] (rank: %.2f, layer: %s)\n%s\n\n",
				r.Title, r.Source, r.Rank, r.MatchLayer, content)
		}

		totalBytes = sb.Len()
	}

	if level == ThrottleDegraded {
		fmt.Fprintf(&sb, "\n⚠ Search is in degraded mode (%d/%d used). Results limited to 1 per query.\n",
			b.throttler.used(), MaxSearchesPerWindow)
	}

	output := sb.String()
	if output == "" {
		// When no results found, show available sources as guidance.
		sources, srcErr := b.content.ListSources()
		if srcErr == nil && len(sources) > 0 {
			var srcSb strings.Builder
			srcSb.WriteString("No results found.\n\n## Available Sources\n")

			for _, src := range sources {
				fmt.Fprintf(&srcSb, "- **%s** (%d chunks)\n", src.Label, src.ChunkCount)
			}

			srcSb.WriteString("\nTry searching with different terms or specify a source filter.")
			output = srcSb.String()
		} else {
			output = "No results found."
		}
	}

	return filterResult(output), struct{}{}, nil
}

func (b *Bridge) handleFetchAndIndex(ctx context.Context, req *mcpsdk.CallToolRequest, input fetchAndIndexInput) (*mcpsdk.CallToolResult, struct{}, error) {
	ctx = progressCtxFromRequest(ctx, req)

	ctx, finish := spanTool(ctx, "ctx_fetch_and_index")
	defer finish(nil)

	b.recordToolCall(ctx, "ctx_fetch_and_index", false)
	b.progress.Report(ctx, "ctx_fetch_and_index", 0, 2, "fetching "+input.URL)

	result, err := fetch.Fetch(ctx, input.URL, nil)
	if err != nil {
		return errorResult("fetch failed: " + err.Error()), struct{}{}, nil
	}

	b.progress.Report(ctx, "ctx_fetch_and_index", 1, 2, "indexing content")

	label := input.Label
	if label == "" {
		label = input.URL
	}

	// Route content type based on the fetched Content-Type header.
	var (
		indexRes model.IndexResult
		idxErr   error
	)

	switch {
	case strings.Contains(result.ContentType, "application/json"):
		indexRes, idxErr = b.content.IndexJSON(result.Content, label)
	case strings.Contains(result.ContentType, "text/plain"):
		indexRes, idxErr = b.content.IndexPlainText(result.Content, label, 20)
	default:
		indexRes, idxErr = b.content.Index(result.Content, label)
	}

	if idxErr != nil {
		return errorResult("index failed: " + idxErr.Error()), struct{}{}, nil
	}

	// Build preview — first ~3KB for immediate use.
	const previewLimit = 3072

	preview := result.Content
	if len(preview) > previewLimit {
		preview = preview[:previewLimit] + "\n\n...[truncated — use search() for full content]"
	}

	totalKB := float64(result.BytesFetched) / 1024

	var sb strings.Builder
	fmt.Fprintf(&sb, "Fetched and indexed **%d sections** (%.1fKB) from: %s\n",
		indexRes.TotalChunks, totalKB, indexRes.Label)
	fmt.Fprintf(&sb, "Full content indexed — use search(queries: [...], source: %q) for specific lookups.\n\n",
		indexRes.Label)
	sb.WriteString("---\n\n")
	sb.WriteString(preview)

	b.progress.Report(ctx, "ctx_fetch_and_index", 2, 2, "done")

	return filterResult(sb.String()), struct{}{}, nil
}

func (b *Bridge) handleBatchExecute(ctx context.Context, req *mcpsdk.CallToolRequest, input batchExecuteInput) (*mcpsdk.CallToolResult, struct{}, error) { //nolint:maintidx // orchestrator function, complexity is inherent
	ctx = progressCtxFromRequest(ctx, req)

	ctx, finish := spanTool(ctx, "ctx_batch_execute")
	defer finish(nil)

	b.recordToolCall(ctx, "ctx_batch_execute", false)

	// Total steps = commands + 1 (for search phase).
	totalSteps := float64(len(input.Commands) + 1)
	b.progress.Report(ctx, "ctx_batch_execute", 0, totalSteps, "starting batch")

	batchStart := time.Now()

	var sb strings.Builder

	// Execute commands and auto-index.
	for i, cmd := range input.Commands {
		elapsed := time.Since(batchStart).Milliseconds()
		if elapsed > BatchTimeoutMs {
			fmt.Fprintf(&sb, "## Command %d: %s — SKIPPED (batch timeout)\n", i+1, cmd.Command)

			continue
		}

		// Security check for each command.
		if err := b.checkCommandDeny(cmd.Command); err != nil {
			fmt.Fprintf(&sb, "## Command %d: %s\nDenied: %s\n\n", i+1, cmd.Command, err)

			continue
		}

		remainingMs := BatchTimeoutMs - elapsed
		timeout := time.Duration(remainingMs) * time.Millisecond

		result, err := b.executor.Execute(ctx, "shell", cmd.Command, timeout, false)
		if err != nil {
			fmt.Fprintf(&sb, "## Command %d: %s\nError: %s\n\n", i+1, cmd.Command, err)

			continue
		}

		output := formatExecOutput(result.Stdout, result.Stderr, result.ExitCode, result.TimedOut, false)

		// Track network bytes from batch commands.
		if netStats := executor.ParseNetMarkers(result.Stderr); netStats != nil {
			b.stats.mu.Lock()
			b.stats.bytesSandboxed += netStats.TotalBytes
			b.stats.mu.Unlock()
		}

		label := cmd.Label
		if label == "" {
			label = fmt.Sprintf("cmd:%d:%d", i+1, time.Now().Unix())
		}

		if output != "" {
			_, _ = b.content.IndexPlainText(output, label, 20)
		}

		if input.ExplainErrors && result.ExitCode != 0 {
			output += "\n\n[classification] " + classifyError(result.Stderr, result.Stdout, result.ExitCode)
		}

		b.progress.Report(ctx, "ctx_batch_execute", float64(i+1), totalSteps,
			fmt.Sprintf("command %d/%d complete", i+1, len(input.Commands)))

		fmt.Fprintf(&sb, "## Command %d: %s (exit %d)\n", i+1, cmd.Command, result.ExitCode)

		if result.ExitCode != 0 || result.TimedOut {
			sb.WriteString(output + "\n")
		} else {
			fmt.Fprintf(&sb, "Indexed as %q (%d bytes)\n\n", label, len(output))
		}

		// Per-command inline search preview: run each query against this
		// command's output immediately so results are available even if
		// later commands time out.
		if len(input.Queries) > 0 && output != "" {
			for _, q := range input.Queries {
				qResults, qErr := b.content.SearchWithFallback(q, 1, label)
				if qErr == nil && len(qResults) > 0 {
					snippet := qResults[0].Title
					if snippet == "" {
						snippet = qResults[0].Content
						if len(snippet) > 120 {
							snippet = snippet[:120]
						}
					}

					fmt.Fprintf(&sb, "  → %s: %s\n", q, snippet)
				}
			}

			sb.WriteString("\n")
		}
	}

	// Build section inventory from per-command indexes.
	// Each command was already indexed individually above — collect all labels
	// so search queries can scope to this batch's content.
	batchLabels := make([]string, 0, len(input.Commands))

	for i, cmd := range input.Commands {
		label := cmd.Label
		if label == "" {
			label = fmt.Sprintf("cmd:%d:%d", i+1, time.Now().Unix())
		}

		batchLabels = append(batchLabels, label)
	}

	// Also index combined output as a single markdown source for section inventory.
	var combined strings.Builder

	for i, cmd := range input.Commands {
		label := cmd.Label
		if label == "" {
			label = fmt.Sprintf("cmd:%d", i+1)
		}

		fmt.Fprintf(&combined, "# %s\n\n", label)
	}

	batchLabel := "batch:" + strings.Join(batchLabels, ",")
	if len(batchLabel) > 80 {
		batchLabel = batchLabel[:77] + "..."
	}

	if combined.Len() > 0 {
		idxRes, idxErr := b.content.Index(combined.String(), batchLabel)

		if idxErr == nil && idxRes.SourceID > 0 {
			chunks, chunksErr := b.content.GetChunksBySource(idxRes.SourceID)
			if chunksErr == nil && len(chunks) > 0 {
				sb.WriteString("## Indexed Sections\n\n")

				for _, chunk := range chunks {
					fmt.Fprintf(&sb, "- **%s** (%d bytes)\n", chunk.Title, len(chunk.Content))
				}

				sb.WriteString("\n")
			}
		}
	}

	// Run search queries with snippet cap for batch mode (3KB).
	// Three-tier fallback: scoped → global fallback → cross-source warning.
	const batchSnippetCap = 3 * 1024

	const batchOutputCap = 80 * 1024 // 80KB total query output cap

	queryOutputSize := 0

	for i, q := range input.Queries {
		if queryOutputSize > batchOutputCap {
			fmt.Fprintf(&sb, "## Query %d: %s\n(output cap reached — use search(queries: [\"%s\"]) for details)\n\n", i+1, q, q)
			continue
		}

		// Tier 1: scoped search (commands from this batch).
		results, err := b.content.SearchWithFallback(q, 5, "")
		if err != nil {
			fmt.Fprintf(&sb, "## Query %d: %s\nError: %s\n\n", i+1, q, err)

			continue
		}

		crossSource := false

		// Tier 2: global fallback when scoped returns nothing.
		if len(results) == 0 {
			globalResults, gerr := b.content.SearchWithFallback(q, 5, "")
			if gerr == nil && len(globalResults) > 0 {
				results = globalResults
				crossSource = true
			}
		}

		fmt.Fprintf(&sb, "## Query %d: %s (%d results)\n", i+1, q, len(results))

		if crossSource {
			sb.WriteString("> **Note:** No results in current batch output. Showing results from previously indexed content.\n\n")
		}

		for _, r := range results {
			content := extractSnippetHL(r.Content, q, batchSnippetCap, r.Highlighted)

			// Extract distinctive terms to help user scan relevance.
			terms := extractDistinctiveTerms(r.Content, q, 5)

			sourceTag := ""
			if crossSource {
				sourceTag = fmt.Sprintf(" _(source: %s)_", r.Source)
			}

			if len(terms) > 0 {
				fmt.Fprintf(&sb, "### %s%s (terms: %s)\n%s\n\n",
					r.Title, sourceTag, strings.Join(terms, ", "), content)
			} else {
				fmt.Fprintf(&sb, "### %s%s\n%s\n\n", r.Title, sourceTag, content)
			}

			queryOutputSize += len(content) + len(r.Title)
		}
	}

	output := sb.String()
	if output == "" {
		output = "No commands or queries provided."
	}

	b.progress.Report(ctx, "ctx_batch_execute", totalSteps, totalSteps, "done")

	return filterResult(output), struct{}{}, nil
}

func (b *Bridge) handleStats(ctx context.Context, _ *mcpsdk.CallToolRequest, _ statsInput) (*mcpsdk.CallToolResult, struct{}, error) {
	_, finish := spanTool(ctx, "ctx_stats")
	defer finish(nil)

	storeStats, err := b.content.GetStats()
	if err != nil {
		return errorResult("stats failed: " + err.Error()), struct{}{}, nil
	}

	snap := b.stats.snapshot()

	var sb strings.Builder

	// Knowledge Base section.
	fmt.Fprintf(&sb, "## Knowledge Base\n\n")
	fmt.Fprintf(&sb, "- **Sources:** %d\n", storeStats.Sources)
	fmt.Fprintf(&sb, "- **Chunks:** %d\n", storeStats.Chunks)
	fmt.Fprintf(&sb, "- **Code chunks:** %d\n\n", storeStats.CodeChunks)

	// Context Window Protection section.
	bytesIndexed, _ := snap["bytesIndexed"].(int)
	bytesReturned := 0

	if brMap, ok := snap["bytesReturned"].(map[string]int); ok {
		for _, v := range brMap {
			bytesReturned += v
		}
	}

	bytesSandboxed, _ := snap["bytesSandboxed"].(int)

	keptOut := bytesIndexed + bytesSandboxed
	totalProcessed := keptOut + bytesReturned

	fmt.Fprintf(&sb, "## Context Window Protection\n\n")
	fmt.Fprintf(&sb, "- **Total data processed:** %s\n", formatBytes(totalProcessed))
	fmt.Fprintf(&sb, "- **Kept in sandbox:** %s\n", formatBytes(keptOut))
	fmt.Fprintf(&sb, "- **Entered context:** %s\n", formatBytes(bytesReturned))
	fmt.Fprintf(&sb, "- **Estimated tokens saved:** ~%d\n", keptOut/4)

	if totalProcessed > 0 {
		savingsRatio := float64(totalProcessed) / float64(max(bytesReturned, 1))
		reductionPct := (1 - float64(bytesReturned)/float64(totalProcessed)) * 100
		fmt.Fprintf(&sb, "- **Context savings:** %.1fx (%.0f%% reduction)\n", savingsRatio, reductionPct)
	}

	sb.WriteString("\n")

	// Per-Tool Breakdown section.
	if calls, ok := snap["calls"].(map[string]int); ok && len(calls) > 0 {
		brMap, _ := snap["bytesReturned"].(map[string]int)

		fmt.Fprintf(&sb, "## Per-Tool Breakdown\n\n")
		fmt.Fprintf(&sb, "| Tool | Calls | Context | Tokens |\n")
		fmt.Fprintf(&sb, "|------|------:|--------:|-------:|\n")

		totalCalls := 0

		for tool, count := range calls {
			br := 0
			if brMap != nil {
				br = brMap[tool]
			}

			tokens := br / 4
			fmt.Fprintf(&sb, "| %s | %d | %s | ~%d |\n", tool, count, formatBytes(br), tokens)
			totalCalls += count
		}

		fmt.Fprintf(&sb, "| **Total** | **%d** | **%s** | **~%d** |\n\n",
			totalCalls, formatBytes(bytesReturned), bytesReturned/4)
	}

	// Session section.
	uptimeSeconds, _ := snap["uptimeSeconds"].(int)

	fmt.Fprintf(&sb, "## Session\n\n")
	fmt.Fprintf(&sb, "- **Uptime:** %ds\n", uptimeSeconds)
	fmt.Fprintf(&sb, "- **Bytes sandboxed:** %d\n\n", bytesSandboxed)

	// Session Continuity subsection — best-effort.
	if b.session != nil && b.sessionID != "" {
		events, evtErr := b.session.GetEvents(b.sessionID, nil)
		if evtErr == nil && len(events) > 0 {
			categoryCounts := make(map[string]int)

			for _, ev := range events {
				cat := ev.Category
				if cat == "" {
					cat = "uncategorized"
				}

				categoryCounts[cat]++
			}

			fmt.Fprintf(&sb, "### Session Continuity\n\n")
			fmt.Fprintf(&sb, "| Category | Count |\n")
			fmt.Fprintf(&sb, "|----------|------:|\n")

			for cat, count := range categoryCounts {
				fmt.Fprintf(&sb, "| %s | %d |\n", cat, count)
			}

			sb.WriteString("\n")
		}
	}

	// Raw JSON at bottom.
	combined := map[string]any{
		"store": map[string]any{
			"sources":    storeStats.Sources,
			"chunks":     storeStats.Chunks,
			"codeChunks": storeStats.CodeChunks,
		},
		"session": snap,
	}

	data, err2 := json.MarshalIndent(combined, "", "  ")
	if err2 == nil {
		fmt.Fprintf(&sb, "```json\n%s\n```\n", string(data))
	}

	return textResult(sb.String()), struct{}{}, nil
}

func (b *Bridge) handleDoctor(ctx context.Context, _ *mcpsdk.CallToolRequest, _ doctorInput) (*mcpsdk.CallToolResult, struct{}, error) {
	_, finish := spanTool(ctx, "ctx_doctor")
	defer finish(nil)

	health := map[string]any{
		"status": "healthy",
		"server": map[string]any{
			"name":    serverName,
			"version": serverVersion,
		},
		"runtime": map[string]any{
			"go":         runtime.Version(),
			"os":         runtime.GOOS,
			"arch":       runtime.GOARCH,
			"goroutines": runtime.NumGoroutine(),
		},
		"throttle": map[string]any{
			"remaining":      b.throttler.remaining(),
			"max_per_window": MaxSearchesPerWindow,
			"window_seconds": int(ThrottleWindow.Seconds()),
		},
	}

	stats, err := b.content.GetStats()
	if err == nil {
		health["knowledge_base"] = map[string]any{
			"sources":     stats.Sources,
			"chunks":      stats.Chunks,
			"code_chunks": stats.CodeChunks,
		}
	}

	health["session"] = b.stats.snapshot()

	data, err2 := json.MarshalIndent(health, "", "  ")
	if err2 != nil {
		return errorResult("marshal failed: " + err2.Error()), struct{}{}, nil
	}

	return textResult(string(data)), struct{}{}, nil
}

// ── Code Analysis Handlers ──

func (b *Bridge) handleAnalyze(ctx context.Context, _ *mcpsdk.CallToolRequest, input analyzeInput) (*mcpsdk.CallToolResult, struct{}, error) {
	_, finish := spanTool(ctx, "ctx_analyze")
	defer finish(nil)

	rootDir := input.RootDir
	if rootDir == "" {
		rootDir = b.projectDir
	}

	analyzer := analysis.NewAnalyzer(rootDir)

	var (
		result *analysis.AnalysisResult
		err    error
	)

	switch {
	case input.Incremental:
		result, err = analyzer.AnalyzeIncremental()
	case len(input.Paths) > 0:
		result, err = analyzer.AnalyzePaths(input.Paths)
	default:
		result, err = analyzer.Analyze()
	}

	if err != nil {
		return errorResult("analyze failed: " + err.Error()), struct{}{}, nil
	}

	// Build a summary to index into knowledge base.
	var sb strings.Builder
	fmt.Fprintf(&sb, "# Code Analysis Summary\n\n")
	fmt.Fprintf(&sb, "- **Files analyzed:** %d\n", result.Summary.TotalFiles)
	fmt.Fprintf(&sb, "- **Symbols extracted:** %d\n", result.Summary.TotalSymbols)
	fmt.Fprintf(&sb, "- **References found:** %d\n\n", result.Summary.TotalReferences)

	if len(result.Summary.ByKind) > 0 {
		sb.WriteString("## By Kind\n")

		for kind, count := range result.Summary.ByKind {
			fmt.Fprintf(&sb, "- %s: %d\n", kind, count)
		}

		sb.WriteString("\n")
	}

	if len(result.Summary.ByLanguage) > 0 {
		sb.WriteString("## By Language\n")

		for lang, count := range result.Summary.ByLanguage {
			fmt.Fprintf(&sb, "- %s: %d\n", lang, count)
		}
	}

	output := sb.String()

	// Also extract exported symbols and index them for searchability.
	var exportedSymbols []analysis.Symbol

	for _, f := range result.Files {
		for _, sym := range f.Symbols {
			if sym.Exported {
				exportedSymbols = append(exportedSymbols, sym)
			}
		}
	}

	if len(exportedSymbols) > 0 {
		var symSb strings.Builder
		symSb.WriteString("# Exported Symbols\n\n")

		limit := min(500, len(exportedSymbols))

		for _, sym := range exportedSymbols[:limit] {
			fmt.Fprintf(&symSb, "## %s.%s (%s)\n", sym.Package, sym.Name, sym.Kind)
			fmt.Fprintf(&symSb, "- File: %s:%d\n", sym.File, sym.Line)
			fmt.Fprintf(&symSb, "- Signature: `%s`\n", sym.Signature)

			if sym.Doc != "" {
				fmt.Fprintf(&symSb, "- Doc: %s\n", sym.Doc)
			}

			symSb.WriteString("\n")
		}

		symContent := symSb.String()
		_, _ = b.content.Index(symContent, "code-analysis:symbols")
	}

	// Index the summary.
	_, _ = b.content.Index(output, "code-analysis:summary")

	// Index file summaries.
	fileSummaries := analysis.GenerateFileSummaries(result)
	if fileSummaries != "" {
		_, _ = b.content.Index(fileSummaries, "code-analysis:files")
	}

	// Index dependency graph.
	depGraph := analysis.GenerateDepGraphMarkdown(result)
	if depGraph != "" {
		_, _ = b.content.Index(depGraph, "code-analysis:dependencies")
	}

	return textResult(output), struct{}{}, nil
}

func (b *Bridge) handleSymbols(ctx context.Context, _ *mcpsdk.CallToolRequest, input symbolsInput) (*mcpsdk.CallToolResult, struct{}, error) {
	_, finish := spanTool(ctx, "ctx_symbols")
	defer finish(nil)

	limit := input.Limit
	if limit <= 0 {
		limit = 50
	}

	// Re-analyze the project to get current symbols.
	analyzer := analysis.NewAnalyzer(b.projectDir)

	result, err := analyzer.Analyze()
	if err != nil {
		return errorResult("symbols query failed: " + err.Error()), struct{}{}, nil
	}

	// Filter symbols based on input criteria.
	var allSymbols []analysis.Symbol

	switch {
	case input.Package != "":
		allSymbols = analysis.SymbolsInPackage(result, input.Package)
	case input.Query != "":
		allSymbols = analysis.FindSymbols(result, input.Query, analysis.SymbolKind(input.Kind))
	default:
		// Collect all symbols from all files.
		for _, f := range result.Files {
			allSymbols = append(allSymbols, f.Symbols...)
		}

		// Filter by kind if specified.
		if input.Kind != "" {
			var filtered []analysis.Symbol

			for _, sym := range allSymbols {
				if string(sym.Kind) == input.Kind {
					filtered = append(filtered, sym)
				}
			}

			allSymbols = filtered
		}
	}

	// Apply exported-only filter.
	if input.ExportedOnly {
		var filtered []analysis.Symbol

		for _, sym := range allSymbols {
			if sym.Exported {
				filtered = append(filtered, sym)
			}
		}

		allSymbols = filtered
	}

	if len(allSymbols) == 0 {
		return textResult("No symbols found."), struct{}{}, nil
	}

	totalCount := len(allSymbols)
	if len(allSymbols) > limit {
		allSymbols = allSymbols[:limit]
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Found %d symbol(s):\n\n", totalCount)

	for _, sym := range allSymbols {
		fmt.Fprintf(&sb, "### %s.%s\n", sym.Package, sym.Name)
		fmt.Fprintf(&sb, "- **Kind:** %s\n", sym.Kind)
		fmt.Fprintf(&sb, "- **File:** %s:%d\n", sym.File, sym.Line)
		fmt.Fprintf(&sb, "- **Signature:** `%s`\n", sym.Signature)

		if sym.Receiver != "" {
			fmt.Fprintf(&sb, "- **Receiver:** %s\n", sym.Receiver)
		}

		if sym.Doc != "" {
			fmt.Fprintf(&sb, "- **Doc:** %s\n", sym.Doc)
		}

		fmt.Fprintf(&sb, "- **Exported:** %v\n\n", sym.Exported)
	}

	return textResult(sb.String()), struct{}{}, nil
}

// ── Helpers ──

func formatExecOutput(stdout, stderr string, exitCode int, timedOut, backgrounded bool) string {
	var sb strings.Builder
	if backgrounded {
		sb.WriteString("[backgrounded]\n")
		return sb.String()
	}

	if stdout != "" {
		sb.WriteString(stdout)
	}

	if stderr != "" {
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}

		sb.WriteString("[stderr]\n")
		sb.WriteString(stderr)
	}

	if timedOut {
		sb.WriteString("\n[timed out]")
	}

	if exitCode != 0 {
		fmt.Fprintf(&sb, "\n[exit code: %d]", exitCode)
	}

	return sb.String()
}

// classifyError delegates to executor.ClassifyError for error classification.
func classifyError(stderr, stdout string, exitCode int) string {
	return executor.ClassifyError(stderr, stdout, exitCode)
}

// isSoftFail returns true when a shell command exits with code 1 but produced
// stdout output — e.g., grep returning "no matches" is not a real error.
func isSoftFail(language string, exitCode int, stdout string) bool {
	return language == "shell" && exitCode == 1 && strings.TrimSpace(stdout) != ""
}

// formatBytes formats a byte count as KB or MB for display.
func formatBytes(b int) string {
	if b >= 1024*1024 {
		return fmt.Sprintf("%.1fMB", float64(b)/1024/1024)
	}

	return fmt.Sprintf("%.1fKB", float64(b)/1024)
}

func textResult(text string) *mcpsdk.CallToolResult {
	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: text}},
	}
}

func errorResult(text string) *mcpsdk.CallToolResult {
	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: text}},
		IsError: true,
	}
}

// intentPreview searches the knowledge base for just-indexed content using the
// intent as the query. Returns title-only preview + vocabulary hints, or "" if no results.
func (b *Bridge) intentPreview(intent, label, rawOutput string, sourceID int64) string {
	results, err := b.content.SearchWithFallback(intent, 5, label)
	if err != nil {
		return ""
	}

	totalLines := strings.Count(rawOutput, "\n") + 1
	totalBytes := len(rawOutput)

	var sb strings.Builder

	if len(results) == 0 {
		fmt.Fprintf(&sb, "Indexed into knowledge base as %q.\n", label)
		fmt.Fprintf(&sb, "No sections matched intent %q in %d-line output (%.1fKB).\n",
			intent, totalLines, float64(totalBytes)/1024)
	} else {
		fmt.Fprintf(&sb, "Indexed into knowledge base as %q.\n", label)
		fmt.Fprintf(&sb, "%d sections matched %q (%d lines, %.1fKB):\n\n",
			len(results), intent, totalLines, float64(totalBytes)/1024)

		// Return ONLY titles + first-line previews — not full content.
		for _, r := range results {
			firstLine := r.Content
			if idx := strings.IndexByte(firstLine, '\n'); idx >= 0 {
				firstLine = firstLine[:idx]
			}

			if len(firstLine) > 120 {
				firstLine = firstLine[:120]
			}

			fmt.Fprintf(&sb, "  - %s: %s\n", r.Title, firstLine)
		}
	}

	// Append vocabulary hints from distinctive terms.
	if sourceID > 0 {
		terms, terr := b.content.GetDistinctiveTerms(sourceID, 40)
		if terr == nil && len(terms) > 0 {
			fmt.Fprintf(&sb, "\nSearchable terms: %s\n", strings.Join(terms, ", "))
		}
	}

	fmt.Fprintf(&sb, "\nUse search(queries: [...]) to retrieve full content of any section.")

	return sb.String()
}

// ── Upgrade Handler ──

func (b *Bridge) handleUpgrade(ctx context.Context, _ *mcpsdk.CallToolRequest, _ upgradeInput) (*mcpsdk.CallToolResult, struct{}, error) {
	_, finish := spanTool(ctx, "ctx_upgrade")
	defer finish(nil)

	installCmd := "go install github.com/inovacc/thimble@latest"
	if runtime.GOOS == "windows" {
		installCmd += " # or: scoop update thimble"
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "## Thimble Upgrade\n\n")
	fmt.Fprintf(&sb, "**Current version:** %s\n\n", serverVersion)
	fmt.Fprintf(&sb, "**Install/upgrade command:**\n```\n%s\n```\n", installCmd)

	return textResult(sb.String()), struct{}{}, nil
}

// ── Delegate Handlers ──

func (b *Bridge) handleDelegate(ctx context.Context, _ *mcpsdk.CallToolRequest, input delegateInput) (*mcpsdk.CallToolResult, struct{}, error) {
	_, finish := spanTool(ctx, "ctx_delegate")
	defer finish(nil)

	resp, err := b.delegate.StartTask(input.Language, input.Code, input.Description, int64(input.TimeoutMs))
	if err != nil {
		return errorResult("delegate failed: " + err.Error()), struct{}{}, nil
	}

	return textResult(fmt.Sprintf("Task started: %s (status: %s)\nUse ctx_delegate_status with task_id=%q to check progress.",
		resp.TaskID, resp.Status, resp.TaskID)), struct{}{}, nil
}

func (b *Bridge) handleDelegateStatus(ctx context.Context, _ *mcpsdk.CallToolRequest, input delegateStatusInput) (*mcpsdk.CallToolResult, struct{}, error) {
	_, finish := spanTool(ctx, "ctx_delegate_status")
	defer finish(nil)

	resp, err := b.delegate.GetTaskStatus(input.TaskID)
	if err != nil {
		return errorResult("status failed: " + err.Error()), struct{}{}, nil
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "## Task %s\n", resp.TaskID)
	fmt.Fprintf(&sb, "- **Status:** %s\n", resp.Status)
	fmt.Fprintf(&sb, "- **Progress:** %d%%\n", resp.ProgressPct)
	fmt.Fprintf(&sb, "- **Description:** %s\n", resp.Description)

	if resp.ErrorMsg != "" {
		fmt.Fprintf(&sb, "- **Error:** %s\n", resp.ErrorMsg)
	}

	if resp.TimedOut {
		sb.WriteString("- **Timed out:** yes\n")
	}

	if resp.ExitCode != 0 {
		fmt.Fprintf(&sb, "- **Exit code:** %d\n", resp.ExitCode)
	}

	if resp.Stdout != "" {
		fmt.Fprintf(&sb, "\n### stdout\n```\n%s```\n", resp.Stdout)
	}

	if resp.Stderr != "" {
		fmt.Fprintf(&sb, "\n### stderr\n```\n%s```\n", resp.Stderr)
	}

	output := sb.String()

	// Auto-index completed task output.
	if resp.Status == "completed" || resp.Status == "failed" {
		if resp.Stdout != "" || resp.Stderr != "" {
			label := fmt.Sprintf("delegate:%s:%d", resp.TaskID[:8], time.Now().Unix())
			_, _ = b.content.IndexPlainText(output, label, 20)
		}
	}

	return filterResult(output), struct{}{}, nil
}

func (b *Bridge) handleDelegateCancel(ctx context.Context, _ *mcpsdk.CallToolRequest, input delegateCancelInput) (*mcpsdk.CallToolResult, struct{}, error) {
	_, finish := spanTool(ctx, "ctx_delegate_cancel")
	defer finish(nil)

	cancelled, reason := b.delegate.CancelTask(input.TaskID)

	if cancelled {
		return textResult(fmt.Sprintf("Task %s cancelled.", input.TaskID)), struct{}{}, nil
	}

	return textResult(fmt.Sprintf("Task %s not cancelled: %s", input.TaskID, reason)), struct{}{}, nil
}

func (b *Bridge) handleDelegateList(ctx context.Context, _ *mcpsdk.CallToolRequest, input delegateListInput) (*mcpsdk.CallToolResult, struct{}, error) {
	_, finish := spanTool(ctx, "ctx_delegate_list")
	defer finish(nil)

	tasks := b.delegate.ListTasks(input.StatusFilter)

	if len(tasks) == 0 {
		return textResult("No tasks found."), struct{}{}, nil
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "| Task ID | Description | Status | Progress |\n")
	fmt.Fprintf(&sb, "|---------|-------------|--------|----------|\n")

	for _, t := range tasks {
		desc := t.Description
		if len(desc) > 40 {
			desc = desc[:37] + "..."
		}

		fmt.Fprintf(&sb, "| %.16s | %s | %s | %d%% |\n",
			t.ID, desc, t.Status, t.ProgressPct)
	}

	return textResult(sb.String()), struct{}{}, nil
}

func (b *Bridge) handleSetGoal(ctx context.Context, _ *mcpsdk.CallToolRequest, input setGoalInput) (*mcpsdk.CallToolResult, struct{}, error) {
	_, finish := spanTool(ctx, "ctx_set_goal")
	defer finish(nil)

	goal := strings.TrimSpace(input.Goal)
	if goal == "" {
		return errorResult("goal must not be empty"), struct{}{}, nil
	}

	b.SetActiveGoal(goal)

	// Record a goal_set event so it appears in session history.
	_ = b.session.EnsureSession("default", b.projectDir)
	_ = b.session.InsertEvent("default", model.SessionEvent{
		Type:     "goal_set",
		Category: "goal",
		Data:     fmt.Sprintf(`{"goal":"%s"}`, goal),
		Priority: 2,
	}, "mcp")

	return textResult(fmt.Sprintf("Goal set: %s", goal)), struct{}{}, nil
}

func (b *Bridge) handleClearGoal(ctx context.Context, _ *mcpsdk.CallToolRequest, _ clearGoalInput) (*mcpsdk.CallToolResult, struct{}, error) {
	_, finish := spanTool(ctx, "ctx_clear_goal")
	defer finish(nil)

	prev := b.ActiveGoal()
	b.SetActiveGoal("")

	if prev == "" {
		return textResult("No active goal to clear."), struct{}{}, nil
	}

	// Record a goal_cleared event.
	_ = b.session.EnsureSession("default", b.projectDir)
	_ = b.session.InsertEvent("default", model.SessionEvent{
		Type:     "goal_cleared",
		Category: "goal",
		Data:     fmt.Sprintf(`{"goal":"%s","cleared":true}`, prev),
		Priority: 2,
	}, "mcp")

	return textResult(fmt.Sprintf("Goal cleared (was: %s)", prev)), struct{}{}, nil
}

func (b *Bridge) handleSessionInsights(ctx context.Context, _ *mcpsdk.CallToolRequest, input sessionInsightsInput) (*mcpsdk.CallToolResult, struct{}, error) {
	_, finish := spanTool(ctx, "ctx_session_insights")
	defer finish(nil)

	b.recordToolCall(ctx, "ctx_session_insights", true)

	sid := input.SessionID
	if sid == "" {
		sid = "default"
	}

	if b.session == nil {
		return errorResult("session database not available"), struct{}{}, nil
	}

	var sb strings.Builder

	sb.WriteString("# Session Insights\n\n")

	// Total events count.
	totalEvents, err := b.session.GetEventCount(sid)
	if err != nil {
		return errorResult(fmt.Sprintf("get event count: %v", err)), struct{}{}, nil
	}

	fmt.Fprintf(&sb, "**Total events:** %d\n\n", totalEvents)

	// Session metadata (compact count).
	meta, err := b.session.GetSessionStats(sid)
	if err != nil {
		return errorResult(fmt.Sprintf("get session stats: %v", err)), struct{}{}, nil
	}

	if meta != nil {
		fmt.Fprintf(&sb, "**Compact count:** %d\n\n", meta.CompactCount)
	}

	// Session duration.
	duration, err := b.session.SessionDuration(sid)
	if err != nil {
		return errorResult(fmt.Sprintf("get session duration: %v", err)), struct{}{}, nil
	}

	if duration > 0 {
		fmt.Fprintf(&sb, "**Session duration:** %s\n\n", duration.Truncate(time.Second))
	} else {
		sb.WriteString("**Session duration:** n/a\n\n")
	}

	// Events by type.
	byType, err := b.session.EventsByType(sid)
	if err != nil {
		return errorResult(fmt.Sprintf("get events by type: %v", err)), struct{}{}, nil
	}

	if len(byType) > 0 {
		sb.WriteString("## Events by Type\n\n")
		sb.WriteString("| Type | Count |\n")
		sb.WriteString("|------|-------|\n")

		// Compute error count while iterating.
		var errorCount int

		for typ, cnt := range byType {
			fmt.Fprintf(&sb, "| %s | %d |\n", typ, cnt)

			if typ == "error" || typ == "tool_error" {
				errorCount += cnt
			}
		}

		sb.WriteString("\n")

		// Error rate.
		if totalEvents > 0 {
			rate := float64(errorCount) / float64(totalEvents) * 100
			fmt.Fprintf(&sb, "**Error count:** %d (%.1f%%)\n\n", errorCount, rate)
		}
	}

	// Top tools.
	topTools, err := b.session.TopTools(sid, 10)
	if err != nil {
		return errorResult(fmt.Sprintf("get top tools: %v", err)), struct{}{}, nil
	}

	if len(topTools) > 0 {
		sb.WriteString("## Top Tools\n\n")
		sb.WriteString("| Tool | Calls |\n")
		sb.WriteString("|------|-------|\n")

		for _, tc := range topTools {
			fmt.Fprintf(&sb, "| %s | %d |\n", tc.Name, tc.Count)
		}

		sb.WriteString("\n")
	}

	// Active goals.
	goals, err := b.session.EventsByType(sid)
	if err == nil {
		goalCount := goals["goal_set"]
		if goalCount > 0 {
			fmt.Fprintf(&sb, "**Goal events:** %d set\n\n", goalCount)
		}
	}

	if goal := b.ActiveGoal(); goal != "" {
		fmt.Fprintf(&sb, "**Active goal:** %s\n", goal)
	}

	return textResult(sb.String()), struct{}{}, nil
}

// ── Workspace Info Handler ──

func (b *Bridge) handleWorkspaceInfo(ctx context.Context, _ *mcpsdk.CallToolRequest, _ workspaceInfoInput) (*mcpsdk.CallToolResult, struct{}, error) {
	_, finish := spanTool(ctx, "ctx_workspace_info")
	defer finish(nil)

	b.recordToolCall(ctx, "ctx_workspace_info", true)

	ws := b.workspace
	if ws == nil {
		ws = &session.Workspace{
			RootDir:  b.projectDir,
			Projects: []string{b.projectDir},
			Type:     session.WorkspaceSingle,
		}
	}

	info := map[string]any{
		"root_dir":    ws.RootDir,
		"projects":    ws.Projects,
		"type":        string(ws.Type),
		"is_monorepo": ws.IsMonorepo(),
	}

	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return errorResult("marshal failed: " + err.Error()), struct{}{}, nil
	}

	return textResult(string(data)), struct{}{}, nil
}
