package main

import (
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/inovacc/thimble/internal/paths"
	"github.com/inovacc/thimble/internal/session"
	"github.com/spf13/cobra"
)

var sessionCmd = &cobra.Command{
	Use:   "session",
	Short: "Manage session data (export, import, list)",
}

var sessionExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export current project session to JSON",
	Long: `Export all events, metadata, and resume snapshot for the current
project's session as a JSON file.

Defaults to stdout; use --output to write to a file.`,
	RunE: runSessionExport,
}

var sessionImportCmd = &cobra.Command{
	Use:   "import <file.json>",
	Short: "Import a session from a JSON file",
	Long:  `Import session events, metadata, and resume snapshot from a previously exported JSON file.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runSessionImport,
}

var sessionListCmd = &cobra.Command{
	Use:   "list",
	Short: "List sessions with event counts and last activity",
	RunE:  runSessionList,
}

var (
	flagSessionOutput     string
	flagSessionProjectDir string
	flagSessionID         string
)

func init() {
	sessionExportCmd.Flags().StringVarP(&flagSessionOutput, "output", "o", "", "output file (default: stdout)")
	sessionExportCmd.Flags().StringVar(&flagSessionProjectDir, "project-dir", "", "project directory (default: current directory)")
	sessionExportCmd.Flags().StringVar(&flagSessionID, "session-id", "", "session ID to export (default: auto-detect from project)")

	sessionImportCmd.Flags().StringVar(&flagSessionProjectDir, "project-dir", "", "project directory for import target (default: current directory)")

	sessionListCmd.Flags().StringVar(&flagSessionProjectDir, "project-dir", "", "project directory (default: current directory)")

	sessionCmd.AddCommand(sessionExportCmd)
	sessionCmd.AddCommand(sessionImportCmd)
	sessionCmd.AddCommand(sessionListCmd)
	rootCmd.AddCommand(sessionCmd)
}

func resolveSessionDB(projectDir string) (*session.SessionDB, error) {
	if projectDir == "" {
		var err error

		projectDir, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("get working directory: %w", err)
		}
	}

	dir := paths.ProjectDataDir(projectDir)
	dbPath := filepath.Join(dir, "session.db")

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("no session database found at %s", dbPath)
	}

	return session.New(dbPath)
}

func resolveSessionDBForImport(projectDir string) (*session.SessionDB, error) {
	if projectDir == "" {
		var err error

		projectDir, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("get working directory: %w", err)
		}
	}

	dir := paths.ProjectDataDir(projectDir)

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create session dir: %w", err)
	}

	return session.New(filepath.Join(dir, "session.db"))
}

func runSessionExport(_ *cobra.Command, _ []string) error {
	sdb, err := resolveSessionDB(flagSessionProjectDir)
	if err != nil {
		return err
	}

	defer sdb.Close()

	sessionID := flagSessionID
	if sessionID == "" {
		// Auto-detect: pick the most recent session.
		sessions, err := sdb.ListSessions()
		if err != nil {
			return fmt.Errorf("list sessions: %w", err)
		}

		if len(sessions) == 0 {
			return fmt.Errorf("no sessions found")
		}

		sessionID = sessions[0].SessionID
		_, _ = fmt.Fprintf(os.Stderr, "Exporting session: %s\n", sessionID)
	}

	exported, err := sdb.ExportSession(sessionID)
	if err != nil {
		return fmt.Errorf("export session: %w", err)
	}

	jsonBytes, err := session.ExportJSON(exported)
	if err != nil {
		return fmt.Errorf("marshal export: %w", err)
	}

	if flagSessionOutput == "" {
		_, err = os.Stdout.Write(jsonBytes)
		if err != nil {
			return fmt.Errorf("write stdout: %w", err)
		}

		_, _ = fmt.Fprintln(os.Stdout)

		return nil
	}

	if err := os.WriteFile(flagSessionOutput, jsonBytes, 0o644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	_, _ = fmt.Fprintf(os.Stderr, "Session exported to %s (%d events)\n", flagSessionOutput, len(exported.Events))

	return nil
}

func runSessionImport(_ *cobra.Command, args []string) error {
	data, err := os.ReadFile(args[0])
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	exported, err := session.ImportJSON(data)
	if err != nil {
		return fmt.Errorf("parse export file: %w", err)
	}

	sdb, err := resolveSessionDBForImport(flagSessionProjectDir)
	if err != nil {
		return err
	}

	defer sdb.Close()

	if err := sdb.ImportSession(exported); err != nil {
		return fmt.Errorf("import session: %w", err)
	}

	_, _ = fmt.Fprintf(os.Stdout, "Imported session %s (%d events)\n", exported.SessionID, len(exported.Events))

	return nil
}

func runSessionList(_ *cobra.Command, _ []string) error {
	sdb, err := resolveSessionDB(flagSessionProjectDir)
	if err != nil {
		return err
	}

	defer sdb.Close()

	sessions, err := sdb.ListSessions()
	if err != nil {
		return fmt.Errorf("list sessions: %w", err)
	}

	if len(sessions) == 0 {
		_, _ = fmt.Fprintln(os.Stdout, "No sessions found.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "SESSION ID\tPROJECT\tEVENTS\tSTARTED\tLAST ACTIVITY")

	for _, s := range sessions {
		lastActivity := s.LastEventAt
		if lastActivity == "" {
			lastActivity = "-"
		}

		_, _ = fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\n",
			s.SessionID, s.ProjectDir, s.EventCount, s.StartedAt, lastActivity)
	}

	_ = w.Flush()

	return nil
}
