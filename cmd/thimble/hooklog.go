package main

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/inovacc/thimble/internal/hooklog"
	"github.com/inovacc/thimble/internal/paths"
	"github.com/inovacc/thimble/internal/report"
	"github.com/spf13/cobra"
)

var hooklogCmd = &cobra.Command{
	Use:   "hooklog",
	Short: "Show hook interaction logs",
	Long: `Show hook interaction logs.

Debug payload capture is enabled by default. Set THIMBLE_HOOKLOG_DEBUG=0 to disable.
Use --debug to display full payloads (tool input, response, payload).`,
	RunE: runHooklog,
}

var (
	flagHooklogLimit    int
	flagHooklogPlatform string
	flagHooklogEvent    string
	flagHooklogTool     string
	flagHooklogBlocked  bool
	flagHooklogClear    bool
	flagHooklogReport   bool
	flagHooklogDebug    bool
)

func init() {
	hooklogCmd.Flags().IntVarP(&flagHooklogLimit, "limit", "n", 50, "max entries to show (0 = all)")
	hooklogCmd.Flags().StringVar(&flagHooklogPlatform, "platform", "", "filter by platform")
	hooklogCmd.Flags().StringVar(&flagHooklogEvent, "event", "", "filter by event type")
	hooklogCmd.Flags().StringVar(&flagHooklogTool, "tool", "", "filter by tool name")
	hooklogCmd.Flags().BoolVar(&flagHooklogBlocked, "blocked", false, "show only blocked interactions")
	hooklogCmd.Flags().BoolVar(&flagHooklogClear, "clear", false, "clear the hook log")
	hooklogCmd.Flags().BoolVar(&flagHooklogReport, "report", false, "save results as an AI-consumable report")
	hooklogCmd.Flags().BoolVar(&flagHooklogDebug, "debug", false, "show full debug payloads (enabled by default, set THIMBLE_HOOKLOG_DEBUG=0 to disable)")
	rootCmd.AddCommand(hooklogCmd)
}

func runHooklog(_ *cobra.Command, _ []string) error {
	hl, err := hooklog.New(paths.DataDir())
	if err != nil {
		return fmt.Errorf("open hook log: %w", err)
	}

	defer func() { _ = hl.Close() }()

	if flagHooklogClear {
		if err := hl.Clear(); err != nil {
			return fmt.Errorf("clear hook log: %w", err)
		}

		_, _ = fmt.Fprintln(os.Stdout, "Hook log cleared.")

		return nil
	}

	opts := &hooklog.ReadOptions{
		Limit:       flagHooklogLimit,
		Platform:    flagHooklogPlatform,
		Event:       flagHooklogEvent,
		ToolName:    flagHooklogTool,
		BlockedOnly: flagHooklogBlocked,
	}

	entries, err := hl.Read(opts)
	if err != nil {
		return fmt.Errorf("list hook logs: %w", err)
	}

	if len(entries) == 0 {
		_, _ = fmt.Fprintln(os.Stdout, "No matching hook log entries.")
		return nil
	}

	total, allowed, blocked, _ := hl.Stats()

	_, _ = fmt.Fprintf(os.Stdout, "Hook log: %d entries shown (total: %d, allowed: %d, blocked: %d)\n\n",
		len(entries), total, allowed, blocked)

	if flagHooklogDebug {
		printDebugEntries(entries)
	} else {
		printTableEntries(entries)
	}

	if flagHooklogReport {
		saveHooklogReport(entries, allowed, blocked)
	}

	return nil
}

func printTableEntries(entries []hooklog.Entry) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "TIME\tEVENT\tTOOL\tBLOCKED\tDURATION\tREASON")

	for _, e := range entries {
		tsStr := e.Timestamp.Local().Format("15:04:05")

		toolName := e.ToolName
		if toolName == "" {
			toolName = "-"
		}

		blockedStr := ""
		if e.Blocked {
			blockedStr = "BLOCKED"
		}

		reason := e.Reason
		if len(reason) > 60 {
			reason = reason[:57] + "..."
		}

		if e.Error != "" {
			if reason != "" {
				reason = reason + " [err: " + e.Error + "]"
			} else {
				reason = "err: " + e.Error
			}
		}

		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%dms\t%s\n",
			tsStr, e.Event, toolName, blockedStr, e.DurationMs, reason)
	}

	_ = w.Flush()
}

func printDebugEntries(entries []hooklog.Entry) {
	for i, e := range entries {
		_, _ = fmt.Fprintf(os.Stdout, "--- Entry %d ---\n", i+1)
		_, _ = fmt.Fprintf(os.Stdout, "  Time:       %s\n", e.Timestamp.Local().Format("2006-01-02 15:04:05.000"))
		_, _ = fmt.Fprintf(os.Stdout, "  Platform:   %s\n", e.Platform)
		_, _ = fmt.Fprintf(os.Stdout, "  Event:      %s\n", e.Event)
		_, _ = fmt.Fprintf(os.Stdout, "  Tool:       %s\n", e.ToolName)
		_, _ = fmt.Fprintf(os.Stdout, "  Session:    %s\n", e.SessionID)
		_, _ = fmt.Fprintf(os.Stdout, "  Project:    %s\n", e.ProjectDir)
		_, _ = fmt.Fprintf(os.Stdout, "  Duration:   %dms\n", e.DurationMs)

		if e.Blocked {
			_, _ = fmt.Fprintf(os.Stdout, "  Blocked:    YES\n")
			_, _ = fmt.Fprintf(os.Stdout, "  Reason:     %s\n", e.Reason)
		}

		if e.HasContext {
			_, _ = fmt.Fprintf(os.Stdout, "  HasContext: YES\n")
		}

		if e.Error != "" {
			_, _ = fmt.Fprintf(os.Stdout, "  Error:      %s\n", e.Error)
		}

		if e.ToolInput != "" {
			_, _ = fmt.Fprintf(os.Stdout, "  ToolInput:  %s\n", prettyJSON(e.ToolInput))
		}

		if e.Response != "" {
			_, _ = fmt.Fprintf(os.Stdout, "  Response:   %s\n", prettyJSON(e.Response))
		}

		if e.GRPCPayload != "" {
			_, _ = fmt.Fprintf(os.Stdout, "  Payload:    %s\n", prettyJSON(e.GRPCPayload))
		}

		_, _ = fmt.Fprintln(os.Stdout)
	}
}

// prettyJSON formats a JSON string with indentation, or returns it as-is if invalid.
func prettyJSON(s string) string {
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return s
	}

	out, err := json.MarshalIndent(v, "              ", "  ")
	if err != nil {
		return s
	}

	return string(out)
}

func saveHooklogReport(entries []hooklog.Entry, allowed, blocked int) {
	events := make(map[string]int)
	tools := make(map[string]int)

	reportEntries := make([]report.HookLogEntry, 0, len(entries))

	for _, e := range entries {
		events[e.Event]++
		if e.ToolName != "" {
			tools[e.ToolName]++
		}

		reportEntries = append(reportEntries, report.HookLogEntry{
			Timestamp:  e.Timestamp,
			Platform:   e.Platform,
			Event:      e.Event,
			ToolName:   e.ToolName,
			Blocked:    e.Blocked,
			Reason:     e.Reason,
			DurationMs: e.DurationMs,
		})
	}

	id, err := report.SaveReport(&report.Report{
		Type: report.ReportHookLog,
		HookLog: &report.HookLogData{
			Total:   len(entries),
			Allowed: allowed,
			Blocked: blocked,
			Events:  events,
			Tools:   tools,
			Entries: reportEntries,
		},
	})
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "failed to save report: %v\n", err)
	} else {
		_, _ = fmt.Fprintf(os.Stderr, "Report saved: %s\n", id)
	}
}
