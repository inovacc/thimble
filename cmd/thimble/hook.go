package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/inovacc/thimble/internal/hooklog"
	"github.com/inovacc/thimble/internal/hooks"
	"github.com/inovacc/thimble/internal/paths"
	"github.com/inovacc/thimble/internal/platform"
	"github.com/inovacc/thimble/internal/plugin"
	"github.com/inovacc/thimble/internal/session"
	"github.com/inovacc/thimble/internal/store"
	"github.com/spf13/cobra"
)

var hookCmd = &cobra.Command{
	Use:   "hook <platform> <event>",
	Short: "Dispatch a hook event (in-process, ~10ms)",
	Args:  cobra.MinimumNArgs(2),
	RunE:  runHook,
}

func init() {
	rootCmd.AddCommand(hookCmd)
}

// eventMap maps platform-specific event names to canonical event names.
var eventMap = map[string]string{
	// Claude Code (PascalCase)
	"pretooluse":       "PreToolUse",
	"posttooluse":      "PostToolUse",
	"precompact":       "PreCompact",
	"sessionstart":     "SessionStart",
	"userpromptsubmit": "UserPromptSubmit",
	// Gemini CLI
	"beforetool":  "PreToolUse",
	"aftertool":   "PostToolUse",
	"precompress": "PreCompact",
	// Already canonical
	"PreToolUse":   "PreToolUse",
	"PostToolUse":  "PostToolUse",
	"PreCompact":   "PreCompact",
	"SessionStart": "SessionStart",
}

func runHook(_ *cobra.Command, args []string) error {
	start := time.Now()

	// Self-heal: check for version mismatch once per day.
	pluginDir := filepath.Join(paths.DataDir(), "plugins")

	healMarker := filepath.Join(paths.DataDir(), fmt.Sprintf("selfheal-%s.marker", time.Now().Format("2006-01-02")))
	if _, err := os.Stat(healMarker); os.IsNotExist(err) {
		logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
		SelfHeal(pluginDir, logger)

		_ = os.MkdirAll(filepath.Dir(healMarker), 0o755)
		_ = os.WriteFile(healMarker, []byte(time.Now().Format(time.RFC3339)), 0o644)
	}

	platformID := platform.PlatformID(args[0])
	rawEvent := args[1]

	dbg := newHookDebugger(string(platformID), rawEvent)

	// Log all received args for debugging.
	if len(args) > 2 {
		argsData, err := json.Marshal(map[string]any{
			"args":      args,
			"arg_count": len(args),
			"timestamp": time.Now().Format(time.RFC3339Nano),
		})
		if err == nil {
			name := fmt.Sprintf("%s_extra_args.json", dbg.ts)
			_ = os.WriteFile(filepath.Join(dbg.dir, name), argsData, 0o644)
		}
	}

	// Resolve platform adapter.
	adapter, err := platform.Get(platformID)
	if err != nil {
		dbg.writeError("resolve_platform", err)
		return fmt.Errorf("unknown platform %q: %w", platformID, err)
	}

	// Normalize event name.
	canonicalEvent, ok := resolveEvent(rawEvent)
	if !ok {
		dbg.writeError("normalize_event", fmt.Errorf("unknown event: %s", rawEvent))
		return fmt.Errorf("unknown event: %s", rawEvent)
	}

	// Read hook payload from stdin.
	payload, err := io.ReadAll(os.Stdin)
	if err != nil {
		dbg.writeError("read_stdin", err)
		return fmt.Errorf("read stdin: %w", err)
	}

	dbg.writeInput(canonicalEvent, payload)

	// Parse input via adapter to extract session/project info.
	normalized := parseNormalizedEvent(adapter, canonicalEvent, json.RawMessage(payload))

	// Build hook payload with normalized data.
	hookPayload := buildHookPayload(normalized, payload)

	// Create in-process hook dispatcher.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_ = ctx // dispatcher methods don't take ctx directly

	hl, _ := hooklog.New(paths.DataDir())

	defer func() {
		if hl != nil {
			_ = hl.Close()
		}
	}()

	// Resolve project dir for database access.
	projectDir := normalized.ProjectDir

	// Lazy session getter — opens DB only if needed.
	var sessDB *session.SessionDB

	getSession := func(pd string) (*session.SessionDB, error) {
		if sessDB != nil {
			return sessDB, nil
		}

		dir := paths.DataDir()
		if pd != "" {
			dir = paths.ProjectDataDir(pd)
		}

		_ = os.MkdirAll(dir, 0o755)

		db, err := session.New(filepath.Join(dir, "session.db"))
		if err != nil {
			return nil, err
		}

		sessDB = db

		return sessDB, nil
	}

	// Lazy store getter — opens DB only if needed.
	var contentStore *store.ContentStore

	getStore := func(pd string) (*store.ContentStore, error) {
		if contentStore != nil {
			return contentStore, nil
		}

		dir := paths.DataDir()
		if pd != "" {
			dir = paths.ProjectDataDir(pd)
		}

		_ = os.MkdirAll(dir, 0o755)

		cs, err := store.New(filepath.Join(dir, "content.db"))
		if err != nil {
			return nil, err
		}

		contentStore = cs

		return contentStore, nil
	}

	defer func() {
		if sessDB != nil {
			sessDB.Close()
		}

		if contentStore != nil {
			contentStore.Close()
		}
	}()

	dispatcher := hooks.NewDispatcher(getSession, getStore, hl)

	// Inject plugin hook runner.
	pluginRunner := plugin.NewHookRunner(pluginDir)

	dispatcher.SetPluginHookRunner(func(event string, toolName string) []hooks.PluginHookResult {
		pResults := pluginRunner(event, toolName)
		results := make([]hooks.PluginHookResult, len(pResults))

		for i, r := range pResults {
			results[i] = hooks.PluginHookResult{
				Plugin:  r.Plugin,
				Command: r.Command,
				Stdout:  r.Stdout,
				Stderr:  r.Stderr,
				Err:     r.Err,
			}
		}

		return results
	})

	var resp *hooks.Response

	switch canonicalEvent {
	case "PreToolUse":
		resp, err = dispatcher.PreToolUse([]byte(platformID), hookPayload)
	case "PostToolUse":
		resp, err = dispatcher.PostToolUse(string(platformID), canonicalEvent, hookPayload)
	case "SessionStart":
		resp, err = dispatcher.SessionStart(string(platformID), hookPayload)
	case "PreCompact":
		resp, err = dispatcher.PreCompact(string(platformID), hookPayload)
	case "UserPromptSubmit":
		// Route through PostToolUse to record user prompt as session event.
		resp, err = dispatcher.PostToolUse(string(platformID), canonicalEvent, hookPayload)
	default:
		return fmt.Errorf("unknown canonical event: %s", canonicalEvent)
	}

	_ = projectDir // used by getSession/getStore closures above

	if err != nil {
		dbg.writeError("hook_dispatch", err)
		return fmt.Errorf("hook %s/%s: %w", platformID, canonicalEvent, err)
	}

	// Format response via platform adapter.
	output := formatViaAdapter(adapter, canonicalEvent, resp)
	if output != "" {
		_, _ = fmt.Fprint(os.Stdout, output)
	}

	dbg.writeOutput(canonicalEvent, output, time.Since(start))

	return nil
}

// prepareHookPayload handles the pre-dispatch portion of hook dispatch:
// resolves platform, normalizes event, reads stdin, and builds the hook payload.
func prepareHookPayload(args []string, stdinPayload []byte) (platform.Adapter, string, []byte, *hookDebugger, error) {
	platformID := platform.PlatformID(args[0])
	rawEvent := args[1]
	dbg := newHookDebugger(string(platformID), rawEvent)

	// Log extra args.
	if len(args) > 2 {
		argsData, err := json.Marshal(map[string]any{
			"args":      args,
			"arg_count": len(args),
			"timestamp": time.Now().Format(time.RFC3339Nano),
		})
		if err == nil {
			name := fmt.Sprintf("%s_extra_args.json", dbg.ts)
			_ = os.WriteFile(filepath.Join(dbg.dir, name), argsData, 0o644)
		}
	}

	adapter, err := platform.Get(platformID)
	if err != nil {
		dbg.writeError("resolve_platform", err)
		return nil, "", nil, dbg, fmt.Errorf("unknown platform %q: %w", platformID, err)
	}

	canonicalEvent, ok := resolveEvent(rawEvent)
	if !ok {
		dbg.writeError("normalize_event", fmt.Errorf("unknown event: %s", rawEvent))
		return nil, "", nil, dbg, fmt.Errorf("unknown event: %s", rawEvent)
	}

	dbg.writeInput(canonicalEvent, stdinPayload)

	normalized := parseNormalizedEvent(adapter, canonicalEvent, json.RawMessage(stdinPayload))
	hookPayload := buildHookPayload(normalized, stdinPayload)

	return adapter, canonicalEvent, hookPayload, dbg, nil
}

// resolveEvent maps a raw event name to its canonical form.
func resolveEvent(rawEvent string) (string, bool) {
	canonicalEvent, ok := eventMap[strings.ToLower(rawEvent)]
	if !ok {
		canonicalEvent, ok = eventMap[rawEvent]
	}

	return canonicalEvent, ok
}

// parseNormalizedEvent uses the platform adapter to parse a raw payload
// into a NormalizedEvent based on the canonical event type.
func parseNormalizedEvent(adapter platform.Adapter, canonicalEvent string, payload json.RawMessage) platform.NormalizedEvent {
	var normalized platform.NormalizedEvent

	switch canonicalEvent {
	case "PreToolUse":
		normalized = adapter.ParsePreToolUse(payload)
	case "PostToolUse", "UserPromptSubmit":
		normalized = adapter.ParsePostToolUse(payload)
	case "PreCompact":
		normalized = adapter.ParsePreCompact(payload)
	case "SessionStart":
		normalized = adapter.ParseSessionStart(payload)
	}

	return normalized
}

// buildHookPayload creates the HookInput JSON, enriching the raw payload with
// normalized session/project data.
func buildHookPayload(normalized platform.NormalizedEvent, rawPayload []byte) []byte {
	hookInput := map[string]any{
		"sessionId":  normalized.SessionID,
		"projectDir": normalized.ProjectDir,
	}

	if normalized.ToolName != "" {
		hookInput["toolCall"] = map[string]any{
			"toolName":  normalized.ToolName,
			"toolInput": normalized.ToolInput,
		}
		if normalized.ToolOutput != "" {
			hookInput["toolCall"].(map[string]any)["toolResponse"] = normalized.ToolOutput
			hookInput["toolCall"].(map[string]any)["isError"] = normalized.IsError
		}
	}

	if normalized.Source != "" {
		hookInput["source"] = normalized.Source
	}

	var rawMap map[string]any
	if err := json.Unmarshal(rawPayload, &rawMap); err == nil {
		hookInput["extra"] = rawMap
	}

	data, err := json.Marshal(hookInput)
	if err != nil {
		return nil
	}

	return data
}

// hookDebugger writes hook inputs, outputs, and errors to the debug directory.
type hookDebugger struct {
	dir string
	ts  string
}

func newHookDebugger(_, _ string) *hookDebugger {
	debugDir := filepath.Join(paths.DataDir(), "debug")
	_ = os.MkdirAll(debugDir, 0o755)

	return &hookDebugger{
		dir: debugDir,
		ts:  time.Now().Format("20060102_150405.000"),
	}
}

func (d *hookDebugger) writeInput(event string, payload []byte) {
	name := fmt.Sprintf("%s_%s_input.json", d.ts, event)
	_ = os.WriteFile(filepath.Join(d.dir, name), payload, 0o644)
}

func (d *hookDebugger) writeOutput(event, output string, duration time.Duration) {
	entry := map[string]any{
		"event":       event,
		"output":      output,
		"duration_ms": duration.Milliseconds(),
		"timestamp":   d.ts,
	}

	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return
	}

	name := fmt.Sprintf("%s_%s_output.json", d.ts, event)
	_ = os.WriteFile(filepath.Join(d.dir, name), data, 0o644)
}

func (d *hookDebugger) writeError(phase string, err error) {
	entry := map[string]any{
		"phase":     phase,
		"error":     err.Error(),
		"timestamp": d.ts,
	}

	data, marshalErr := json.MarshalIndent(entry, "", "  ")
	if marshalErr != nil {
		return
	}

	name := fmt.Sprintf("%s_%s_error.json", d.ts, phase)
	_ = os.WriteFile(filepath.Join(d.dir, name), data, 0o644)
}

// formatViaAdapter converts a hook response to platform-specific output.
func formatViaAdapter(adapter platform.Adapter, event string, resp *hooks.Response) string {
	if resp == nil {
		return "{}"
	}

	hookResp := platform.HookResponse{Decision: "allow"}

	if resp.Blocked {
		hookResp.Decision = "deny"
		hookResp.Reason = resp.Reason
	}

	if len(resp.Result) > 0 {
		var resultMap map[string]any
		if err := json.Unmarshal(resp.Result, &resultMap); err == nil {
			// Explicit keys (forward-compatible).
			if ctx, ok := resultMap["additionalContext"].(string); ok {
				hookResp.AdditionalContext = ctx
				if hookResp.Decision == "allow" {
					hookResp.Decision = "context"
				}
			}

			if ctx, ok := resultMap["context"].(string); ok {
				hookResp.Context = ctx
			}

			// The HookOutput.Result under the "result" key.
			// Type-switch to distinguish context (string) from modify (object).
			if hookResp.Context == "" && hookResp.AdditionalContext == "" {
				switch r := resultMap["result"].(type) {
				case string:
					if r != "" {
						hookResp.Context = r

						hookResp.AdditionalContext = r
						if hookResp.Decision == "allow" {
							hookResp.Decision = "context"
						}
					}
				case map[string]any:
					hookResp.UpdatedInput = r
					if hookResp.Decision == "allow" {
						hookResp.Decision = "modify"
					}
				}
			}
		}
	}

	var output json.RawMessage

	switch event {
	case "PreToolUse":
		output = adapter.FormatPreToolUse(hookResp)
	case "PostToolUse":
		output = adapter.FormatPostToolUse(hookResp)
	case "PreCompact":
		output = adapter.FormatPreCompact(hookResp)
	case "SessionStart":
		output = adapter.FormatSessionStart(hookResp)
	default:
		return ""
	}

	if output == nil {
		return ""
	}

	return string(output)
}
