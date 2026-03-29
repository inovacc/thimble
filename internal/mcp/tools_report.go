package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/inovacc/thimble/internal/report"
)

// ── Report Input Types ──

type reportListInput struct {
	TypeFilter string `json:"type_filter,omitempty" jsonschema:"filter by type: doctor crash stats (empty = all)"`
}

type reportShowInput struct {
	ID string `json:"id" jsonschema:"Report ID (UUID)"`
}

type reportDeleteInput struct {
	ID string `json:"id" jsonschema:"Report ID (UUID) to delete"`
}

// registerReportTools adds report management tools to the MCP server.
func (b *Bridge) registerReportTools() {
	mcpsdk.AddTool(b.server, &mcpsdk.Tool{
		Name:        "ctx_report_list",
		Description: "List all saved diagnostic reports with ID, type, issues count, and creation date.",
	}, b.handleReportList)

	mcpsdk.AddTool(b.server, &mcpsdk.Tool{
		Name:        "ctx_report_show",
		Description: "Show the full AI-consumable text of a saved report by ID.",
	}, b.handleReportShow)

	mcpsdk.AddTool(b.server, &mcpsdk.Tool{
		Name:        "ctx_report_delete",
		Description: "Delete a saved report by ID.",
	}, b.handleReportDelete)
}

func (b *Bridge) handleReportList(_ context.Context, _ *mcpsdk.CallToolRequest, input reportListInput) (*mcpsdk.CallToolResult, struct{}, error) {
	reports, err := report.ListReports()
	if err != nil {
		return errorResult(fmt.Sprintf("list reports: %s", err)), struct{}{}, nil
	}

	// Apply type filter.
	if input.TypeFilter != "" {
		filtered := reports[:0]
		for _, r := range reports {
			if string(r.Type) == input.TypeFilter {
				filtered = append(filtered, r)
			}
		}

		reports = filtered
	}

	if len(reports) == 0 {
		return textResult("No reports found."), struct{}{}, nil
	}

	type summary struct {
		ID        string `json:"id"`
		Type      string `json:"type"`
		Issues    int    `json:"issues"`
		CreatedAt string `json:"created_at"`
	}

	summaries := make([]summary, 0, len(reports))
	for _, r := range reports {
		summaries = append(summaries, summary{
			ID:        r.ID,
			Type:      string(r.Type),
			Issues:    r.IssueCount(),
			CreatedAt: r.CreatedAt.Format(time.RFC3339),
		})
	}

	data, err := json.MarshalIndent(summaries, "", "  ")
	if err != nil {
		return errorResult("marshal failed: " + err.Error()), struct{}{}, nil //nolint:nilerr // MCP tools surface errors as results
	}

	return textResult(string(data)), struct{}{}, nil
}

func (b *Bridge) handleReportShow(_ context.Context, _ *mcpsdk.CallToolRequest, input reportShowInput) (*mcpsdk.CallToolResult, struct{}, error) {
	if input.ID == "" {
		return errorResult("id is required"), struct{}{}, nil
	}

	content, err := report.ReadReportRaw(input.ID)
	if err != nil {
		return errorResult(fmt.Sprintf("read report: %s", err)), struct{}{}, nil
	}

	return textResult(content), struct{}{}, nil
}

func (b *Bridge) handleReportDelete(_ context.Context, _ *mcpsdk.CallToolRequest, input reportDeleteInput) (*mcpsdk.CallToolResult, struct{}, error) {
	if input.ID == "" {
		return errorResult("id is required"), struct{}{}, nil
	}

	if err := report.DeleteReport(input.ID); err != nil {
		return errorResult(fmt.Sprintf("delete report: %s", err)), struct{}{}, nil
	}

	return textResult(fmt.Sprintf("Report %s deleted.", input.ID)), struct{}{}, nil
}
