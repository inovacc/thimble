package main

import (
	"database/sql"
	"fmt"
	"os"
	"runtime"
	"slices"

	"github.com/inovacc/thimble/internal/executor"
	"github.com/inovacc/thimble/internal/platform"
	"github.com/inovacc/thimble/internal/report"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Run diagnostic checks",
	RunE:  runDoctor,
}

var flagDoctorReport bool

func init() {
	doctorCmd.Flags().BoolVar(&flagDoctorReport, "report", false, "save results as an AI-consumable report")
	rootCmd.AddCommand(doctorCmd)
}

type checkResult struct {
	name    string
	status  string // pass, fail, warn
	message string
}

func (c checkResult) String() string {
	var icon string

	switch c.status {
	case "pass":
		icon = "[OK]"
	case "fail":
		icon = "[FAIL]"
	case "warn":
		icon = "[WARN]"
	default:
		icon = "[??]"
	}

	return fmt.Sprintf("  %-6s %-24s %s", icon, c.name, c.message)
}

func runDoctor(_ *cobra.Command, _ []string) error {
	_, _ = fmt.Fprintf(os.Stderr, "thimble doctor (%s/%s)\n\n", runtime.GOOS, runtime.GOARCH)

	results := make([]checkResult, 0, 8) //nolint:mnd // rough estimate of check count

	results = append(results, checkResult{
		name:    "Go version",
		status:  "pass",
		message: runtime.Version(),
	})

	results = append(results, checkSQLiteFTS5()...)
	results = append(results, checkRuntimes()...)
	results = append(results, checkServer())
	results = append(results, checkPlatform())

	var fails int

	for _, r := range results {
		_, _ = fmt.Fprintln(os.Stderr, r)
		if r.status == "fail" {
			fails++
		}
	}

	_, _ = fmt.Fprintln(os.Stderr)
	if fails > 0 {
		_, _ = fmt.Fprintf(os.Stderr, "%d check(s) failed.\n", fails)
	} else {
		_, _ = fmt.Fprintln(os.Stderr, "All checks passed.")
	}

	if flagDoctorReport {
		checks := make([]report.Check, 0, len(results))
		for _, r := range results {
			checks = append(checks, report.Check{
				Name:    r.name,
				Status:  r.status,
				Message: r.message,
			})
		}

		summary := "healthy"
		if fails > 0 {
			summary = fmt.Sprintf("%d failure(s)", fails)
		}

		id, err := report.SaveReport(&report.Report{
			Type: report.ReportDoctor,
			Doctor: &report.DoctorData{
				Summary: summary,
				Checks:  checks,
			},
		})
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "failed to save report: %v\n", err)
		} else {
			_, _ = fmt.Fprintf(os.Stderr, "Report saved: %s\n", id)
		}
	}

	return nil
}

func checkSQLiteFTS5() []checkResult {
	var results []checkResult

	// The sqlite driver is registered by the server process (which imports modernc.org/sqlite).
	// Here we check if it's available via database/sql drivers.
	drivers := sql.Drivers()
	hasDriver := slices.Contains(drivers, "sqlite")

	if !hasDriver {
		results = append(results, checkResult{"SQLite", "warn", "driver not loaded (OK — server handles DB)"})
		results = append(results, checkResult{"FTS5", "warn", "cannot test without driver (OK — server handles DB)"})

		return results
	}

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		results = append(results, checkResult{"SQLite", "fail", fmt.Sprintf("cannot open: %v", err)})
		return results
	}

	defer func() { _ = db.Close() }()

	var sqliteVer string
	if err := db.QueryRow("SELECT sqlite_version()").Scan(&sqliteVer); err != nil {
		results = append(results, checkResult{"SQLite", "fail", fmt.Sprintf("version query: %v", err)})
	} else {
		results = append(results, checkResult{"SQLite", "pass", "v" + sqliteVer + " (modernc.org/sqlite)"})
	}

	_, err = db.Exec("CREATE VIRTUAL TABLE IF NOT EXISTS _fts5_test USING fts5(content)")
	if err != nil {
		results = append(results, checkResult{"FTS5", "fail", fmt.Sprintf("not available: %v", err)})
	} else {
		_, _ = db.Exec("DROP TABLE IF EXISTS _fts5_test")

		results = append(results, checkResult{"FTS5", "pass", "available"})
	}

	return results
}

func checkRuntimes() []checkResult {
	runtimes := executor.DetectRuntimes()
	available := executor.GetAvailableLanguages(runtimes)

	var results []checkResult

	keyRuntimes := []struct {
		lang    string
		display string
	}{
		{"python", "Python"},
		{"node", "Node.js"},
		{"bash", "Bash"},
		{"go", "Go"},
		{"rust", "Rust"},
		{"ruby", "Ruby"},
		{"php", "PHP"},
		{"perl", "Perl"},
		{"swift", "Swift"},
		{"r", "R"},
		{"powershell", "PowerShell"},
	}

	availSet := make(map[string]bool)
	for _, l := range available {
		availSet[l] = true
	}

	var found []string

	for _, rt := range keyRuntimes {
		if availSet[rt.lang] {
			path := runtimes[rt.lang]
			results = append(results, checkResult{rt.display, "pass", path})
			found = append(found, rt.lang)
		} else {
			results = append(results, checkResult{rt.display, "warn", "not found"})
		}
	}

	results = append(results, checkResult{
		name:    "Runtimes",
		status:  "pass",
		message: fmt.Sprintf("%d/%d available", len(found), len(keyRuntimes)),
	})

	return results
}

func checkServer() checkResult {
	return checkResult{"Architecture", "pass", "single-binary (no daemon)"}
}

func checkPlatform() checkResult {
	signal := platform.Detect()

	return checkResult{
		name:    "Platform",
		status:  "pass",
		message: fmt.Sprintf("%s (%s confidence: %s)", signal.Platform, signal.Confidence, signal.Reason),
	}
}
