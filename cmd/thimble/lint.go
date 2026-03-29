package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/inovacc/thimble/internal/linter"
)

var (
	flagLintFix     bool
	flagLintFast    bool
	flagLintEnable  []string
	flagLintTimeout int
)

var lintCmd = &cobra.Command{
	Use:   "lint [paths...]",
	Short: "Run golangci-lint on the project",
	Long: `Run golangci-lint on the project. Results are structured with file, line, linter, and message.

Examples:
  thimble lint                    # lint entire project
  thimble lint ./internal/...     # lint specific path
  thimble lint --fix              # auto-fix issues
  thimble lint --fast             # only fast linters
  thimble lint --enable errcheck  # enable specific linter`,
	RunE: runLint,
}

func init() {
	lintCmd.Flags().BoolVar(&flagLintFix, "fix", false, "Auto-fix issues")
	lintCmd.Flags().BoolVar(&flagLintFast, "fast", false, "Only run fast linters")
	lintCmd.Flags().StringSliceVar(&flagLintEnable, "enable", nil, "Enable specific linters")
	lintCmd.Flags().IntVar(&flagLintTimeout, "timeout", 300, "Timeout in seconds")
	rootCmd.AddCommand(lintCmd)
}

func runLint(cmd *cobra.Command, args []string) error {
	projectDir, _ := os.Getwd()

	resp, err := linter.Run(cmd.Context(), projectDir, args, flagLintEnable, flagLintFast, flagLintFix, flagLintTimeout)
	if err != nil {
		return fmt.Errorf("lint: %w", err)
	}

	if resp.Success {
		_, _ = fmt.Fprintln(os.Stderr, "No lint issues found.")
		return nil
	}

	_, _ = fmt.Fprintf(os.Stderr, "%d issue(s) found:\n\n", resp.TotalIssues)
	for _, issue := range resp.Issues {
		_, _ = fmt.Fprintf(os.Stderr, "%s:%d:%d: %s (%s)\n",
			issue.File, issue.Line, issue.Column, issue.Message, issue.Linter)
	}

	// Return non-zero exit via os.Exit.
	os.Exit(resp.ExitCode)

	return nil
}
