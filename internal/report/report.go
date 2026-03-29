// Package report provides persistent, AI-consumable reports for thimble operations.
// Reports are stored as structured markdown files in the data directory.
package report

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/inovacc/thimble/internal/paths"
)

// ReportType identifies the kind of report.
type ReportType string

const (
	ReportDoctor  ReportType = "doctor"
	ReportCrash   ReportType = "crash"
	ReportStats   ReportType = "stats"
	ReportHookLog ReportType = "hooklog"
)

// Report wraps a doctor, crash, stats, or hooklog result with metadata.
type Report struct {
	ID        string       `json:"id"`
	Type      ReportType   `json:"type"`
	CreatedAt time.Time    `json:"created_at"`
	Doctor    *DoctorData  `json:"doctor,omitempty"`
	Crash     *CrashData   `json:"crash,omitempty"`
	Stats     *StatsData   `json:"stats,omitempty"`
	HookLog   *HookLogData `json:"hooklog,omitempty"`
}

// DoctorData holds health check results.
type DoctorData struct {
	Checks  []Check `json:"checks"`
	Summary string  `json:"summary"`
}

// Check is a single diagnostic result.
type Check struct {
	Name    string `json:"name"`
	Status  string `json:"status"` // pass, fail, warn
	Message string `json:"message"`
}

// CrashData holds crash history.
type CrashData struct {
	Entries []CrashEntry `json:"entries"`
	Total   int          `json:"total"`
}

// CrashEntry is a single crash log entry.
type CrashEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Error     string    `json:"error"`
	UptimeMs  int64     `json:"uptime_ms"`
	CrashNum  int       `json:"crash_num"`
	BackoffMs int64     `json:"backoff_ms"`
}

// StatsData holds knowledge base and session statistics.
type StatsData struct {
	Sources    int            `json:"sources"`
	Chunks     int            `json:"chunks"`
	CodeChunks int            `json:"code_chunks"`
	Session    map[string]any `json:"session,omitempty"`
}

// HookLogData holds hook interaction log summary.
type HookLogData struct {
	Total   int            `json:"total"`
	Allowed int            `json:"allowed"`
	Blocked int            `json:"blocked"`
	Events  map[string]int `json:"events"`
	Tools   map[string]int `json:"tools"`
	Entries []HookLogEntry `json:"entries"`
}

// HookLogEntry is a single hook interaction for report rendering.
type HookLogEntry struct {
	Timestamp  time.Time `json:"timestamp"`
	Platform   string    `json:"platform"`
	Event      string    `json:"event"`
	ToolName   string    `json:"tool_name,omitempty"`
	Blocked    bool      `json:"blocked,omitempty"`
	Reason     string    `json:"reason,omitempty"`
	DurationMs int64     `json:"duration_ms"`
}

// ReportsDir is the function that returns the base directory for reports.
// It is a variable so tests can override it.
var ReportsDir = defaultReportsDir

func defaultReportsDir() string {
	return filepath.Join(paths.DataDir(), "reports")
}

// SaveReport persists a report as a structured, AI-consumable markdown document.
// Returns the report ID.
func SaveReport(r *Report) (string, error) {
	if r.ID == "" {
		id, err := uuid.NewV7()
		if err != nil {
			return "", fmt.Errorf("thimble: report: generate id: %w", err)
		}

		r.ID = id.String()
	}

	if r.CreatedAt.IsZero() {
		r.CreatedAt = time.Now()
	}

	dir := ReportsDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("thimble: report: create dir: %w", err)
	}

	content := renderReport(r)

	path := filepath.Join(dir, r.ID+".txt")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("thimble: report: write: %w", err)
	}

	return r.ID, nil
}

// renderReport produces a structured text document designed for AI consumption.
func renderReport(r *Report) string {
	var b strings.Builder

	b.WriteString("# Thimble Report\n\n")
	b.WriteString("## Metadata\n\n")
	fmt.Fprintf(&b, "- **Report ID:** %s\n", r.ID)
	fmt.Fprintf(&b, "- **Type:** %s\n", r.Type)
	fmt.Fprintf(&b, "- **Generated:** %s\n", r.CreatedAt.Format(time.RFC3339))
	fmt.Fprintf(&b, "- **Tool:** Thimble (MCP plugin)\n")
	b.WriteString("\n---\n\n")

	switch {
	case r.Doctor != nil:
		renderDoctorReport(&b, r)
	case r.Crash != nil:
		renderCrashReport(&b, r)
	case r.Stats != nil:
		renderStatsReport(&b, r)
	case r.HookLog != nil:
		renderHookLogReport(&b, r)
	}

	// Append raw JSON for machine parsing.
	b.WriteString("## Raw Data (JSON)\n\n")
	b.WriteString("```json\n")

	raw, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		b.WriteString("{\"error\": \"failed to marshal report\"}")
		b.WriteString("\n```\n")

		return b.String()
	}

	b.Write(raw)
	b.WriteString("\n```\n")

	return b.String()
}

func renderDoctorReport(b *strings.Builder, r *Report) {
	d := r.Doctor

	var passes, fails, warns int

	for _, c := range d.Checks {
		switch c.Status {
		case "pass":
			passes++
		case "fail":
			fails++
		case "warn":
			warns++
		}
	}

	b.WriteString("## Summary\n\n")
	fmt.Fprintf(b, "- **Total checks:** %d\n", len(d.Checks))
	fmt.Fprintf(b, "- **Passed:** %d\n", passes)
	fmt.Fprintf(b, "- **Failed:** %d\n", fails)
	fmt.Fprintf(b, "- **Warnings:** %d\n", warns)

	if d.Summary != "" {
		fmt.Fprintf(b, "- **Status:** %s\n", d.Summary)
	}

	b.WriteString("\n---\n\n")

	if len(d.Checks) > 0 {
		b.WriteString("## Check Results\n\n")

		// Group by status.
		for _, status := range []string{"fail", "warn", "pass"} {
			var group []Check

			for _, c := range d.Checks {
				if c.Status == status {
					group = append(group, c)
				}
			}

			if len(group) == 0 {
				continue
			}

			fmt.Fprintf(b, "### %s (%d)\n\n", strings.ToUpper(status), len(group))

			for i, c := range group {
				fmt.Fprintf(b, "%d. **%s** — %s\n", i+1, c.Name, c.Message)
			}

			b.WriteString("\n")
		}

		b.WriteString("---\n\n")
	}

	b.WriteString("## Instructions\n\n")
	b.WriteString("You are reviewing a thimble health check report. Analyze the results and provide:\n\n")
	b.WriteString("1. **Issue Summary** — For each failure/warning, explain the likely cause.\n")
	b.WriteString("2. **Priority Ranking** — Rank issues by impact (critical, high, medium, low).\n")
	b.WriteString("3. **Recommended Fixes** — Specific, actionable steps to resolve each issue.\n")
	b.WriteString("4. **Environment Gaps** — Missing runtimes or tools that limit functionality.\n")
	b.WriteString("5. **Monitoring Suggestions** — What to watch for to prevent recurrence.\n\n")

	fmt.Fprintf(b, "Total checks: %d (pass: %d, fail: %d, warn: %d)\n\n", len(d.Checks), passes, fails, warns)

	if fails == 0 && warns == 0 {
		b.WriteString("All checks passed. Confirm thimble is healthy and suggest proactive improvements.\n\n")
	}
}

func renderCrashReport(b *strings.Builder, r *Report) {
	c := r.Crash

	b.WriteString("## Summary\n\n")
	fmt.Fprintf(b, "- **Total crashes:** %d\n", c.Total)
	fmt.Fprintf(b, "- **Entries shown:** %d\n", len(c.Entries))
	b.WriteString("\n---\n\n")

	if len(c.Entries) > 0 {
		b.WriteString("## Crash History\n\n")

		for i, e := range c.Entries {
			fmt.Fprintf(b, "%d. **%s**\n", i+1, e.Timestamp.Format(time.RFC3339))
			fmt.Fprintf(b, "   - Error: %s\n", e.Error)
			fmt.Fprintf(b, "   - Uptime: %dms\n", e.UptimeMs)
			fmt.Fprintf(b, "   - Crash #: %d\n", e.CrashNum)
			fmt.Fprintf(b, "   - Backoff: %dms\n\n", e.BackoffMs)
		}

		b.WriteString("---\n\n")
	}

	b.WriteString("## Instructions\n\n")
	b.WriteString("You are reviewing a thimble crash report. Analyze the crash history and provide:\n\n")
	b.WriteString("1. **Root Cause Analysis** — For each crash, identify the likely cause from the error message.\n")
	b.WriteString("2. **Pattern Detection** — Identify recurring crashes, escalating frequency, or degradation trends.\n")
	b.WriteString("3. **Stability Assessment** — Evaluate overall stability (uptime trends, backoff patterns).\n")
	b.WriteString("4. **Recommended Fixes** — Specific steps to prevent each crash type.\n")
	b.WriteString("5. **Monitoring Suggestions** — Metrics to track for early warning of instability.\n\n")

	fmt.Fprintf(b, "Total crashes recorded: %d\n\n", c.Total)

	if c.Total == 0 {
		b.WriteString("No crashes recorded. Thimble appears stable. Suggest proactive monitoring.\n\n")
	}
}

func renderStatsReport(b *strings.Builder, r *Report) {
	s := r.Stats

	b.WriteString("## Summary\n\n")
	fmt.Fprintf(b, "- **Sources:** %d\n", s.Sources)
	fmt.Fprintf(b, "- **Chunks:** %d\n", s.Chunks)
	fmt.Fprintf(b, "- **Code chunks:** %d\n", s.CodeChunks)
	b.WriteString("\n---\n\n")

	b.WriteString("## Instructions\n\n")
	b.WriteString("You are reviewing a thimble knowledge base statistics report. Analyze the data and provide:\n\n")
	b.WriteString("1. **Usage Analysis** — Assess knowledge base utilization and growth.\n")
	b.WriteString("2. **Content Quality** — Evaluate the ratio of code vs prose content.\n")
	b.WriteString("3. **Optimization Suggestions** — Recommend cleanup, re-indexing, or restructuring.\n")
	b.WriteString("4. **Capacity Planning** — Project growth and recommend maintenance schedules.\n\n")

	fmt.Fprintf(b, "Sources: %d, Chunks: %d, Code chunks: %d\n\n", s.Sources, s.Chunks, s.CodeChunks)
}

func renderHookLogReport(b *strings.Builder, r *Report) {
	h := r.HookLog

	b.WriteString("## Summary\n\n")
	fmt.Fprintf(b, "- **Total interactions:** %d\n", h.Total)
	fmt.Fprintf(b, "- **Allowed:** %d\n", h.Allowed)
	fmt.Fprintf(b, "- **Blocked:** %d\n", h.Blocked)
	b.WriteString("\n")

	if len(h.Events) > 0 {
		b.WriteString("## Events by Type\n\n")

		for event, count := range h.Events {
			fmt.Fprintf(b, "- %s: %d\n", event, count)
		}

		b.WriteString("\n")
	}

	if len(h.Tools) > 0 {
		b.WriteString("## Tools by Frequency\n\n")

		for tool, count := range h.Tools {
			fmt.Fprintf(b, "- %s: %d\n", tool, count)
		}

		b.WriteString("\n")
	}

	// Show blocked entries.
	var blockedEntries []HookLogEntry

	for _, e := range h.Entries {
		if e.Blocked {
			blockedEntries = append(blockedEntries, e)
		}
	}

	if len(blockedEntries) > 0 {
		b.WriteString("## Blocked Interactions\n\n")

		for i, e := range blockedEntries {
			fmt.Fprintf(b, "%d. **%s** %s `%s`: %s\n",
				i+1, e.Timestamp.Format("15:04:05"), e.Event, e.ToolName, e.Reason)
		}

		b.WriteString("\n")
	}

	b.WriteString("---\n\n")
	b.WriteString("## Instructions\n\n")
	b.WriteString("You are reviewing a thimble hook interaction log. Analyze the data and provide:\n\n")
	b.WriteString("1. **Security Assessment** — Review blocked interactions for patterns of concern.\n")
	b.WriteString("2. **Performance Analysis** — Identify slow hooks or bottlenecks.\n")
	b.WriteString("3. **Usage Patterns** — Summarize which tools and events are most frequent.\n")
	b.WriteString("4. **Recommendations** — Suggest policy adjustments or optimizations.\n\n")
}

// ReadReport reads a report by ID.
func ReadReport(id string) (*Report, error) {
	path := filepath.Join(ReportsDir(), id+".txt")

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("thimble: report: read %s: %w", id, err)
	}

	content := string(data)

	// Extract JSON from ```json ... ``` code block.
	if start := strings.Index(content, "```json\n"); start >= 0 {
		start += len("```json\n")
		if end := strings.Index(content[start:], "\n```"); end >= 0 {
			data = []byte(content[start : start+end])
		}
	}

	var r Report
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("thimble: report: parse %s: %w", id, err)
	}

	return &r, nil
}

// ReadReportRaw returns the full text content of a report file.
func ReadReportRaw(id string) (string, error) {
	path := filepath.Join(ReportsDir(), id+".txt")

	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("thimble: report: read %s: %w", id, err)
	}

	return string(data), nil
}

// ListReports returns all reports sorted by creation time (newest first).
func ListReports() ([]Report, error) {
	dir := ReportsDir()

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("thimble: report: list: %w", err)
	}

	var reports []Report

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".txt") {
			continue
		}

		id := strings.TrimSuffix(e.Name(), ".txt")

		r, err := ReadReport(id)
		if err != nil {
			continue
		}

		reports = append(reports, *r)
	}

	sort.Slice(reports, func(i, j int) bool {
		return reports[i].CreatedAt.After(reports[j].CreatedAt)
	})

	return reports, nil
}

// DeleteReport removes a report file by ID.
func DeleteReport(id string) error {
	path := filepath.Join(ReportsDir(), id+".txt")
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("thimble: report: delete %s: %w", id, err)
	}

	return nil
}

// IssueCount returns the number of issues in a report.
func (r *Report) IssueCount() int {
	switch {
	case r.Doctor != nil:
		count := 0

		for _, c := range r.Doctor.Checks {
			if c.Status == "fail" || c.Status == "warn" {
				count++
			}
		}

		return count
	case r.Crash != nil:
		return r.Crash.Total
	case r.HookLog != nil:
		return r.HookLog.Blocked
	default:
		return 0
	}
}
