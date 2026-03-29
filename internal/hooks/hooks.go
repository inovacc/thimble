// Package hooks implements the hook dispatch logic for PreToolUse, PostToolUse,
// SessionStart, and PreCompact events. It enforces security policies, records
// session events, auto-indexes files, and provides one-time guidance advisories.
package hooks

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/inovacc/thimble/internal/hooklog"
	"github.com/inovacc/thimble/internal/model"
	"github.com/inovacc/thimble/internal/paths"
	"github.com/inovacc/thimble/internal/platform"
	"github.com/inovacc/thimble/internal/routing"
	"github.com/inovacc/thimble/internal/security"
	"github.com/inovacc/thimble/internal/session"
	"github.com/inovacc/thimble/internal/store"
	"github.com/inovacc/thimble/internal/tracing"
)

// Response is the result of a hook dispatch.
type Response struct {
	Result  []byte
	Blocked bool
	Reason  string
}

// PluginHookResult holds the outcome of a single plugin hook execution.
type PluginHookResult struct {
	Plugin  string
	Command string
	Stdout  string
	Stderr  string
	Err     error
}

// PluginHookRunner executes plugin hooks for a given lifecycle event.
// The toolName parameter is used for matcher filtering on Pre/PostToolUse events.
// Implementations must not block longer than a reasonable timeout (e.g. 5s per hook).
type PluginHookRunner func(event string, toolName string) []PluginHookResult

// SessionGetter resolves a SessionDB for a given project directory.
type SessionGetter func(projectDir string) (*session.SessionDB, error)

// StoreGetter resolves a ContentStore for a given project directory.
type StoreGetter func(projectDir string) (*store.ContentStore, error)

// GoalProvider returns the current active goal string (empty if none).
type GoalProvider func() string

// Dispatcher handles hook events using direct service references.
type Dispatcher struct {
	getSession       SessionGetter
	getStore         StoreGetter
	guidance         *guidanceTracker
	hookLog          *hooklog.Logger
	pluginHookRunner PluginHookRunner
	goalProvider     GoalProvider
}

// NewDispatcher creates a new hook dispatcher.
func NewDispatcher(getSession SessionGetter, getStore StoreGetter, hookLog *hooklog.Logger) *Dispatcher {
	return &Dispatcher{
		getSession: getSession,
		getStore:   getStore,
		guidance:   newGuidanceTracker(),
		hookLog:    hookLog,
	}
}

// SetGoalProvider injects a goal provider into the dispatcher.
// When set, recorded events will include the active goal tag.
func (d *Dispatcher) SetGoalProvider(provider GoalProvider) {
	d.goalProvider = provider
}

// SetPluginHookRunner injects a plugin hook runner into the dispatcher.
// This avoids a direct import of the plugin package from hooks.
func (d *Dispatcher) SetPluginHookRunner(runner PluginHookRunner) {
	d.pluginHookRunner = runner
}

// runPluginHooks executes plugin hooks for the given event and tool name.
// Results are logged but do not affect the main hook response.
func (d *Dispatcher) runPluginHooks(event, toolName string) {
	if d.pluginHookRunner == nil {
		return
	}

	results := d.pluginHookRunner(event, toolName)
	for _, r := range results {
		if r.Err != nil {
			slog.Warn("plugin hook failed",
				"plugin", r.Plugin,
				"event", event,
				"command", r.Command,
				"error", r.Err,
			)
		} else if r.Stderr != "" {
			slog.Debug("plugin hook stderr",
				"plugin", r.Plugin,
				"event", event,
				"stderr", r.Stderr,
			)
		}
	}
}

// startHookSpan starts an OTel span for a hook dispatch event.
// Returns a finish function that records duration and error status.
func startHookSpan(event, platformStr string, attrs ...attribute.KeyValue) func(error) { //nolint:unparam // variadic attrs reserved for future callers
	if !tracing.Enabled() {
		return func(error) {}
	}

	baseAttrs := make([]attribute.KeyValue, 0, 2+len(attrs))
	baseAttrs = append(baseAttrs,
		attribute.String("hook.event", event),
		attribute.String("hook.platform", platformStr),
	)
	baseAttrs = append(baseAttrs, attrs...)

	_, span := tracing.Tracer().Start( //nolint:spancheck // span.End called inside returned closure
		context.Background(),
		"hook."+event,
		trace.WithAttributes(baseAttrs...),
		trace.WithSpanKind(trace.SpanKindInternal),
	)

	hookStart := time.Now()

	return func(err error) { //nolint:spancheck // span.End called below
		span.SetAttributes(attribute.Int64("duration_ms", time.Since(hookStart).Milliseconds()))

		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		} else {
			span.SetStatus(codes.Ok, "")
		}

		span.End()
	}
}

func (d *Dispatcher) logHook(entry hooklog.Entry) {
	if d.hookLog != nil {
		d.hookLog.Log(entry)
	}
}

// PreToolUse handles pre-tool-use hook events (security checks, guidance).
func (d *Dispatcher) PreToolUse(platformID, payload []byte) (resp *Response, retErr error) { //nolint:maintidx // security dispatcher, complexity is inherent
	finishSpan := startHookSpan("PreToolUse", string(platformID))

	defer func() { finishSpan(retErr) }()

	start := time.Now()

	var input model.HookInput
	if err := json.Unmarshal(payload, &input); err != nil {
		return nil, fmt.Errorf("unmarshal hook input: %w", err)
	}

	defer func() {
		entry := hooklog.Entry{
			Platform:    string(platformID),
			Event:       "PreToolUse",
			SessionID:   input.SessionID,
			ProjectDir:  input.ProjectDir,
			DurationMs:  time.Since(start).Milliseconds(),
			GRPCPayload: string(payload),
		}
		if input.ToolCall != nil {
			entry.ToolName = input.ToolCall.ToolName
			if toolJSON, err := json.Marshal(input.ToolCall.ToolInput); err == nil {
				entry.ToolInput = string(toolJSON)
			}
		}

		if resp != nil {
			entry.Blocked = resp.Blocked
			entry.Reason = resp.Reason
			entry.HasContext = len(resp.Result) > 0 && !resp.Blocked
			entry.Response = string(resp.Result)
		}

		if retErr != nil {
			entry.Error = retErr.Error()
		}

		d.logHook(entry)
	}()

	// Normalize platform-specific tool names to canonical names.
	if input.ToolCall != nil {
		input.ToolCall.ToolName = security.NormalizeToolName(input.ToolCall.ToolName)
	}

	// Security check for Bash tool.
	if input.ToolCall != nil && input.ToolCall.ToolName == "Bash" {
		command, _ := input.ToolCall.ToolInput["command"].(string)
		if command != "" {
			policies := security.ReadBashPolicies(input.ProjectDir, "")

			decision := security.EvaluateCommandDenyOnly(command, policies)
			if decision.Decision == security.Deny {
				output := model.HookOutput{
					Blocked: true,
					Reason:  fmt.Sprintf("Command denied by policy: %s", decision.MatchedPattern),
				}

				data, err := json.Marshal(output)
				if err != nil {
					return &Response{}, nil
				}

				return &Response{Result: data, Blocked: true, Reason: output.Reason}, nil
			}

			// Check dangerous git/gh subcommands with configurable overrides.
			gitPolicy, ghPolicy := security.CustomPoliciesFromSettings(policies)

			segments := security.SplitChainedCommands(command)
			for _, seg := range segments {
				if pattern := security.IsGitCommandDangerousWithPolicy(seg, gitPolicy); pattern != "" {
					output := model.HookOutput{
						Blocked: true,
						Reason:  fmt.Sprintf("Dangerous git command denied: %s", pattern),
					}

					data, err := json.Marshal(output)
					if err != nil {
						return &Response{}, nil
					}

					return &Response{Result: data, Blocked: true, Reason: output.Reason}, nil
				}

				if pattern := security.IsGhCommandDangerousWithPolicy(seg, ghPolicy); pattern != "" {
					output := model.HookOutput{
						Blocked: true,
						Reason:  fmt.Sprintf("Dangerous gh command denied: %s", pattern),
					}

					data, err := json.Marshal(output)
					if err != nil {
						return &Response{}, nil
					}

					return &Response{Result: data, Blocked: true, Reason: output.Reason}, nil
				}
			}
		}
	}

	// Check shell-escape in code execution tools.
	if input.ToolCall != nil {
		toolName := input.ToolCall.ToolName
		if toolName == "ctx_execute" || toolName == "ctx_execute_file" ||
			toolName == "mcp__thimble__ctx_execute" || toolName == "mcp__thimble__ctx_execute_file" {
			code, _ := input.ToolCall.ToolInput["code"].(string)

			lang, _ := input.ToolCall.ToolInput["language"].(string)
			if code != "" && lang != "" {
				cmds := security.ExtractShellCommands(code, lang)
				for _, cmd := range cmds {
					policies := security.ReadBashPolicies(input.ProjectDir, "")

					decision := security.EvaluateCommandDenyOnly(cmd, policies)
					if decision.Decision == security.Deny {
						output := model.HookOutput{
							Blocked: true,
							Reason:  fmt.Sprintf("Shell escape denied: %s (in %s code)", decision.MatchedPattern, lang),
						}

						data, err := json.Marshal(output)
						if err != nil {
							return &Response{}, nil
						}

						return &Response{Result: data, Blocked: true, Reason: output.Reason}, nil
					}
				}
			}
		}

		// Check shell-escape in batch execution tools.
		if toolName == "ctx_batch_execute" || toolName == "mcp__thimble__ctx_batch_execute" {
			rawCmds, _ := input.ToolCall.ToolInput["commands"].([]any)
			policies := security.ReadBashPolicies(input.ProjectDir, "")

			for _, raw := range rawCmds {
				cmd, _ := raw.(string)
				if cmd == "" {
					continue
				}

				decision := security.EvaluateCommandDenyOnly(cmd, policies)
				if decision.Decision == security.Deny {
					output := model.HookOutput{
						Blocked: true,
						Reason:  fmt.Sprintf("Command denied by policy: %s", decision.MatchedPattern),
					}

					data, err := json.Marshal(output)
					if err != nil {
						return &Response{}, nil
					}

					return &Response{Result: data, Blocked: true, Reason: output.Reason}, nil
				}
			}
		}
	}

	// File path check for Read/Edit/Write tools.
	if input.ToolCall != nil {
		toolName := input.ToolCall.ToolName
		if toolName == "Read" || toolName == "Edit" || toolName == "Write" {
			filePath, _ := input.ToolCall.ToolInput["file_path"].(string)
			if filePath != "" {
				denyGlobs := security.ReadToolDenyPatterns(toolName, input.ProjectDir, "")

				decision := security.EvaluateFilePath(filePath, denyGlobs)
				if decision.Denied {
					output := model.HookOutput{
						Blocked: true,
						Reason:  fmt.Sprintf("File access denied: %s", decision.MatchedPattern),
					}

					data, err := json.Marshal(output)
					if err != nil {
						return &Response{Blocked: true, Reason: output.Reason}, nil
					}

					return &Response{Result: data, Blocked: true, Reason: output.Reason}, nil
				}
			}
		}
	}

	// curl/wget, inline HTTP, and build tool advisories in Bash commands.
	if input.ToolCall != nil && input.ToolCall.ToolName == "Bash" {
		command, _ := input.ToolCall.ToolInput["command"].(string)
		if command != "" {
			netResult := security.DetectNetworkCommand(command)
			if netResult.IsInlineHTTP || netResult.IsCurlWget {
				advisory := "Consider using ctx_fetch_and_index instead of curl/wget for better context management."
				output := model.HookOutput{Result: advisory}
				data, _ := json.Marshal(output) //nolint:errchkjson

				return &Response{Result: data}, nil
			}

			if tool := security.DetectBuildTool(command); tool != "" {
				advisory := fmt.Sprintf("Consider using ctx_execute instead of %s to manage output in the knowledge base.", tool)
				output := model.HookOutput{Result: advisory}
				data, _ := json.Marshal(output) //nolint:errchkjson

				return &Response{Result: data}, nil
			}
		}
	}

	// WebFetch advisory — suggest ctx_fetch_and_index.
	if input.ToolCall != nil && security.IsWebFetchBlocked(input.ToolCall.ToolName) {
		advisory := "Consider using ctx_fetch_and_index instead of WebFetch for better context management."
		output := model.HookOutput{Result: advisory}
		data, _ := json.Marshal(output) //nolint:errchkjson

		return &Response{Result: data}, nil
	}

	// Agent/Task routing injection.
	if input.ToolCall != nil && security.IsAgentOrTask(input.ToolCall.ToolName) {
		instructions := routing.GenerateInstructions(platform.Detect().Platform)

		if subType, ok := input.ToolCall.ToolInput["subagent_type"].(string); ok && subType == "Bash" {
			input.ToolCall.ToolInput["subagent_type"] = "general-purpose"
		}

		promptFields := []string{"prompt", "request", "objective", "question", "query", "task"}
		injected := false

		for _, field := range promptFields {
			if val, ok := input.ToolCall.ToolInput[field].(string); ok && val != "" {
				input.ToolCall.ToolInput[field] = val + "\n\n" + instructions
				injected = true

				break
			}
		}

		if injected {
			output := model.HookOutput{Result: input.ToolCall.ToolInput}
			data, _ := json.Marshal(output) //nolint:errchkjson

			return &Response{Result: data}, nil
		}

		output := model.HookOutput{Result: instructions}
		data, _ := json.Marshal(output) //nolint:errchkjson

		return &Response{Result: data}, nil
	}

	// One-time guidance advisories for Read, Grep, Bash tools.
	if input.ToolCall != nil {
		toolName := input.ToolCall.ToolName

		sessionID := input.SessionID
		if sessionID == "" {
			sessionID = "default"
		}

		switch toolName {
		case "Read":
			if d.guidance.ShouldShow(sessionID, AdvisoryReadForAnalysis) {
				advisory := "If reading to Edit, Read is correct. For analysis, consider using ctx_execute_file instead — it keeps raw output in the knowledge base."
				output := model.HookOutput{Result: advisory}
				data, _ := json.Marshal(output) //nolint:errchkjson

				return &Response{Result: data}, nil
			}
		case "Grep":
			if d.guidance.ShouldShow(sessionID, AdvisoryGrepContextFlood) {
				advisory := "Grep results can flood the context window. Consider using ctx_search for knowledge-base-backed search that stays sandboxed."
				output := model.HookOutput{Result: advisory}
				data, _ := json.Marshal(output) //nolint:errchkjson

				return &Response{Result: data}, nil
			}
		case "Bash":
			if d.guidance.ShouldShow(sessionID, AdvisoryBashLargeOutput) {
				advisory := "For commands producing large output, consider using ctx_batch_execute to keep output in the knowledge base."
				output := model.HookOutput{Result: advisory}
				data, _ := json.Marshal(output) //nolint:errchkjson

				return &Response{Result: data}, nil
			}
		}
	}

	// Run plugin hooks for PreToolUse.
	toolName := ""
	if input.ToolCall != nil {
		toolName = input.ToolCall.ToolName
	}

	d.runPluginHooks("PreToolUse", toolName)

	output := model.HookOutput{}
	data, _ := json.Marshal(output) //nolint:errchkjson

	return &Response{Result: data}, nil
}

// PostToolUse handles post-tool-use hook events (session recording, auto-indexing).
func (d *Dispatcher) PostToolUse(platformStr string, eventName string, payload []byte) (resp *Response, retErr error) {
	finishSpan := startHookSpan("PostToolUse", platformStr)

	defer func() { finishSpan(retErr) }()

	start := time.Now()

	var input model.HookInput
	if err := json.Unmarshal(payload, &input); err != nil {
		return nil, fmt.Errorf("unmarshal hook input: %w", err)
	}

	defer func() {
		entry := hooklog.Entry{
			Platform:    platformStr,
			Event:       eventName,
			SessionID:   input.SessionID,
			ProjectDir:  input.ProjectDir,
			DurationMs:  time.Since(start).Milliseconds(),
			GRPCPayload: string(payload),
		}
		if input.ToolCall != nil {
			entry.ToolName = input.ToolCall.ToolName
			if toolJSON, err := json.Marshal(input.ToolCall.ToolInput); err == nil {
				entry.ToolInput = string(toolJSON)
			}
		}

		if retErr != nil {
			entry.Error = retErr.Error()
		}

		d.logHook(entry)
	}()

	// Extract events from user messages (UserPromptSubmit).
	if input.Message != nil && input.ToolCall == nil && input.Message.Content != "" {
		userEvents := session.ExtractUserEvents(input.Message.Content)
		if len(userEvents) > 0 && input.ProjectDir != "" {
			d.recordEvents(input.ProjectDir, userEvents)
		}
	}

	// Extract events from tool calls and user messages.
	events := session.ExtractEvents(input)
	if len(events) > 0 && input.ProjectDir != "" {
		d.recordEvents(input.ProjectDir, events)

		// Auto-index file content for file_read/file_write/file_edit events.
		d.autoIndexFiles(events, input.ProjectDir)
	}

	// Run plugin hooks for PostToolUse.
	postToolName := ""
	if input.ToolCall != nil {
		postToolName = input.ToolCall.ToolName
	}

	d.runPluginHooks("PostToolUse", postToolName)

	output := model.HookOutput{}
	data, _ := json.Marshal(output) //nolint:errchkjson

	return &Response{Result: data}, nil
}

// SessionStart handles session start events (routing instructions, resume directives).
func (d *Dispatcher) SessionStart(platformStr string, payload []byte) (resp *Response, retErr error) {
	finishSpan := startHookSpan("SessionStart", platformStr)

	defer func() { finishSpan(retErr) }()

	start := time.Now()

	var input model.HookInput
	if err := json.Unmarshal(payload, &input); err != nil {
		return nil, fmt.Errorf("unmarshal hook input: %w", err)
	}

	defer func() {
		entry := hooklog.Entry{
			Platform:    platformStr,
			Event:       "SessionStart",
			SessionID:   input.SessionID,
			ProjectDir:  input.ProjectDir,
			DurationMs:  time.Since(start).Milliseconds(),
			HasContext:  resp != nil && len(resp.Result) > 0,
			GRPCPayload: string(payload),
		}
		if resp != nil {
			entry.Response = string(resp.Result)
		}

		if retErr != nil {
			entry.Error = retErr.Error()
		}

		d.logHook(entry)
	}()

	source := "startup"

	if input.Extra != nil {
		if s, ok := input.Extra["source"].(string); ok && s != "" {
			source = s
		}
	}

	if input.ProjectDir != "" {
		db, err := d.getSession(input.ProjectDir)
		if err == nil {
			_ = db.EnsureSession("default", input.ProjectDir)
		}

		projDataDir := paths.ProjectDataDir(input.ProjectDir)
		cleanupMarker := filepath.Join(projDataDir, "cleanup.marker")

		switch source {
		case "startup":
			_ = os.MkdirAll(projDataDir, 0o755)
			_ = os.WriteFile(cleanupMarker, []byte(time.Now().UTC().Format(time.RFC3339)), 0o644)
		case "resume":
			_ = os.Remove(cleanupMarker)
		}
	}

	var directive string

	if input.ProjectDir != "" && (source == "resume" || source == "compact") {
		db, dbErr := d.getSession(input.ProjectDir)
		if dbErr == nil {
			events, evErr := db.GetEvents("default", nil)
			if evErr == nil && len(events) > 0 {
				directive = session.BuildSessionDirective(source, events)
			}
		}
	}

	var result string
	if source != "clear" {
		result = routing.GenerateInstructions(platform.Detect().Platform)
	}

	if directive != "" {
		if result != "" {
			result = result + "\n\n" + directive
		} else {
			result = directive
		}
	}

	// Run plugin hooks for SessionStart.
	d.runPluginHooks("SessionStart", "")

	output := model.HookOutput{}
	if result != "" {
		output.Result = result
	}

	data, _ := json.Marshal(output) //nolint:errchkjson

	return &Response{Result: data}, nil
}

// PreCompact handles pre-compact events (resume snapshot, session directive).
func (d *Dispatcher) PreCompact(platformStr string, payload []byte) (resp *Response, retErr error) {
	finishSpan := startHookSpan("PreCompact", platformStr)

	defer func() { finishSpan(retErr) }()

	start := time.Now()

	var input model.HookInput
	if err := json.Unmarshal(payload, &input); err != nil {
		return nil, fmt.Errorf("unmarshal hook input: %w", err)
	}

	defer func() {
		entry := hooklog.Entry{
			Platform:    platformStr,
			Event:       "PreCompact",
			SessionID:   input.SessionID,
			ProjectDir:  input.ProjectDir,
			DurationMs:  time.Since(start).Milliseconds(),
			HasContext:  resp != nil && len(resp.Result) > 0,
			GRPCPayload: string(payload),
		}
		if resp != nil {
			entry.Response = string(resp.Result)
		}

		if retErr != nil {
			entry.Error = retErr.Error()
		}

		d.logHook(entry)
	}()

	var additionalContext string

	if input.ProjectDir != "" {
		db, err := d.getSession(input.ProjectDir)
		if err == nil {
			events, err := db.GetEvents("default", nil)
			if err == nil && len(events) > 0 {
				snapshot := session.BuildResumeSnapshot(events, nil)
				_ = db.UpsertResume("default", snapshot, len(events))
				additionalContext = session.BuildSessionDirective("compact", events)
			}
		}
	}

	// Run plugin hooks for PreCompact.
	d.runPluginHooks("PreCompact", "")

	output := model.HookOutput{}
	if additionalContext != "" {
		output.Result = additionalContext
	}

	data, _ := json.Marshal(output) //nolint:errchkjson

	return &Response{Result: data}, nil
}

// recordEvents inserts session events into the database for a project.
// When a goal provider is set and returns a non-empty goal, a companion
// goal-tagged event is recorded alongside each original event.
func (d *Dispatcher) recordEvents(projectDir string, events []model.SessionEvent) {
	db, err := d.getSession(projectDir)
	if err != nil {
		return
	}

	_ = db.EnsureSession("default", projectDir)

	var goal string
	if d.goalProvider != nil {
		goal = d.goalProvider()
	}

	for _, ev := range events {
		_ = db.InsertEvent("default", ev, "hook")

		// Record a companion goal-tagged event for snapshot grouping.
		if goal != "" {
			goalEv := model.SessionEvent{
				Type:     ev.Type,
				Category: ev.Category,
				Data:     fmt.Sprintf(`{"goal":"%s","type":"%s","ref":"%s"}`, goal, ev.Type, truncateForGoal(ev.Data, 200)),
				Priority: ev.Priority,
			}
			_ = db.InsertEvent("default", goalEv, "hook")
		}
	}
}

// truncateForGoal truncates and escapes a string for embedding in JSON.
func truncateForGoal(s string, maxLen int) string {
	if len(s) > maxLen {
		s = s[:maxLen]
	}

	// Escape quotes and backslashes for safe JSON embedding.
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)

	return s
}

// autoIndexMaxFileSize is the maximum file size (1MB) to auto-index.
const autoIndexMaxFileSize = 1 << 20

// autoIndexFiles reads and indexes files referenced by file events.
func (d *Dispatcher) autoIndexFiles(events []model.SessionEvent, projectDir string) {
	for _, ev := range events {
		if ev.Category != "file" {
			continue
		}

		switch ev.Type {
		case "file_read", "file_write", "file_edit":
		default:
			continue
		}

		filePath := ev.Data
		if filePath == "" {
			continue
		}

		info, err := os.Stat(filePath)
		if err != nil || info.IsDir() || info.Size() > autoIndexMaxFileSize || info.Size() == 0 {
			continue
		}

		content, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		probe := content
		if len(probe) > 512 {
			probe = probe[:512]
		}

		if slices.Contains(probe, 0) {
			continue
		}

		ct := "plain"

		ext := strings.ToLower(filepath.Ext(filePath))
		switch ext {
		case ".md", ".markdown":
			ct = "markdown"
		case ".json":
			ct = "json"
		}

		label := fmt.Sprintf("file:%s:%s", ev.Type, filepath.Base(filePath))

		cs, err := d.getStore(projectDir)
		if err != nil {
			continue
		}

		switch ct {
		case "json":
			_, _ = cs.IndexJSON(string(content), label)
		case "plain":
			_, _ = cs.IndexPlainText(string(content), label, 20)
		default:
			_, _ = cs.Index(string(content), label)
		}
	}
}
