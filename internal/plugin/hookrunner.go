package plugin

import (
	"bytes"
	"context"
	"log/slog"
	"os/exec"
	"regexp"
	"runtime"
	"time"
)

// HookResult holds the outcome of a single plugin hook execution.
type HookResult struct {
	Plugin  string
	Command string
	Stdout  string
	Stderr  string
	Err     error
}

// hookTimeout is the maximum duration for a single plugin hook command.
const hookTimeout = 5 * time.Second

// NewHookRunner returns a function that executes plugin hooks for a given event.
// The returned function loads plugins from the given directory and runs matching
// hooks. It is safe to call concurrently.
//
// The returned function signature matches hooks.PluginHookRunner so it can be
// injected into the Dispatcher without the hooks package importing plugin.
func NewHookRunner(pluginDir string) func(event string, toolName string) []HookResult {
	return func(event string, toolName string) []HookResult {
		plugins, err := LoadPlugins(pluginDir)
		if err != nil {
			slog.Warn("plugin hook runner: failed to load plugins", "error", err)
			return nil
		}

		var results []HookResult

		for _, p := range plugins {
			hks, ok := p.Hooks[event]
			if !ok {
				continue
			}

			for _, h := range hks {
				// Check matcher if set (only relevant for Pre/PostToolUse).
				if h.Matcher != "" && toolName != "" {
					matched, err := regexp.MatchString(h.Matcher, toolName)
					if err != nil || !matched {
						continue
					}
				}

				// Skip hooks with matcher when no tool name is provided
				// (e.g. SessionStart, PreCompact).
				if h.Matcher != "" && toolName == "" {
					continue
				}

				r := executeHookCommand(p.Name, h.Command)
				results = append(results, r)
			}
		}

		return results
	}
}

// executeHookCommand runs a shell command with a timeout and captures output.
func executeHookCommand(pluginName, command string) HookResult {
	ctx, cancel := context.WithTimeout(context.Background(), hookTimeout)
	defer cancel()

	var cmd *exec.Cmd

	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/C", command)
	} else {
		cmd = exec.CommandContext(ctx, "bash", "-c", command)
	}

	var stdout, stderr bytes.Buffer

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	return HookResult{
		Plugin:  pluginName,
		Command: command,
		Stdout:  stdout.String(),
		Stderr:  stderr.String(),
		Err:     err,
	}
}
