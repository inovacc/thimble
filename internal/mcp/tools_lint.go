package mcp

import (
	"context"
	"fmt"
	"strings"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/inovacc/thimble/internal/linter"
)

type lintRunInput struct {
	ProjectDir string   `json:"project_dir,omitempty" jsonschema:"project directory to lint (defaults to current)"`
	Paths      []string `json:"paths,omitempty" jsonschema:"specific files or directories to lint (default: ./...)"`
	Linters    []string `json:"linters,omitempty" jsonschema:"enable specific linters (e.g. [errcheck govet])"`
	Fast       bool     `json:"fast,omitempty" jsonschema:"only run fast linters"`
	TimeoutSec int32    `json:"timeout_seconds,omitempty" jsonschema:"timeout in seconds (default 300)"`
}

type lintFixInput struct {
	ProjectDir string   `json:"project_dir,omitempty" jsonschema:"project directory to lint (defaults to current)"`
	Paths      []string `json:"paths,omitempty" jsonschema:"specific files or directories to fix"`
	Linters    []string `json:"linters,omitempty" jsonschema:"enable specific linters"`
}

func (b *Bridge) registerLintTools() {
	mcpsdk.AddTool(b.server, &mcpsdk.Tool{
		Name:        "ctx_lint",
		Description: "Run golangci-lint on the project. Returns structured issues with file, line, linter, and message. Results are auto-indexed.",
	}, b.handleLintRun)

	mcpsdk.AddTool(b.server, &mcpsdk.Tool{
		Name:        "ctx_lint_fix",
		Description: "Run golangci-lint with --fix to auto-fix issues. Returns what was fixed and remaining issues.",
	}, b.handleLintFix)
}

func (b *Bridge) handleLintRun(ctx context.Context, _ *mcpsdk.CallToolRequest, input lintRunInput) (*mcpsdk.CallToolResult, struct{}, error) {
	ctx, finish := spanTool(ctx, "ctx_lint")
	defer finish(nil)

	b.recordToolCall(ctx, "ctx_lint", true)

	dir := input.ProjectDir
	if dir == "" {
		dir = b.projectDir
	}

	result, err := linter.Run(ctx, dir, input.Paths, input.Linters, input.Fast, false, int(input.TimeoutSec))
	if err != nil {
		return errResult(fmt.Sprintf("lint failed: %v", err)), struct{}{}, nil
	}

	output := formatLintOutput(result)
	b.autoIndex(ctx, output, "lint-results")

	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: output}},
		IsError: !result.Success,
	}, struct{}{}, nil
}

func (b *Bridge) handleLintFix(ctx context.Context, _ *mcpsdk.CallToolRequest, input lintFixInput) (*mcpsdk.CallToolResult, struct{}, error) {
	ctx, finish := spanTool(ctx, "ctx_lint_fix")
	defer finish(nil)

	b.recordToolCall(ctx, "ctx_lint_fix", false)

	dir := input.ProjectDir
	if dir == "" {
		dir = b.projectDir
	}

	result, err := linter.Run(ctx, dir, input.Paths, input.Linters, false, true, 300)
	if err != nil {
		return errResult(fmt.Sprintf("lint fix failed: %v", err)), struct{}{}, nil
	}

	var sb strings.Builder
	if result.Success {
		sb.WriteString("All fixable issues resolved. No remaining issues.\n")
	} else {
		fmt.Fprintf(&sb, "Fixed what could be auto-fixed. %d issue(s) remain:\n\n", result.TotalIssues)

		for _, issue := range result.Issues {
			fmt.Fprintf(&sb, "%s:%d:%d: %s (%s)\n", issue.File, issue.Line, issue.Column, issue.Message, issue.Linter)
		}
	}

	output := sb.String()
	b.autoIndex(ctx, output, "lint-fix-results")

	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: output}},
	}, struct{}{}, nil
}

func formatLintOutput(result *linter.Result) string {
	if result.Success {
		return "No lint issues found.\n"
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "%d issue(s) found:\n\n", result.TotalIssues)

	for _, issue := range result.Issues {
		if issue.SourceLine != "" {
			fmt.Fprintf(&sb, "%s:%d:%d: %s (%s) [%s]\n", issue.File, issue.Line, issue.Column, issue.Message, issue.Linter, issue.SourceLine)
		} else {
			fmt.Fprintf(&sb, "%s:%d:%d: %s (%s)\n", issue.File, issue.Line, issue.Column, issue.Message, issue.Linter)
		}
	}

	return sb.String()
}
