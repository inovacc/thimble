package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/inovacc/thimble/internal/gitops"
)

// ── Git Tool Input Types ──

type gitStatusInput struct {
	ProjectDir string `json:"project_dir,omitempty" jsonschema:"project directory (defaults to current working directory)"`
}

type gitDiffInput struct {
	ProjectDir   string   `json:"project_dir,omitempty" jsonschema:"project directory (defaults to current working directory)"`
	Staged       bool     `json:"staged,omitempty" jsonschema:"show staged changes (git diff --staged)"`
	Ref          string   `json:"ref,omitempty" jsonschema:"diff against ref (e.g. HEAD~3 or main)"`
	Paths        []string `json:"paths,omitempty" jsonschema:"limit diff to specific files"`
	ContextLines int32    `json:"context_lines,omitempty" jsonschema:"unified diff context lines (default 3)"`
	StatOnly     bool     `json:"stat_only,omitempty" jsonschema:"show only diffstat summary"`
}

type gitLogInput struct {
	ProjectDir string   `json:"project_dir,omitempty" jsonschema:"project directory (defaults to current working directory)"`
	Limit      int32    `json:"limit,omitempty" jsonschema:"max commits to return (default 20)"`
	Since      string   `json:"since,omitempty" jsonschema:"show commits after date (e.g. 2024-01-01 or '2 weeks ago')"`
	Until      string   `json:"until,omitempty" jsonschema:"show commits before date"`
	Author     string   `json:"author,omitempty" jsonschema:"filter by author name or email"`
	Ref        string   `json:"ref,omitempty" jsonschema:"branch, tag, or range (e.g. main..HEAD)"`
	Paths      []string `json:"paths,omitempty" jsonschema:"limit to commits touching these files"`
	Oneline    bool     `json:"oneline,omitempty" jsonschema:"compact one-line format"`
}

type gitBlameInput struct {
	ProjectDir string `json:"project_dir,omitempty" jsonschema:"project directory (defaults to current working directory)"`
	File       string `json:"file" jsonschema:"file path to blame"`
	LineStart  int32  `json:"line_start,omitempty" jsonschema:"start line (0 = entire file)"`
	LineEnd    int32  `json:"line_end,omitempty" jsonschema:"end line"`
	Ref        string `json:"ref,omitempty" jsonschema:"blame at a specific commit/ref"`
}

type gitBranchInput struct {
	ProjectDir string `json:"project_dir,omitempty" jsonschema:"project directory (defaults to current working directory)"`
	Remote     bool   `json:"remote,omitempty" jsonschema:"show remote branches only"`
	All        bool   `json:"all,omitempty" jsonschema:"show both local and remote branches"`
}

type gitCommitInput struct {
	ProjectDir string   `json:"project_dir,omitempty" jsonschema:"project directory (defaults to current working directory)"`
	Files      []string `json:"files,omitempty" jsonschema:"files to stage before committing (empty = commit staged changes)"`
	Message    string   `json:"message" jsonschema:"commit message (required)"`
	All        bool     `json:"all,omitempty" jsonschema:"stage all modified tracked files (git commit -a)"`
}

type gitChangelogInput struct {
	ProjectDir  string `json:"project_dir,omitempty" jsonschema:"project directory"`
	Since       string `json:"since,omitempty" jsonschema:"start ref: tag, commit, or date (e.g. v1.0.0)"`
	Until       string `json:"until,omitempty" jsonschema:"end ref (default HEAD)"`
	GroupByType bool   `json:"group_by_type,omitempty" jsonschema:"group commits by conventional commit type (default true)"`
}

type gitRebaseInput struct {
	ProjectDir     string `json:"project_dir,omitempty" jsonschema:"project directory (defaults to current working directory)"`
	Onto           string `json:"onto,omitempty" jsonschema:"branch or ref to rebase onto (e.g. main)"`
	Abort          bool   `json:"abort,omitempty" jsonschema:"abort in-progress rebase"`
	ContinueRebase bool   `json:"continue_rebase,omitempty" jsonschema:"continue rebase after resolving conflicts"`
	Skip           bool   `json:"skip,omitempty" jsonschema:"skip the current conflicting commit"`
}

type gitValidateBranchInput struct {
	Name string `json:"name" jsonschema:"branch name to validate against conventional patterns"`
}

type gitLintCommitInput struct {
	Message string `json:"message" jsonschema:"commit message to lint against conventional commits format"`
}

type gitMergeInput struct {
	ProjectDir string `json:"project_dir,omitempty" jsonschema:"project directory (defaults to current working directory)"`
	Branch     string `json:"branch" jsonschema:"branch to merge into current branch"`
	NoFF       bool   `json:"no_ff,omitempty" jsonschema:"always create a merge commit (--no-ff)"`
	Squash     bool   `json:"squash,omitempty" jsonschema:"squash commits into single change (--squash)"`
	Message    string `json:"message,omitempty" jsonschema:"custom merge commit message"`
	Abort      bool   `json:"abort,omitempty" jsonschema:"abort an in-progress merge"`
}

type gitListConflictsInput struct {
	ProjectDir string `json:"project_dir,omitempty" jsonschema:"project directory (defaults to current working directory)"`
}

type gitStashInput struct {
	ProjectDir string `json:"project_dir,omitempty" jsonschema:"project directory (defaults to current working directory)"`
	Action     string `json:"action,omitempty" jsonschema:"stash action: list (default), show, save, pop, drop"`
	Index      int32  `json:"index,omitempty" jsonschema:"stash index for show/pop/drop"`
	Message    string `json:"message,omitempty" jsonschema:"message for stash save"`
}

// registerGitTools registers git MCP tools on the bridge's MCP server.
func (b *Bridge) registerGitTools() {
	mcpsdk.AddTool(b.server, &mcpsdk.Tool{
		Name:        "ctx_git_status",
		Description: "Show git repository status: branch, staged/unstaged changes, untracked files, ahead/behind upstream.",
	}, b.handleGitStatus)

	mcpsdk.AddTool(b.server, &mcpsdk.Tool{
		Name:        "ctx_git_diff",
		Description: "Show git diff with options for staged changes, ref comparison, file filtering, and stat summaries.",
	}, b.handleGitDiff)

	mcpsdk.AddTool(b.server, &mcpsdk.Tool{
		Name:        "ctx_git_log",
		Description: "Show git commit history with filtering by date, author, ref range, and file paths.",
	}, b.handleGitLog)

	mcpsdk.AddTool(b.server, &mcpsdk.Tool{
		Name:        "ctx_git_blame",
		Description: "Show per-line git blame attribution with commit hash, author, date, and content.",
	}, b.handleGitBlame)

	mcpsdk.AddTool(b.server, &mcpsdk.Tool{
		Name:        "ctx_git_branches",
		Description: "List git branches with upstream tracking info, ahead/behind counts, and current branch indicator.",
	}, b.handleGitBranches)

	mcpsdk.AddTool(b.server, &mcpsdk.Tool{
		Name:        "ctx_git_stash",
		Description: "Manage git stashes: list, show, save, pop, drop.",
	}, b.handleGitStash)

	mcpsdk.AddTool(b.server, &mcpsdk.Tool{
		Name:        "ctx_git_commit",
		Description: "Stage files and create a git commit. Requires explicit commit message.",
	}, b.handleGitCommit)

	mcpsdk.AddTool(b.server, &mcpsdk.Tool{
		Name:        "ctx_git_changelog",
		Description: "Generate a markdown changelog from git history, optionally grouped by conventional commit type.",
	}, b.handleGitChangelog)

	mcpsdk.AddTool(b.server, &mcpsdk.Tool{
		Name:        "ctx_git_merge",
		Description: "Merge a branch into the current branch with conflict detection. Use abort to cancel in-progress merge.",
	}, b.handleGitMerge)

	mcpsdk.AddTool(b.server, &mcpsdk.Tool{
		Name:        "ctx_git_rebase",
		Description: "Rebase current branch onto another branch. Supports abort, continue, and skip for conflict resolution.",
	}, b.handleGitRebase)

	mcpsdk.AddTool(b.server, &mcpsdk.Tool{
		Name:        "ctx_git_validate_branch",
		Description: "Validate a branch name against conventional patterns (feat/*, fix/*, chore/*, etc.) and suggest corrections.",
	}, b.handleGitValidateBranch)

	mcpsdk.AddTool(b.server, &mcpsdk.Tool{
		Name:        "ctx_git_lint_commit",
		Description: "Lint a commit message against conventional commits format (type(scope): description) and suggest fixes.",
	}, b.handleGitLintCommit)

	mcpsdk.AddTool(b.server, &mcpsdk.Tool{
		Name:        "ctx_git_conflicts",
		Description: "List merge/rebase conflicts with parsed hunks showing both sides. Use after a failed merge or rebase to understand conflicts.",
	}, b.handleGitConflicts)
}

// ── Git Tool Handlers ──

func (b *Bridge) handleGitStatus(ctx context.Context, _ *mcpsdk.CallToolRequest, input gitStatusInput) (*mcpsdk.CallToolResult, struct{}, error) {
	ctx, finish := spanTool(ctx, "ctx_git_status")
	defer finish(nil)

	b.recordToolCall(ctx, "ctx_git_status", true)

	dir := input.ProjectDir
	if dir == "" {
		dir = b.projectDir
	}

	resp, err := gitops.Status(ctx, dir)
	if err != nil {
		return &mcpsdk.CallToolResult{
			Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: fmt.Sprintf("git status failed: %v", err)}},
			IsError: true,
		}, struct{}{}, nil
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Branch: %s", resp.Branch)

	if resp.Upstream != "" {
		fmt.Fprintf(&sb, " → %s", resp.Upstream)

		if resp.Ahead > 0 || resp.Behind > 0 {
			fmt.Fprintf(&sb, " (ahead %d, behind %d)", resp.Ahead, resp.Behind)
		}
	}

	sb.WriteString("\n")

	if resp.Clean {
		sb.WriteString("Working tree clean\n")
	} else {
		if len(resp.Staged) > 0 {
			sb.WriteString("\nStaged:\n")

			for _, f := range resp.Staged {
				fmt.Fprintf(&sb, "  %s  %s\n", f.Status, f.Path)
			}
		}

		if len(resp.Unstaged) > 0 {
			sb.WriteString("\nUnstaged:\n")

			for _, f := range resp.Unstaged {
				fmt.Fprintf(&sb, "  %s  %s\n", f.Status, f.Path)
			}
		}

		if len(resp.Untracked) > 0 {
			sb.WriteString("\nUntracked:\n")

			for _, f := range resp.Untracked {
				fmt.Fprintf(&sb, "  %s\n", f)
			}
		}
	}

	output := sb.String()
	b.autoIndex(ctx, output, "git-status")

	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: output}},
	}, struct{}{}, nil
}

func (b *Bridge) handleGitDiff(ctx context.Context, _ *mcpsdk.CallToolRequest, input gitDiffInput) (*mcpsdk.CallToolResult, struct{}, error) {
	ctx, finish := spanTool(ctx, "ctx_git_diff")
	defer finish(nil)

	b.recordToolCall(ctx, "ctx_git_diff", true)

	dir := input.ProjectDir
	if dir == "" {
		dir = b.projectDir
	}

	resp, err := gitops.Diff(ctx, dir, gitops.DiffOptions{
		Staged:       input.Staged,
		Ref:          input.Ref,
		Paths:        input.Paths,
		ContextLines: int(input.ContextLines),
		StatOnly:     input.StatOnly,
	})
	if err != nil {
		return &mcpsdk.CallToolResult{
			Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: fmt.Sprintf("git diff failed: %v", err)}},
			IsError: true,
		}, struct{}{}, nil
	}

	var output string
	if input.StatOnly {
		output = resp.Diff
	} else {
		var sb strings.Builder
		if resp.Stat != "" {
			sb.WriteString(resp.Stat)
			sb.WriteString("\n")
		}

		sb.WriteString(resp.Diff)
		output = sb.String()
	}

	if output == "" {
		output = "No differences found."
	}

	b.autoIndex(ctx, output, "git-diff")

	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: output}},
	}, struct{}{}, nil
}

func (b *Bridge) handleGitLog(ctx context.Context, _ *mcpsdk.CallToolRequest, input gitLogInput) (*mcpsdk.CallToolResult, struct{}, error) {
	ctx, finish := spanTool(ctx, "ctx_git_log")
	defer finish(nil)

	b.recordToolCall(ctx, "ctx_git_log", true)

	dir := input.ProjectDir
	if dir == "" {
		dir = b.projectDir
	}

	resp, err := gitops.Log(ctx, dir, gitops.LogOptions{
		Limit:  int(input.Limit),
		Since:  input.Since,
		Until:  input.Until,
		Author: input.Author,
		Ref:    input.Ref,
		Paths:  input.Paths,
	})
	if err != nil {
		return &mcpsdk.CallToolResult{
			Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: fmt.Sprintf("git log failed: %v", err)}},
			IsError: true,
		}, struct{}{}, nil
	}

	var sb strings.Builder

	for _, e := range resp.Entries {
		if input.Oneline {
			fmt.Fprintf(&sb, "%s %s\n", e.ShortHash, e.Subject)
		} else {
			fmt.Fprintf(&sb, "%s %s <%s> %s\n  %s\n", e.ShortHash, e.Author, e.Email, e.Date, e.Subject)

			if e.Body != "" {
				fmt.Fprintf(&sb, "  %s\n", e.Body)
			}

			sb.WriteString("\n")
		}
	}

	output := sb.String()
	if output == "" {
		output = "No commits found."
	}

	b.autoIndex(ctx, output, "git-log")

	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: output}},
	}, struct{}{}, nil
}

func (b *Bridge) handleGitBlame(ctx context.Context, _ *mcpsdk.CallToolRequest, input gitBlameInput) (*mcpsdk.CallToolResult, struct{}, error) {
	ctx, finish := spanTool(ctx, "ctx_git_blame")
	defer finish(nil)

	b.recordToolCall(ctx, "ctx_git_blame", true)

	dir := input.ProjectDir
	if dir == "" {
		dir = b.projectDir
	}

	resp, err := gitops.Blame(ctx, dir, gitops.BlameOptions{
		File:      input.File,
		Ref:       input.Ref,
		LineStart: int(input.LineStart),
		LineEnd:   int(input.LineEnd),
	})
	if err != nil {
		return &mcpsdk.CallToolResult{
			Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: fmt.Sprintf("git blame failed: %v", err)}},
			IsError: true,
		}, struct{}{}, nil
	}

	var sb strings.Builder
	for _, l := range resp.Lines {
		fmt.Fprintf(&sb, "%s %s %s %4d: %s\n", l.Hash[:8], l.Author, l.Date, l.LineNumber, l.Content)
	}

	output := sb.String()
	b.autoIndex(ctx, output, "git-blame")

	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: output}},
	}, struct{}{}, nil
}

func (b *Bridge) handleGitBranches(ctx context.Context, _ *mcpsdk.CallToolRequest, input gitBranchInput) (*mcpsdk.CallToolResult, struct{}, error) {
	ctx, finish := spanTool(ctx, "ctx_git_branches")
	defer finish(nil)

	b.recordToolCall(ctx, "ctx_git_branches", true)

	dir := input.ProjectDir
	if dir == "" {
		dir = b.projectDir
	}

	resp, err := gitops.BranchList(ctx, dir, gitops.BranchListOptions{
		Remote: input.Remote,
		All:    input.All,
	})
	if err != nil {
		return &mcpsdk.CallToolResult{
			Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: fmt.Sprintf("git branch failed: %v", err)}},
			IsError: true,
		}, struct{}{}, nil
	}

	var sb strings.Builder

	for _, br := range resp.Branches {
		marker := "  "
		if br.Current {
			marker = "* "
		}

		fmt.Fprintf(&sb, "%s%s %s", marker, br.Name, br.Hash)

		if br.Upstream != "" {
			fmt.Fprintf(&sb, " → %s", br.Upstream)

			if br.Ahead > 0 || br.Behind > 0 {
				fmt.Fprintf(&sb, " [ahead %d, behind %d]", br.Ahead, br.Behind)
			}
		}

		sb.WriteString("\n")
	}

	output := sb.String()

	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: output}},
	}, struct{}{}, nil
}

func (b *Bridge) handleGitStash(ctx context.Context, _ *mcpsdk.CallToolRequest, input gitStashInput) (*mcpsdk.CallToolResult, struct{}, error) {
	ctx, finish := spanTool(ctx, "ctx_git_stash")
	defer finish(nil)

	b.recordToolCall(ctx, "ctx_git_stash", false)

	dir := input.ProjectDir
	if dir == "" {
		dir = b.projectDir
	}

	// Security: check stash write operations against git policies.
	action := input.Action
	if action == "" {
		action = "list"
	}

	if action != "list" && action != "show" {
		if err := b.checkGitDeny(action); err != nil {
			return &mcpsdk.CallToolResult{				Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: err.Error()}},
				IsError: true,
			}, struct{}{}, nil
		}
	}

	resp, err := gitops.Stash(ctx, dir, gitops.StashOptions{
		Action:  action,
		Index:   int(input.Index),
		Message: input.Message,
	})
	if err != nil {
		return &mcpsdk.CallToolResult{
			Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: fmt.Sprintf("git stash %s failed: %v", action, err)}},
			IsError: true,
		}, struct{}{}, nil
	}

	var output string

	switch action {
	case "list":
		if len(resp.Entries) == 0 {
			output = "No stashes."
		} else {
			var sb strings.Builder
			for _, e := range resp.Entries {
				fmt.Fprintf(&sb, "stash@{%d}: %s\n", e.Index, e.Message)
			}

			output = sb.String()
		}
	case "show":
		output = resp.Diff
	default:
		output = resp.Output
	}

	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: output}},
	}, struct{}{}, nil
}

func (b *Bridge) handleGitCommit(ctx context.Context, _ *mcpsdk.CallToolRequest, input gitCommitInput) (*mcpsdk.CallToolResult, struct{}, error) {
	ctx, finish := spanTool(ctx, "ctx_git_commit")
	defer finish(nil)

	b.recordToolCall(ctx, "ctx_git_commit", false)

	if err := b.checkGitDeny("commit"); err != nil {
		return &mcpsdk.CallToolResult{			Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: err.Error()}},
			IsError: true,
		}, struct{}{}, nil
	}

	dir := input.ProjectDir
	if dir == "" {
		dir = b.projectDir
	}

	resp, err := gitops.Commit(ctx, dir, gitops.CommitOptions{
		Files:   input.Files,
		Message: input.Message,
		All:     input.All,
	})
	if err != nil {
		return &mcpsdk.CallToolResult{
			Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: fmt.Sprintf("git commit failed: %v", err)}},
			IsError: true,
		}, struct{}{}, nil
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Committed %s %s\n", resp.ShortHash, resp.Subject)

	if resp.FilesChanged > 0 || resp.Insertions > 0 || resp.Deletions > 0 {
		fmt.Fprintf(&sb, "%d file(s) changed, %d insertion(s), %d deletion(s)\n", resp.FilesChanged, resp.Insertions, resp.Deletions)
	}

	output := sb.String()
	b.autoIndex(ctx, output, "git-commit")

	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: output}},
	}, struct{}{}, nil
}

func (b *Bridge) handleGitChangelog(ctx context.Context, _ *mcpsdk.CallToolRequest, input gitChangelogInput) (*mcpsdk.CallToolResult, struct{}, error) {
	ctx, finish := spanTool(ctx, "ctx_git_changelog")
	defer finish(nil)

	b.recordToolCall(ctx, "ctx_git_changelog", true)

	dir := input.ProjectDir
	if dir == "" {
		dir = b.projectDir
	}

	// Default group_by_type to true when not explicitly set.
	groupByType := input.GroupByType

	resp, err := gitops.Changelog(ctx, dir, gitops.ChangelogOptions{
		Since:       input.Since,
		Until:       input.Until,
		GroupByType: groupByType,
	})
	if err != nil {
		return &mcpsdk.CallToolResult{
			Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: fmt.Sprintf("git changelog failed: %v", err)}},
			IsError: true,
		}, struct{}{}, nil
	}

	output := resp.Changelog
	b.autoIndex(ctx, output, "git-changelog")

	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: output}},
	}, struct{}{}, nil
}

func (b *Bridge) handleGitMerge(ctx context.Context, _ *mcpsdk.CallToolRequest, input gitMergeInput) (*mcpsdk.CallToolResult, struct{}, error) {
	ctx, finish := spanTool(ctx, "ctx_git_merge")
	defer finish(nil)

	b.recordToolCall(ctx, "ctx_git_merge", false)

	if err := b.checkGitDeny("merge"); err != nil {
		return &mcpsdk.CallToolResult{			Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: err.Error()}},
			IsError: true,
		}, struct{}{}, nil
	}

	dir := input.ProjectDir
	if dir == "" {
		dir = b.projectDir
	}

	resp, err := gitops.Merge(ctx, dir, gitops.MergeOptions{
		Branch:  input.Branch,
		NoFf:    input.NoFF,
		Squash:  input.Squash,
		Message: input.Message,
		Abort:   input.Abort,
	})
	if err != nil {
		return &mcpsdk.CallToolResult{
			Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: fmt.Sprintf("git merge failed: %v", err)}},
			IsError: true,
		}, struct{}{}, nil
	}

	var sb strings.Builder

	switch {
	case resp.HasConflicts:
		sb.WriteString("Merge conflicts detected!\n\n")
		sb.WriteString(resp.Output)

		if len(resp.ConflictFiles) > 0 {
			sb.WriteString("\n\nConflicting files:\n")

			for _, f := range resp.ConflictFiles {
				fmt.Fprintf(&sb, "  - %s\n", f)
			}
		}
	case resp.Success:
		sb.WriteString("Merge successful.\n")

		if resp.MergeHash != "" {
			fmt.Fprintf(&sb, "Commit: %s\n", resp.MergeHash)
		}

		if resp.Output != "" {
			fmt.Fprintf(&sb, "\n%s\n", resp.Output)
		}
	default:
		sb.WriteString(resp.Output)
	}

	output := sb.String()
	b.autoIndex(ctx, output, "git-merge")

	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: output}},
	}, struct{}{}, nil
}

func (b *Bridge) handleGitRebase(ctx context.Context, _ *mcpsdk.CallToolRequest, input gitRebaseInput) (*mcpsdk.CallToolResult, struct{}, error) {
	ctx, finish := spanTool(ctx, "ctx_git_rebase")
	defer finish(nil)

	b.recordToolCall(ctx, "ctx_git_rebase", false)

	// Security: only deny rebase for non-recovery operations.
	// Abort, continue, and skip are recovery operations and should not be denied.
	if !input.Abort && !input.ContinueRebase && !input.Skip {
		if err := b.checkGitDeny("rebase"); err != nil {
			return &mcpsdk.CallToolResult{				Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: err.Error()}},
				IsError: true,
			}, struct{}{}, nil
		}
	}

	dir := input.ProjectDir
	if dir == "" {
		dir = b.projectDir
	}

	resp, err := gitops.Rebase(ctx, dir, gitops.RebaseOptions{
		Onto:     input.Onto,
		Abort:    input.Abort,
		Continue: input.ContinueRebase,
		Skip:     input.Skip,
	})
	if err != nil {
		return &mcpsdk.CallToolResult{
			Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: fmt.Sprintf("git rebase failed: %v", err)}},
			IsError: true,
		}, struct{}{}, nil
	}

	var sb strings.Builder

	switch {
	case resp.HasConflicts:
		sb.WriteString("Rebase conflicts detected!\n\n")
		sb.WriteString(resp.Output)

		if len(resp.ConflictFiles) > 0 {
			sb.WriteString("\n\nConflicting files:\n")

			for _, f := range resp.ConflictFiles {
				fmt.Fprintf(&sb, "  - %s\n", f)
			}
		}

		sb.WriteString("\nUse abort, continue_rebase, or skip to resolve.\n")
	case resp.Success:
		sb.WriteString("Rebase successful.\n")

		if resp.Output != "" {
			fmt.Fprintf(&sb, "\n%s\n", resp.Output)
		}
	default:
		sb.WriteString(resp.Output)
	}

	if resp.InProgress {
		sb.WriteString("\nRebase is still in progress.\n")
	}

	output := sb.String()
	b.autoIndex(ctx, output, "git-rebase")

	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: output}},
	}, struct{}{}, nil
}

func (b *Bridge) handleGitValidateBranch(ctx context.Context, _ *mcpsdk.CallToolRequest, input gitValidateBranchInput) (*mcpsdk.CallToolResult, struct{}, error) {
	ctx, finish := spanTool(ctx, "ctx_git_validate_branch")
	defer finish(nil)

	b.recordToolCall(ctx, "ctx_git_validate_branch", true)

	resp := gitops.ValidateBranch(input.Name)

	var sb strings.Builder
	if resp.Valid {
		fmt.Fprintf(&sb, "Valid branch name: %s\nMatched pattern: %s\n", input.Name, resp.Pattern)
	} else {
		fmt.Fprintf(&sb, "Invalid branch name: %s\nIssue: %s\nExpected patterns: %s\n", input.Name, resp.Suggestion, resp.Pattern)
	}

	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: sb.String()}},
	}, struct{}{}, nil
}

func (b *Bridge) handleGitLintCommit(ctx context.Context, _ *mcpsdk.CallToolRequest, input gitLintCommitInput) (*mcpsdk.CallToolResult, struct{}, error) {
	ctx, finish := spanTool(ctx, "ctx_git_lint_commit")
	defer finish(nil)

	b.recordToolCall(ctx, "ctx_git_lint_commit", true)

	resp := gitops.LintCommit(input.Message)

	var sb strings.Builder
	if resp.Valid {
		sb.WriteString("Commit message is valid.\n")
	} else {
		sb.WriteString("Commit message issues:\n")

		for _, issue := range resp.Issues {
			fmt.Fprintf(&sb, "  - %s\n", issue)
		}

		if resp.Suggested != "" {
			fmt.Fprintf(&sb, "\nSuggested:\n  %s\n", resp.Suggested)
		}
	}

	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: sb.String()}},
	}, struct{}{}, nil
}

func (b *Bridge) handleGitConflicts(ctx context.Context, _ *mcpsdk.CallToolRequest, input gitListConflictsInput) (*mcpsdk.CallToolResult, struct{}, error) {
	ctx, finish := spanTool(ctx, "ctx_git_conflicts")
	defer finish(nil)

	b.recordToolCall(ctx, "ctx_git_conflicts", true)

	dir := input.ProjectDir
	if dir == "" {
		dir = b.projectDir
	}

	resp, err := gitops.ListConflicts(ctx, dir)
	if err != nil {
		return &mcpsdk.CallToolResult{
			Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: fmt.Sprintf("git conflicts failed: %v", err)}},
			IsError: true,
		}, struct{}{}, nil
	}

	var sb strings.Builder
	if len(resp.Files) == 0 {
		sb.WriteString("No conflicts found.\n")
	} else {
		if resp.MergeType != "" {
			fmt.Fprintf(&sb, "Conflict type: %s\n", resp.MergeType)
		}

		fmt.Fprintf(&sb, "Files with conflicts: %d\n", len(resp.Files))
		fmt.Fprintf(&sb, "Total conflict hunks: %d\n\n", resp.TotalConflicts)

		for _, f := range resp.Files {
			fmt.Fprintf(&sb, "── %s ──\n", f)

			for _, h := range resp.Hunks {
				if h.File != f {
					continue
				}

				fmt.Fprintf(&sb, "  Lines %d-%d:\n", h.StartLine, h.EndLine)
				fmt.Fprintf(&sb, "    OURS (HEAD):\n")

				for line := range strings.SplitSeq(h.Ours, "\n") {
					fmt.Fprintf(&sb, "      %s\n", line)
				}

				fmt.Fprintf(&sb, "    THEIRS (incoming):\n")

				for line := range strings.SplitSeq(h.Theirs, "\n") {
					fmt.Fprintf(&sb, "      %s\n", line)
				}

				if h.Ancestor != "" {
					fmt.Fprintf(&sb, "    ANCESTOR (common base):\n")

					for line := range strings.SplitSeq(h.Ancestor, "\n") {
						fmt.Fprintf(&sb, "      %s\n", line)
					}
				}

				sb.WriteString("\n")
			}
		}
	}

	output := sb.String()
	b.autoIndex(ctx, output, "git-conflicts")

	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: output}},
	}, struct{}{}, nil
}

// autoIndex indexes output into the knowledge base if non-empty.
func (b *Bridge) autoIndex(_ context.Context, content, label string) {
	if content == "" || b.content == nil {
		return
	}

	_, _ = b.content.Index(content, label)
}

// checkGitDeny checks if a git subcommand is denied by security policy.
func (b *Bridge) checkGitDeny(subcommand string) error {
	return b.checkCommandDeny("git " + subcommand)
}

// marshalJSON marshals v to indented JSON, returning the error string on failure.
func marshalJSON(v any) string {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf("marshal error: %v", err)
	}

	return string(data)
}
