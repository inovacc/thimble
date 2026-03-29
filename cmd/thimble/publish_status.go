package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(newPublishStatusCmd())
}

func newPublishStatusCmd() *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:   "publish-status",
		Short: "Show GitHub Actions pipeline status",
		Long: `Show the status of recent GitHub Actions runs.

Displays CI, release, and Docker pipeline status for recent pushes and tags.

Examples:
  thimble publish-status
  thimble publish-status --limit 10`,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runPublishStatus(context.Background(), limit)
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 5, "number of recent runs to show")

	return cmd
}

func runPublishStatus(ctx context.Context, limit int) error {
	out, err := run(ctx, "gh", "run", "list", "--limit", fmt.Sprintf("%d", limit))
	if err != nil {
		return fmt.Errorf("gh run list: %w", err)
	}

	if strings.TrimSpace(out) == "" {
		_, _ = fmt.Fprintln(os.Stdout, "No recent pipeline runs found.")
		return nil
	}

	_, _ = fmt.Fprint(os.Stdout, out)

	// Summary.
	lines := strings.Split(strings.TrimSpace(out), "\n")

	var running, passed, failed int

	for _, line := range lines {
		switch {
		case strings.Contains(line, "in_progress") || strings.Contains(line, "queued"):
			running++
		case strings.Contains(line, "success"):
			passed++
		case strings.Contains(line, "failure"):
			failed++
		}
	}

	_, _ = fmt.Fprintf(os.Stdout, "\nSummary: %d passed, %d failed, %d running\n", passed, failed, running)

	return nil
}
