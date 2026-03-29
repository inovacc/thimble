package mcp

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/inovacc/thimble/internal/ghcli"
	"github.com/inovacc/thimble/internal/gitops"
	"github.com/inovacc/thimble/internal/security"
)

// ── GH CLI Tool Input Types ──

type ghExecInput struct {
	Args       []string `json:"args" jsonschema:"gh subcommand and arguments (e.g. [pr list --state open])"`
	JSONFields []string `json:"json_fields,omitempty" jsonschema:"auto-append --json with these fields for structured output (e.g. [number title state])"`
	ProjectDir string   `json:"project_dir,omitempty" jsonschema:"project directory for repo context (defaults to current working directory)"`
	TimeoutMs  int32    `json:"timeout_ms,omitempty" jsonschema:"execution timeout in milliseconds (default 30000)"`
}

type ghPRStatusInput struct {
	ProjectDir string `json:"project_dir,omitempty" jsonschema:"project directory (defaults to current working directory)"`
}

type ghRunStatusInput struct {
	ProjectDir string `json:"project_dir,omitempty" jsonschema:"project directory (defaults to current working directory)"`
	Limit      int    `json:"limit,omitempty" jsonschema:"max runs to show (default 10)"`
}

type ghIssueListInput struct {
	ProjectDir string   `json:"project_dir,omitempty" jsonschema:"project directory (defaults to current working directory)"`
	State      string   `json:"state,omitempty" jsonschema:"filter by state: open (default), closed, all"`
	Labels     []string `json:"labels,omitempty" jsonschema:"filter by labels"`
	Assignee   string   `json:"assignee,omitempty" jsonschema:"filter by assignee (@me for self)"`
	Limit      int      `json:"limit,omitempty" jsonschema:"max issues to show (default 20)"`
}

type ghSearchInput struct {
	Query      string `json:"query" jsonschema:"search query (GitHub search syntax)"`
	Type       string `json:"type,omitempty" jsonschema:"search type: repos (default), issues, prs, code, commits"`
	Limit      int    `json:"limit,omitempty" jsonschema:"max results (default 20)"`
	ProjectDir string `json:"project_dir,omitempty" jsonschema:"project directory for repo context"`
}

type ghAPIInput struct {
	Endpoint   string `json:"endpoint" jsonschema:"GitHub API endpoint (e.g. /repos/{owner}/{repo}/pulls)"`
	Method     string `json:"method,omitempty" jsonschema:"HTTP method: GET (default), POST, PATCH, PUT, DELETE"`
	Body       string `json:"body,omitempty" jsonschema:"JSON request body for POST/PATCH/PUT"`
	JQ         string `json:"jq,omitempty" jsonschema:"jq filter expression to process response"`
	ProjectDir string `json:"project_dir,omitempty" jsonschema:"project directory for repo context"`
}

type ghPRTemplateInput struct {
	ProjectDir string `json:"project_dir,omitempty" jsonschema:"project directory (defaults to current)"`
	BaseBranch string `json:"base_branch,omitempty" jsonschema:"base branch to diff against (default: main)"`
}

type ghRepoViewInput struct {
	Repo       string   `json:"repo,omitempty" jsonschema:"repository (owner/name). Defaults to current repo."`
	JSONFields []string `json:"json_fields,omitempty" jsonschema:"fields to include (e.g. [name description stargazerCount])"`
	ProjectDir string   `json:"project_dir,omitempty" jsonschema:"project directory for repo context"`
}

// registerGhTools registers GitHub CLI MCP tools on the bridge's MCP server.
func (b *Bridge) registerGhTools() {
	mcpsdk.AddTool(b.server, &mcpsdk.Tool{
		Name:        "ctx_gh",
		Description: "Run any GitHub CLI (gh) command. Uses the user's existing gh auth. Output is auto-indexed into the knowledge base.",
	}, b.handleGhExec)

	mcpsdk.AddTool(b.server, &mcpsdk.Tool{
		Name:        "ctx_gh_pr_status",
		Description: "Show pull request status for the current branch: checks, reviews, merge state.",
	}, b.handleGhPRStatus)

	mcpsdk.AddTool(b.server, &mcpsdk.Tool{
		Name:        "ctx_gh_run_status",
		Description: "Show recent GitHub Actions workflow runs with status, conclusion, and duration.",
	}, b.handleGhRunStatus)

	mcpsdk.AddTool(b.server, &mcpsdk.Tool{
		Name:        "ctx_gh_issue_list",
		Description: "List GitHub issues with filtering by state, labels, and assignee.",
	}, b.handleGhIssueList)

	mcpsdk.AddTool(b.server, &mcpsdk.Tool{
		Name:        "ctx_gh_search",
		Description: "Search GitHub for repositories, issues, PRs, code, or commits using GitHub search syntax.",
	}, b.handleGhSearch)

	mcpsdk.AddTool(b.server, &mcpsdk.Tool{
		Name:        "ctx_gh_api",
		Description: "Call the GitHub REST API directly with optional jq filtering. Uses the user's gh auth token.",
	}, b.handleGhAPI)

	mcpsdk.AddTool(b.server, &mcpsdk.Tool{
		Name:        "ctx_gh_repo_view",
		Description: "View repository details: description, stars, forks, language, license, topics.",
	}, b.handleGhRepoView)

	mcpsdk.AddTool(b.server, &mcpsdk.Tool{
		Name:        "ctx_gh_pr_template",
		Description: "Generate a pull request description from the current branch's changes. Analyzes diff against base branch and creates a structured PR body.",
	}, b.handleGhPRTemplate)
}

// ── GH CLI Tool Handlers ──

func (b *Bridge) handleGhExec(ctx context.Context, _ *mcpsdk.CallToolRequest, input ghExecInput) (*mcpsdk.CallToolResult, struct{}, error) {
	ctx, finish := spanTool(ctx, "ctx_gh")
	defer finish(nil)

	b.recordToolCall(ctx, "ctx_gh", false)

	if len(input.Args) == 0 {
		return errResult("args required (e.g. [\"pr\", \"list\"])"), struct{}{}, nil
	}

	// Security check: deny dangerous gh commands (with configurable overrides).

	fullCmd := "gh " + strings.Join(input.Args, " ")

	dir := input.ProjectDir
	if dir == "" {
		dir = b.projectDir
	}

	policies := security.ReadBashPolicies(dir, "")
	_, ghPolicy := security.CustomPoliciesFromSettings(policies)

	if pattern := security.IsGhCommandDangerousWithPolicy(fullCmd, ghPolicy); pattern != "" {
		return errResult(fmt.Sprintf("command denied by security policy: %s", pattern)), struct{}{}, nil
	}

	resp, err := ghcli.Exec(ctx, dir, input.Args, input.JSONFields, int64(input.TimeoutMs))
	if err != nil {
		return errResult(fmt.Sprintf("gh exec failed: %v", err)), struct{}{}, nil
	}

	output := formatGhOutput(resp)

	label := "gh-" + input.Args[0]
	if len(input.Args) > 1 {
		label += "-" + input.Args[1]
	}

	b.autoIndex(ctx, output, label)

	isErr := resp.ExitCode != 0 && !resp.TimedOut

	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: output}},
		IsError: isErr,
	}, struct{}{}, nil
}

func (b *Bridge) handleGhPRStatus(ctx context.Context, _ *mcpsdk.CallToolRequest, input ghPRStatusInput) (*mcpsdk.CallToolResult, struct{}, error) {
	ctx, finish := spanTool(ctx, "ctx_gh_pr_status")
	defer finish(nil)

	b.recordToolCall(ctx, "ctx_gh_pr_status", true)

	dir := input.ProjectDir
	if dir == "" {
		dir = b.projectDir
	}

	resp, err := ghcli.Exec(ctx, dir, []string{"pr", "status"}, nil, 30000)
	if err != nil {
		return errResult(fmt.Sprintf("gh pr status failed: %v", err)), struct{}{}, nil
	}

	output := formatGhOutput(resp)
	b.autoIndex(ctx, output, "gh-pr-status")

	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: output}},
	}, struct{}{}, nil
}

func (b *Bridge) handleGhRunStatus(ctx context.Context, _ *mcpsdk.CallToolRequest, input ghRunStatusInput) (*mcpsdk.CallToolResult, struct{}, error) {
	ctx, finish := spanTool(ctx, "ctx_gh_run_status")
	defer finish(nil)

	b.recordToolCall(ctx, "ctx_gh_run_status", true)

	dir := input.ProjectDir
	if dir == "" {
		dir = b.projectDir
	}

	limit := input.Limit
	if limit <= 0 {
		limit = 10
	}

	resp, err := ghcli.Exec(ctx, dir, []string{"run", "list", "--limit", fmt.Sprintf("%d", limit)}, nil, 30000)
	if err != nil {
		return errResult(fmt.Sprintf("gh run list failed: %v", err)), struct{}{}, nil
	}

	output := formatGhOutput(resp)
	b.autoIndex(ctx, output, "gh-run-status")

	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: output}},
	}, struct{}{}, nil
}

func (b *Bridge) handleGhIssueList(ctx context.Context, _ *mcpsdk.CallToolRequest, input ghIssueListInput) (*mcpsdk.CallToolResult, struct{}, error) {
	ctx, finish := spanTool(ctx, "ctx_gh_issue_list")
	defer finish(nil)

	b.recordToolCall(ctx, "ctx_gh_issue_list", true)

	dir := input.ProjectDir
	if dir == "" {
		dir = b.projectDir
	}

	args := []string{"issue", "list"}
	if input.State != "" {
		args = append(args, "--state", input.State)
	}

	for _, label := range input.Labels {
		args = append(args, "--label", label)
	}

	if input.Assignee != "" {
		args = append(args, "--assignee", input.Assignee)
	}

	limit := input.Limit
	if limit <= 0 {
		limit = 20
	}

	args = append(args, "--limit", fmt.Sprintf("%d", limit))

	resp, err := ghcli.Exec(ctx, dir, args, nil, 30000)
	if err != nil {
		return errResult(fmt.Sprintf("gh issue list failed: %v", err)), struct{}{}, nil
	}

	output := formatGhOutput(resp)
	b.autoIndex(ctx, output, "gh-issue-list")

	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: output}},
	}, struct{}{}, nil
}

func (b *Bridge) handleGhSearch(ctx context.Context, _ *mcpsdk.CallToolRequest, input ghSearchInput) (*mcpsdk.CallToolResult, struct{}, error) {
	ctx, finish := spanTool(ctx, "ctx_gh_search")
	defer finish(nil)

	b.recordToolCall(ctx, "ctx_gh_search", true)

	dir := input.ProjectDir
	if dir == "" {
		dir = b.projectDir
	}

	searchType := input.Type
	if searchType == "" {
		searchType = "repos"
	}

	limit := input.Limit
	if limit <= 0 {
		limit = 20
	}

	args := []string{"search", searchType, input.Query, "--limit", fmt.Sprintf("%d", limit)}

	resp, err := ghcli.Exec(ctx, dir, args, nil, 30000)
	if err != nil {
		return errResult(fmt.Sprintf("gh search failed: %v", err)), struct{}{}, nil
	}

	output := formatGhOutput(resp)
	b.autoIndex(ctx, output, "gh-search-"+searchType)

	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: output}},
	}, struct{}{}, nil
}

func (b *Bridge) handleGhAPI(ctx context.Context, _ *mcpsdk.CallToolRequest, input ghAPIInput) (*mcpsdk.CallToolResult, struct{}, error) {
	ctx, finish := spanTool(ctx, "ctx_gh_api")
	defer finish(nil)

	b.recordToolCall(ctx, "ctx_gh_api", false)

	dir := input.ProjectDir
	if dir == "" {
		dir = b.projectDir
	}

	args := []string{"api", input.Endpoint}
	if input.Method != "" && input.Method != http.MethodGet {
		args = append(args, "--method", input.Method)
	}

	if input.Body != "" {
		args = append(args, "--input", "-")
	}

	if input.JQ != "" {
		args = append(args, "--jq", input.JQ)
	}

	// Security: deny destructive API calls unless method is GET.
	method := strings.ToUpper(input.Method)
	if method == "" {
		method = "GET"
	}

	if method == "DELETE" {
		fullCmd := "gh api --method DELETE " + input.Endpoint
		apiPolicies := security.ReadBashPolicies(dir, "")
		_, apiGhPolicy := security.CustomPoliciesFromSettings(apiPolicies)

		if pattern := security.IsGhCommandDangerousWithPolicy(fullCmd, apiGhPolicy); pattern != "" {
			return errResult(fmt.Sprintf("API call denied by security policy: %s", pattern)), struct{}{}, nil
		}
	}

	resp, err := ghcli.Exec(ctx, dir, args, nil, 30000)
	if err != nil {
		return errResult(fmt.Sprintf("gh api failed: %v", err)), struct{}{}, nil
	}

	output := formatGhOutput(resp)
	b.autoIndex(ctx, output, "gh-api")

	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: output}},
		IsError: resp.ExitCode != 0,
	}, struct{}{}, nil
}

func (b *Bridge) handleGhRepoView(ctx context.Context, _ *mcpsdk.CallToolRequest, input ghRepoViewInput) (*mcpsdk.CallToolResult, struct{}, error) {
	ctx, finish := spanTool(ctx, "ctx_gh_repo_view")
	defer finish(nil)

	b.recordToolCall(ctx, "ctx_gh_repo_view", true)

	dir := input.ProjectDir
	if dir == "" {
		dir = b.projectDir
	}

	args := []string{"repo", "view"}
	if input.Repo != "" {
		args = append(args, input.Repo)
	}

	jsonFields := input.JSONFields
	if len(jsonFields) == 0 {
		jsonFields = []string{"name", "owner", "description", "url", "stargazerCount", "forkCount", "primaryLanguage", "licenseInfo", "isPrivate", "defaultBranchRef"}
	}

	resp, err := ghcli.Exec(ctx, dir, args, jsonFields, 30000)
	if err != nil {
		return errResult(fmt.Sprintf("gh repo view failed: %v", err)), struct{}{}, nil
	}

	output := formatGhOutput(resp)
	b.autoIndex(ctx, output, "gh-repo-view")

	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: output}},
	}, struct{}{}, nil
}

func (b *Bridge) handleGhPRTemplate(ctx context.Context, _ *mcpsdk.CallToolRequest, input ghPRTemplateInput) (*mcpsdk.CallToolResult, struct{}, error) {
	ctx, finish := spanTool(ctx, "ctx_gh_pr_template")
	defer finish(nil)

	b.recordToolCall(ctx, "ctx_gh_pr_template", true)

	dir := input.ProjectDir
	if dir == "" {
		dir = b.projectDir
	}

	baseBranch := input.BaseBranch
	if baseBranch == "" {
		baseBranch = "main"
	}

	// 1. Get current branch.
	statusResult, err := gitops.Status(ctx, dir)
	if err != nil {
		return errResult(fmt.Sprintf("git status failed: %v", err)), struct{}{}, nil
	}

	currentBranch := statusResult.Branch

	// 2. Get diff stat.
	diffResult, err := gitops.Diff(ctx, dir, gitops.DiffOptions{
		StatOnly: true,
		Ref:      baseBranch,
	})
	if err != nil {
		return errResult(fmt.Sprintf("git diff failed: %v", err)), struct{}{}, nil
	}

	// 3. Get commit log.
	logResult, err := gitops.Log(ctx, dir, gitops.LogOptions{
		Ref: baseBranch + "..HEAD",
	})
	if err != nil {
		return errResult(fmt.Sprintf("git log failed: %v", err)), struct{}{}, nil
	}

	// 4. Build markdown PR template.
	var sb strings.Builder
	sb.WriteString("## Summary\n")

	if len(logResult.Entries) > 0 {
		for _, e := range logResult.Entries {
			fmt.Fprintf(&sb, "- %s\n", e.Subject)
		}
	} else {
		fmt.Fprintf(&sb, "Branch `%s` has no new commits relative to `%s`.\n", currentBranch, baseBranch)
	}

	sb.WriteString("\n## Changes\n")

	if diffResult.Diff != "" {
		sb.WriteString(diffResult.Diff)
		sb.WriteString("\n")
	} else {
		sb.WriteString("No file changes detected.\n")
	}

	if len(logResult.Entries) > 0 {
		sb.WriteString("\n### Commits\n")

		for _, e := range logResult.Entries {
			fmt.Fprintf(&sb, "- %s %s\n", e.ShortHash, e.Subject)
		}
	}

	sb.WriteString("\n## Test Plan\n")
	sb.WriteString("- [ ] Tests pass locally\n")
	sb.WriteString("- [ ] Lint clean\n")

	output := sb.String()
	b.autoIndex(ctx, output, "gh-pr-template")

	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: output}},
	}, struct{}{}, nil
}

// ── Helpers ──

func formatGhOutput(resp *ghcli.ExecResult) string {
	if resp.TimedOut {
		return "gh command timed out"
	}

	var sb strings.Builder
	if resp.Stdout != "" {
		sb.WriteString(resp.Stdout)
	}

	if resp.Stderr != "" {
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}

		sb.WriteString(resp.Stderr)
	}

	output := sb.String()
	if output == "" {
		if resp.ExitCode != 0 {
			return fmt.Sprintf("gh exited with code %d (no output)", resp.ExitCode)
		}

		return "(no results)"
	}

	return output
}

func errResult(msg string) *mcpsdk.CallToolResult {
	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: msg}},
		IsError: true,
	}
}
