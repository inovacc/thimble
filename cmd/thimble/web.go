package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/inovacc/thimble/internal/paths"
	"github.com/inovacc/thimble/internal/web"
	"github.com/spf13/cobra"
)

var webCmd = &cobra.Command{
	Use:   "web",
	Short: "Start the session insights web dashboard",
	RunE:  runWeb,
}

var flagWebPort int

// webProjectDir is the function used to resolve the project directory.
// Tests can override this to avoid filesystem dependencies.
var webProjectDir = func() string {
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}

	return dir
}

func init() {
	webCmd.Flags().IntVar(&flagWebPort, "port", 8080, "HTTP port to listen on")
	rootCmd.AddCommand(webCmd)
}

func runWeb(cmd *cobra.Command, _ []string) error {
	projectDir := webProjectDir()
	dataDir := paths.ProjectDataDir(projectDir)
	dbPath := dataDir + "/session.db"

	// Check if session DB exists.
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "No session data found for this project.")
		return nil
	}

	srv := web.New(dbPath, flagWebPort)

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "thimble web dashboard: http://localhost:%d\n", flagWebPort)
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Press Ctrl+C to stop.")

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	errCh := make(chan error, 1)

	go func() {
		errCh <- srv.Start()
	}()

	select {
	case <-ctx.Done():
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "\nShutting down.")
		return nil
	case err := <-errCh:
		return fmt.Errorf("web server: %w", err)
	}
}
