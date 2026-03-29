package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"text/tabwriter"
	"time"

	"github.com/inovacc/thimble/internal/model"
	"github.com/inovacc/thimble/internal/paths"
	"github.com/inovacc/thimble/internal/report"
	"github.com/inovacc/thimble/internal/session"
	"github.com/spf13/cobra"
)

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show session analytics for the current project",
	RunE:  runStats,
}

var (
	flagStatsJSON   bool
	flagStatsReport bool
)

// statsProjectDir is the function used to resolve the project directory.
// Tests can override this to avoid filesystem dependencies.
var statsProjectDir = func() string {
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}

	return dir
}

func init() {
	statsCmd.Flags().BoolVar(&flagStatsJSON, "json", false, "output as JSON")
	statsCmd.Flags().BoolVar(&flagStatsReport, "report", false, "save results as an AI-consumable report")
	rootCmd.AddCommand(statsCmd)
}

// statsResult holds aggregated analytics for rendering.
type statsResult struct {
	SessionID    string           `json:"session_id"`
	Duration     time.Duration    `json:"duration_ns"`
	DurationText string           `json:"duration"`
	TotalEvents  int              `json:"total_events"`
	EventsByType map[string]int   `json:"events_by_type"`
	TopTools     []model.ToolCount `json:"top_tools"`
	ErrorCount   int              `json:"error_count"`
	ErrorRate    float64          `json:"error_rate"`
}

func runStats(cmd *cobra.Command, _ []string) error {
	projectDir := statsProjectDir()
	dataDir := paths.ProjectDataDir(projectDir)
	dbPath := dataDir + "/session.db"

	// Check if session DB exists.
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "No session data found for this project.")
		return nil
	}

	sdb, err := session.New(dbPath)
	if err != nil {
		return fmt.Errorf("open session db: %w", err)
	}
	defer sdb.Close()

	// Find the most recent session.
	sessionIDs, err := sdb.ListSessionIDs()
	if err != nil {
		return fmt.Errorf("list sessions: %w", err)
	}

	if len(sessionIDs) == 0 {
		_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "No sessions found.")
		return nil
	}

	sid := sessionIDs[0]

	result, err := gatherStats(sdb, sid)
	if err != nil {
		return err
	}

	if flagStatsJSON {
		return printStatsJSON(cmd, result)
	}

	printStatsTable(cmd, result)

	if flagStatsReport {
		return saveStatsReport(cmd, result)
	}

	return nil
}

func gatherStats(sdb *session.SessionDB, sessionID string) (*statsResult, error) {
	duration, err := sdb.SessionDuration(sessionID)
	if err != nil {
		return nil, fmt.Errorf("session duration: %w", err)
	}

	eventsByType, err := sdb.EventsByType(sessionID)
	if err != nil {
		return nil, fmt.Errorf("events by type: %w", err)
	}

	topTools, err := sdb.TopTools(sessionID, 10)
	if err != nil {
		return nil, fmt.Errorf("top tools: %w", err)
	}

	totalEvents, err := sdb.GetEventCount(sessionID)
	if err != nil {
		return nil, fmt.Errorf("event count: %w", err)
	}

	errorCount, err := sdb.ErrorCount(sessionID)
	if err != nil {
		return nil, fmt.Errorf("error count: %w", err)
	}

	var errorRate float64
	if totalEvents > 0 {
		errorRate = float64(errorCount) / float64(totalEvents) * 100
	}

	return &statsResult{
		SessionID:    sessionID,
		Duration:     duration,
		DurationText: formatDuration(duration),
		TotalEvents:  totalEvents,
		EventsByType: eventsByType,
		TopTools:     topTools,
		ErrorCount:   errorCount,
		ErrorRate:    errorRate,
	}, nil
}

func printStatsJSON(cmd *cobra.Command, result *statsResult) error {
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")

	return enc.Encode(result)
}

func printStatsTable(cmd *cobra.Command, result *statsResult) {
	out := cmd.OutOrStdout()

	_, _ = fmt.Fprintf(out, "Session: %s\n", result.SessionID)
	_, _ = fmt.Fprintf(out, "Duration: %s\n", result.DurationText)
	_, _ = fmt.Fprintf(out, "Total events: %d\n", result.TotalEvents)
	_, _ = fmt.Fprintf(out, "Errors: %d (%.1f%%)\n\n", result.ErrorCount, result.ErrorRate)

	// Events by type table.
	if len(result.EventsByType) > 0 {
		_, _ = fmt.Fprintln(out, "Events by Type:")

		w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
		_, _ = fmt.Fprintln(w, "  TYPE\tCOUNT")

		// Sort by count descending.
		type kv struct {
			k string
			v int
		}

		sorted := make([]kv, 0, len(result.EventsByType))
		for k, v := range result.EventsByType {
			sorted = append(sorted, kv{k, v})
		}

		sort.Slice(sorted, func(i, j int) bool { return sorted[i].v > sorted[j].v })

		for _, kv := range sorted {
			_, _ = fmt.Fprintf(w, "  %s\t%d\n", kv.k, kv.v)
		}

		_ = w.Flush()
		_, _ = fmt.Fprintln(out)
	}

	// Top tools table.
	if len(result.TopTools) > 0 {
		_, _ = fmt.Fprintln(out, "Top 10 Tools:")

		w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
		_, _ = fmt.Fprintln(w, "  TOOL\tCOUNT")

		for _, tc := range result.TopTools {
			_, _ = fmt.Fprintf(w, "  %s\t%d\n", tc.Name, tc.Count)
		}

		_ = w.Flush()
		_, _ = fmt.Fprintln(out)
	}
}

func saveStatsReport(cmd *cobra.Command, result *statsResult) error {
	sessionData := map[string]any{
		"session_id":     result.SessionID,
		"duration":       result.DurationText,
		"total_events":   result.TotalEvents,
		"events_by_type": result.EventsByType,
		"top_tools":      result.TopTools,
		"error_count":    result.ErrorCount,
		"error_rate":     result.ErrorRate,
	}

	id, err := report.SaveReport(&report.Report{
		Type: report.ReportStats,
		Stats: &report.StatsData{
			Session: sessionData,
		},
	})
	if err != nil {
		return fmt.Errorf("save report: %w", err)
	}

	_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Report saved: %s\n", id)

	return nil
}

func formatDuration(d time.Duration) string {
	if d == 0 {
		return "0s"
	}

	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60

	switch {
	case h > 0:
		return fmt.Sprintf("%dh%dm%ds", h, m, s)
	case m > 0:
		return fmt.Sprintf("%dm%ds", m, s)
	default:
		return fmt.Sprintf("%ds", s)
	}
}
