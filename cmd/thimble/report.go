package main

import (
	"fmt"
	"text/tabwriter"

	"github.com/inovacc/thimble/internal/report"
	"github.com/spf13/cobra"
)

var reportCmd = &cobra.Command{
	Use:   "report",
	Short: "Manage saved diagnostic reports",
}

var reportListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all saved reports",
	RunE: func(cmd *cobra.Command, _ []string) error {
		reports, err := report.ListReports()
		if err != nil {
			return err
		}

		if len(reports) == 0 {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No reports found.")
			return nil
		}

		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
		_, _ = fmt.Fprintln(w, "ID\tTYPE\tISSUES\tCREATED")

		for _, r := range reports {
			_, _ = fmt.Fprintf(w, "%s\t%s\t%d\t%s\n",
				r.ID, r.Type, r.IssueCount(),
				r.CreatedAt.Format("2006-01-02 15:04:05"))
		}

		return w.Flush()
	},
}

var reportShowCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Show a saved report (full AI-consumable text)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		content, err := report.ReadReportRaw(args[0])
		if err != nil {
			return err
		}

		_, _ = fmt.Fprint(cmd.OutOrStdout(), content)

		return nil
	},
}

var reportDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a saved report",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := report.DeleteReport(args[0]); err != nil {
			return err
		}

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Deleted report: %s\n", args[0])

		return nil
	},
}

func init() {
	reportCmd.AddCommand(reportListCmd, reportShowCmd, reportDeleteCmd)
	rootCmd.AddCommand(reportCmd)
}
