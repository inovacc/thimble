// Package gitops provides git subprocess operations.
package gitops

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const gitTimeout = 30 * time.Second

// git runs a git command in the given directory and returns stdout.
func git(ctx context.Context, dir string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, gitTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir

	out, err := cmd.Output()
	if err != nil {
		ee := &exec.ExitError{}
		if errors.As(err, &ee) {
			// Include both stdout and stderr in the error message.
			// On Windows, git may write conflict markers to stdout rather than stderr.
			combined := strings.TrimSpace(string(out) + "\n" + string(ee.Stderr))
			return "", fmt.Errorf("git %s: %s", args[0], combined)
		}

		return "", fmt.Errorf("git %s: %w", args[0], err)
	}

	return string(out), nil
}

// ── Types ──

// FileStatus represents a file's git status.
type FileStatus struct {
	Path   string
	Status string // "modified", "added", "deleted", "renamed", "copied"
}

// StatusResult holds the output of git status.
type StatusResult struct {
	Branch    string
	Upstream  string
	Ahead     int
	Behind    int
	Staged    []FileStatus
	Unstaged  []FileStatus
	Untracked []string
	Clean     bool
}

// DiffResult holds the output of git diff.
type DiffResult struct {
	Diff         string
	Stat         string
	FilesChanged int
	Insertions   int
	Deletions    int
}

// LogEntry represents a single git log entry.
type LogEntry struct {
	Hash      string
	ShortHash string
	Author    string
	Email     string
	Date      string
	Subject   string
	Body      string
}

// LogResult holds git log output.
type LogResult struct {
	Entries []LogEntry
	Total   int
}

// BlameLine represents a single line of git blame output.
type BlameLine struct {
	Hash       string
	Author     string
	Date       string
	LineNumber int
	Content    string
}

// BlameResult holds git blame output.
type BlameResult struct {
	Lines []BlameLine
}

// Branch represents a git branch.
type Branch struct {
	Name     string
	Hash     string
	Upstream string
	Current  bool
	Ahead    int
	Behind   int
}

// BranchListResult holds the output of git branch.
type BranchListResult struct {
	Branches []Branch
	Current  string
}

// CommitResult holds the output of git commit.
type CommitResult struct {
	Hash         string
	ShortHash    string
	Subject      string
	Success      bool
	FilesChanged int
	Insertions   int
	Deletions    int
}

// ChangelogResult holds generated changelog output.
type ChangelogResult struct {
	Changelog    string
	TotalCommits int
	ByType       map[string]int
}

// MergeResult holds git merge output.
type MergeResult struct {
	Success       bool
	Output        string
	MergeHash     string
	HasConflicts  bool
	ConflictFiles []string
}

// RebaseResult holds git rebase output.
type RebaseResult struct {
	Success       bool
	Output        string
	HasConflicts  bool
	ConflictFiles []string
	InProgress    bool
}

// ConflictHunk represents a single conflict region in a file.
type ConflictHunk struct {
	File      string
	StartLine int
	EndLine   int
	Ours      string
	Theirs    string
	Ancestor  string
}

// ListConflictsResult holds conflict information.
type ListConflictsResult struct {
	Files          []string
	Hunks          []ConflictHunk
	TotalConflicts int
	MergeType      string // "merge" or "rebase"
}

// StashEntry represents a single stash entry.
type StashEntry struct {
	Index   int
	Message string
}

// StashResult holds git stash output.
type StashResult struct {
	Entries []StashEntry
	Diff    string
	Success bool
	Output  string
}

// ValidateBranchResult holds branch name validation output.
type ValidateBranchResult struct {
	Valid      bool
	Suggestion string
	Pattern    string
}

// LintCommitResult holds commit message lint output.
type LintCommitResult struct {
	Valid     bool
	Issues    []string
	Suggested string
}

// ── Branch validation ──

var validBranchPrefixes = []string{
	"feat/", "fix/", "chore/", "docs/", "test/", "refactor/",
	"release/v", "hotfix/",
}

var branchNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9._/-]*[a-z0-9]$`)

// ValidateBranch checks a branch name against conventional patterns.
func ValidateBranch(name string) *ValidateBranchResult {
	if name == "" {
		return &ValidateBranchResult{Valid: false, Suggestion: "branch name must not be empty"}
	}

	if !branchNameRe.MatchString(name) {
		suggestion := suggestBranchName(name)

		return &ValidateBranchResult{
			Valid:      false,
			Suggestion: fmt.Sprintf("invalid characters; try: %s", suggestion),
			Pattern:    "feat/*, fix/*, chore/*, docs/*, test/*, refactor/*, release/v*, hotfix/*",
		}
	}

	if strings.Contains(name, "..") || strings.Contains(name, "~") || strings.Contains(name, "^") || strings.Contains(name, ":") || strings.Contains(name, " ") {
		suggestion := suggestBranchName(name)

		return &ValidateBranchResult{
			Valid:      false,
			Suggestion: fmt.Sprintf("contains git-invalid characters; try: %s", suggestion),
			Pattern:    "feat/*, fix/*, chore/*, docs/*, test/*, refactor/*, release/v*, hotfix/*",
		}
	}

	for _, prefix := range validBranchPrefixes {
		if strings.HasPrefix(name, prefix) {
			rest := name[len(prefix):]
			if rest == "" {
				return &ValidateBranchResult{
					Valid:      false,
					Suggestion: fmt.Sprintf("branch name needs a descriptor after %s", prefix),
					Pattern:    "feat/*, fix/*, chore/*, docs/*, test/*, refactor/*, release/v*, hotfix/*",
				}
			}

			return &ValidateBranchResult{Valid: true, Pattern: prefix + "*"}
		}
	}

	if name == "main" || name == "master" || name == "develop" {
		return &ValidateBranchResult{Valid: true, Pattern: name}
	}

	suggestion := suggestBranchName(name)

	return &ValidateBranchResult{
		Valid:      false,
		Suggestion: fmt.Sprintf("use conventional prefix (feat/, fix/, chore/, docs/, test/, refactor/, release/v, hotfix/); try: feat/%s", suggestion),
		Pattern:    "feat/*, fix/*, chore/*, docs/*, test/*, refactor/*, release/v*, hotfix/*",
	}
}

func suggestBranchName(name string) string {
	s := strings.ToLower(name)
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "_", "-")

	var b strings.Builder

	for _, c := range s {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' || c == '/' || c == '.' {
			b.WriteRune(c)
		}
	}

	result := b.String()

	result = strings.Trim(result, "-./")
	if result == "" {
		return "my-branch"
	}

	return result
}

// ── Commit message linting ──

var conventionalTypes = map[string]bool{
	"feat": true, "fix": true, "docs": true, "test": true,
	"chore": true, "refactor": true, "perf": true, "ci": true,
	"build": true, "style": true,
}

var conventionalCommitRe = regexp.MustCompile(`^([a-z]+)(?:\(([^)]+)\))?\s*:\s*(.*)$`)

// LintCommit checks a commit message against conventional commits format.
func LintCommit(message string) *LintCommitResult {
	issues := lintCommitMessage(message)

	result := &LintCommitResult{
		Valid:  len(issues) == 0,
		Issues: issues,
	}
	if len(issues) > 0 {
		result.Suggested = suggestCommitMessage(message)
	}

	return result
}

func lintCommitMessage(message string) []string {
	if message == "" {
		return []string{"commit message must not be empty"}
	}

	lines := strings.Split(message, "\n")
	subject := lines[0]

	var issues []string

	if len(subject) > 72 {
		issues = append(issues, fmt.Sprintf("subject line is %d chars, should be ≤72", len(subject)))
	}

	if strings.HasSuffix(strings.TrimSpace(subject), ".") {
		issues = append(issues, "subject line should not end with a period")
	}

	m := conventionalCommitRe.FindStringSubmatch(subject)
	if m == nil {
		issues = append(issues, "subject must follow conventional format: type(scope): description")
		return issues
	}

	commitType := m[1]
	description := m[3]

	if !conventionalTypes[commitType] {
		issues = append(issues, fmt.Sprintf("unknown commit type %q; use one of: feat, fix, docs, test, chore, refactor, perf, ci, build, style", commitType))
	}

	if len(description) > 0 && description[0] >= 'A' && description[0] <= 'Z' {
		issues = append(issues, "description after colon should start with lowercase letter")
	}

	if strings.TrimSpace(description) == "" {
		issues = append(issues, "description after colon must not be empty")
	}

	if len(lines) > 1 && lines[1] != "" {
		issues = append(issues, "body must be separated from subject by a blank line")
	}

	return issues
}

func suggestCommitMessage(message string) string {
	if message == "" {
		return ""
	}

	lines := strings.Split(message, "\n")
	subject := lines[0]

	m := conventionalCommitRe.FindStringSubmatch(subject)
	if m != nil {
		commitType := m[1]
		scope := m[2]
		desc := m[3]

		desc = strings.TrimRight(desc, ".")

		if len(desc) > 0 && desc[0] >= 'A' && desc[0] <= 'Z' {
			desc = strings.ToLower(desc[:1]) + desc[1:]
		}

		var newSubject string
		if scope != "" {
			newSubject = fmt.Sprintf("%s(%s): %s", commitType, scope, desc)
		} else {
			newSubject = fmt.Sprintf("%s: %s", commitType, desc)
		}

		if len(newSubject) > 72 {
			newSubject = newSubject[:69] + "..."
		}

		lines[0] = newSubject

		if len(lines) > 1 && lines[1] != "" {
			lines = append(lines[:1], append([]string{""}, lines[1:]...)...)
		}

		return strings.Join(lines, "\n")
	}

	return message
}

// ── Git operations ──

// Status returns the git status for a directory.
func Status(ctx context.Context, dir string) (*StatusResult, error) {
	if dir == "" {
		return nil, fmt.Errorf("project_dir required")
	}

	resp := &StatusResult{}

	if branch, err := git(ctx, dir, "rev-parse", "--abbrev-ref", "HEAD"); err == nil {
		resp.Branch = strings.TrimSpace(branch)
	}

	if upstream, err := git(ctx, dir, "rev-parse", "--abbrev-ref", "@{u}"); err == nil {
		resp.Upstream = strings.TrimSpace(upstream)
	}

	if counts, err := git(ctx, dir, "rev-list", "--left-right", "--count", "HEAD...@{u}"); err == nil {
		parts := strings.Fields(strings.TrimSpace(counts))
		if len(parts) == 2 {
			resp.Ahead, _ = strconv.Atoi(parts[0])
			resp.Behind, _ = strconv.Atoi(parts[1])
		}
	}

	out, err := git(ctx, dir, "status", "--porcelain=v1", "-z")
	if err != nil {
		return resp, nil //nolint:nilerr // graceful degradation on git errors
	}

	entries := strings.SplitSeq(out, "\x00")
	for entry := range entries {
		if len(entry) < 4 {
			continue
		}

		x := entry[0]
		y := entry[1]
		path := entry[3:]

		if x == '?' {
			resp.Untracked = append(resp.Untracked, path)
			continue
		}

		if x != ' ' && x != '?' {
			resp.Staged = append(resp.Staged, FileStatus{
				Path:   path,
				Status: statusChar(x),
			})
		}

		if y != ' ' && y != '?' {
			resp.Unstaged = append(resp.Unstaged, FileStatus{
				Path:   path,
				Status: statusChar(y),
			})
		}
	}

	resp.Clean = len(resp.Staged) == 0 && len(resp.Unstaged) == 0 && len(resp.Untracked) == 0

	return resp, nil
}

func statusChar(c byte) string {
	switch c {
	case 'M':
		return "modified"
	case 'A':
		return "added"
	case 'D':
		return "deleted"
	case 'R':
		return "renamed"
	case 'C':
		return "copied"
	default:
		return string(c)
	}
}

// DiffOptions controls git diff behavior.
type DiffOptions struct {
	Staged       bool
	Ref          string
	ContextLines int
	StatOnly     bool
	Paths        []string
}

var statSummaryRe = regexp.MustCompile(`(\d+) files? changed(?:, (\d+) insertions?\(\+\))?(?:, (\d+) deletions?\(-\))?`)

// Diff runs git diff in the given directory.
func Diff(ctx context.Context, dir string, opts DiffOptions) (*DiffResult, error) {
	if dir == "" {
		return nil, fmt.Errorf("project_dir required")
	}

	args := []string{"diff"}
	if opts.Staged {
		args = append(args, "--staged")
	}

	if opts.Ref != "" {
		args = append(args, opts.Ref)
	}

	if opts.ContextLines > 0 {
		args = append(args, fmt.Sprintf("-U%d", opts.ContextLines))
	}

	if opts.StatOnly {
		args = append(args, "--stat")
	}

	args = append(args, "--")
	args = append(args, opts.Paths...)

	diff, err := git(ctx, dir, args...)
	if err != nil {
		return nil, err
	}

	resp := &DiffResult{Diff: diff}

	if !opts.StatOnly {
		statArgs := []string{"diff", "--stat"}
		if opts.Staged {
			statArgs = append(statArgs, "--staged")
		}

		if opts.Ref != "" {
			statArgs = append(statArgs, opts.Ref)
		}

		statArgs = append(statArgs, "--")

		statArgs = append(statArgs, opts.Paths...)
		if stat, err := git(ctx, dir, statArgs...); err == nil {
			resp.Stat = stat
			parseStatSummary(stat, resp)
		}
	} else {
		parseStatSummary(diff, resp)
	}

	return resp, nil
}

func parseStatSummary(stat string, resp *DiffResult) {
	m := statSummaryRe.FindStringSubmatch(stat)
	if m == nil {
		return
	}

	resp.FilesChanged, _ = strconv.Atoi(m[1])
	resp.Insertions, _ = strconv.Atoi(m[2])
	resp.Deletions, _ = strconv.Atoi(m[3])
}

// LogOptions controls git log behavior.
type LogOptions struct {
	Limit  int
	Since  string
	Until  string
	Author string
	Ref    string
	Paths  []string
}

// Log runs git log in the given directory.
func Log(ctx context.Context, dir string, opts LogOptions) (*LogResult, error) {
	if dir == "" {
		return nil, fmt.Errorf("project_dir required")
	}

	limit := opts.Limit
	if limit <= 0 {
		limit = 20
	}

	const sep = "§§§"

	format := strings.Join([]string{"%H", "%h", "%an", "%ae", "%aI", "%s", "%b"}, sep)

	args := []string{"log", fmt.Sprintf("-n%d", limit), fmt.Sprintf("--format=%s", format)}
	if opts.Since != "" {
		args = append(args, "--since="+opts.Since)
	}

	if opts.Until != "" {
		args = append(args, "--until="+opts.Until)
	}

	if opts.Author != "" {
		args = append(args, "--author="+opts.Author)
	}

	if opts.Ref != "" {
		args = append(args, opts.Ref)
	}

	if len(opts.Paths) > 0 {
		args = append(args, "--")
		args = append(args, opts.Paths...)
	}

	out, err := git(ctx, dir, args...)
	if err != nil {
		return nil, err
	}

	resp := &LogResult{}

	for line := range strings.SplitSeq(strings.TrimSpace(out), "\n") {
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, sep, 7)
		if len(parts) < 6 {
			continue
		}

		entry := LogEntry{
			Hash:      parts[0],
			ShortHash: parts[1],
			Author:    parts[2],
			Email:     parts[3],
			Date:      parts[4],
			Subject:   parts[5],
		}
		if len(parts) > 6 {
			entry.Body = parts[6]
		}

		resp.Entries = append(resp.Entries, entry)
	}

	resp.Total = len(resp.Entries)

	return resp, nil
}

// BlameOptions controls git blame behavior.
type BlameOptions struct {
	File      string
	Ref       string
	LineStart int
	LineEnd   int
}

// Blame runs git blame in the given directory.
func Blame(ctx context.Context, dir string, opts BlameOptions) (*BlameResult, error) {
	if dir == "" || opts.File == "" {
		return nil, fmt.Errorf("project_dir and file required")
	}

	args := []string{"blame", "--porcelain"}
	if opts.LineStart > 0 {
		args = append(args, fmt.Sprintf("-L%d,%d", opts.LineStart, opts.LineEnd))
	}

	if opts.Ref != "" {
		args = append(args, opts.Ref)
	}

	args = append(args, "--", opts.File)

	out, err := git(ctx, dir, args...)
	if err != nil {
		return nil, err
	}

	resp := &BlameResult{}

	var current *BlameLine

	lineNum := 0

	for line := range strings.SplitSeq(out, "\n") {
		if len(line) == 0 {
			continue
		}

		if line[0] == '\t' {
			if current != nil {
				current.Content = line[1:]
				resp.Lines = append(resp.Lines, *current)
				current = nil
			}

			continue
		}

		fields := strings.Fields(line)
		if len(fields) >= 3 && len(fields[0]) == 40 {
			lineNum++
			bl := BlameLine{Hash: fields[0], LineNumber: lineNum}
			current = &bl

			continue
		}

		if current == nil {
			continue
		}

		if after, ok := strings.CutPrefix(line, "author "); ok {
			current.Author = after
		} else if after, ok := strings.CutPrefix(line, "author-time "); ok {
			if n, err := strconv.ParseInt(after, 10, 64); err == nil {
				current.Date = time.Unix(n, 0).Format("2006-01-02")
			}
		}
	}

	return resp, nil
}

// BranchListOptions controls git branch list behavior.
type BranchListOptions struct {
	All    bool
	Remote bool
}

// BranchList lists git branches.
func BranchList(ctx context.Context, dir string, opts BranchListOptions) (*BranchListResult, error) {
	if dir == "" {
		return nil, fmt.Errorf("project_dir required")
	}

	args := []string{"branch", "--format=%(HEAD)§%(refname:short)§%(objectname:short)§%(upstream:short)§%(upstream:track,nobracket)"}
	if opts.All {
		args = append(args[:1], append([]string{"-a"}, args[1:]...)...)
	} else if opts.Remote {
		args = append(args[:1], append([]string{"-r"}, args[1:]...)...)
	}

	out, err := git(ctx, dir, args...)
	if err != nil {
		return nil, err
	}

	resp := &BranchListResult{}

	for line := range strings.SplitSeq(strings.TrimSpace(out), "\n") {
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, "§", 5)
		if len(parts) < 4 {
			continue
		}

		branch := Branch{
			Name:     parts[1],
			Hash:     parts[2],
			Upstream: parts[3],
			Current:  parts[0] == "*",
		}
		if branch.Current {
			resp.Current = branch.Name
		}

		if len(parts) > 4 && parts[4] != "" {
			parseTrackInfo(parts[4], &branch)
		}

		resp.Branches = append(resp.Branches, branch)
	}

	return resp, nil
}

var aheadRe = regexp.MustCompile(`ahead (\d+)`)
var behindRe = regexp.MustCompile(`behind (\d+)`)

func parseTrackInfo(track string, b *Branch) {
	if m := aheadRe.FindStringSubmatch(track); m != nil {
		b.Ahead, _ = strconv.Atoi(m[1])
	}

	if m := behindRe.FindStringSubmatch(track); m != nil {
		b.Behind, _ = strconv.Atoi(m[1])
	}
}

// CommitOptions controls git commit behavior.
type CommitOptions struct {
	Message string
	Files   []string
	All     bool
}

// Commit creates a git commit.
func Commit(ctx context.Context, dir string, opts CommitOptions) (*CommitResult, error) {
	if dir == "" {
		return nil, fmt.Errorf("project_dir required")
	}

	if opts.Message == "" {
		return nil, fmt.Errorf("commit message required")
	}

	if len(opts.Files) > 0 {
		args := append([]string{"add", "--"}, opts.Files...)
		if _, err := git(ctx, dir, args...); err != nil {
			return nil, fmt.Errorf("git add: %w", err)
		}
	}

	commitArgs := []string{"commit"}
	if opts.All {
		commitArgs = append(commitArgs, "-a")
	}

	commitArgs = append(commitArgs, "-m", opts.Message)

	out, err := git(ctx, dir, commitArgs...)
	if err != nil {
		return nil, err
	}

	resp := &CommitResult{
		Subject: strings.SplitN(opts.Message, "\n", 2)[0],
		Success: true,
	}

	if hash, err := git(ctx, dir, "rev-parse", "HEAD"); err == nil {
		resp.Hash = strings.TrimSpace(hash)
		if len(resp.Hash) >= 8 {
			resp.ShortHash = resp.Hash[:8]
		}
	}

	if m := statSummaryRe.FindStringSubmatch(out); m != nil {
		resp.FilesChanged, _ = strconv.Atoi(m[1])
		resp.Insertions, _ = strconv.Atoi(m[2])
		resp.Deletions, _ = strconv.Atoi(m[3])
	}

	return resp, nil
}

// ChangelogOptions controls changelog generation.
type ChangelogOptions struct {
	Since       string
	Until       string
	GroupByType bool
}

// Changelog generates a changelog from git history.
func Changelog(ctx context.Context, dir string, opts ChangelogOptions) (*ChangelogResult, error) {
	if dir == "" {
		return nil, fmt.Errorf("project_dir required")
	}

	const sep = "§"

	format := strings.Join([]string{"%H", "%h", "%s", "%an", "%aI"}, sep)

	args := []string{"log", fmt.Sprintf("--format=%s", format)}

	until := opts.Until
	if until == "" {
		until = "HEAD"
	}

	if opts.Since != "" {
		args = append(args, opts.Since+".."+until)
	} else {
		args = append(args, until)
	}

	out, err := git(ctx, dir, args...)
	if err != nil {
		return nil, err
	}

	type commit struct {
		hash, shortHash, subject, author, date string
	}

	var commits []commit

	for line := range strings.SplitSeq(strings.TrimSpace(out), "\n") {
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, sep, 5)
		if len(parts) < 5 {
			continue
		}

		commits = append(commits, commit{
			hash: parts[0], shortHash: parts[1], subject: parts[2], author: parts[3], date: parts[4],
		})
	}

	typeLabels := map[string]string{
		"feat": "Features", "fix": "Bug Fixes", "refactor": "Refactoring",
		"docs": "Documentation", "test": "Tests", "chore": "Chores",
		"perf": "Performance", "ci": "CI", "style": "Style", "build": "Build",
	}

	byType := make(map[string]int)
	grouped := make(map[string][]commit)

	for _, c := range commits {
		ctype := "other"

		subj := c.subject
		if idx := strings.Index(subj, ":"); idx > 0 {
			prefix := strings.TrimSpace(subj[:idx])
			if pi := strings.Index(prefix, "("); pi > 0 {
				prefix = prefix[:pi]
			}

			if _, ok := typeLabels[prefix]; ok {
				ctype = prefix
			}
		}

		byType[ctype]++
		grouped[ctype] = append(grouped[ctype], c)
	}

	var sb strings.Builder
	sb.WriteString("# Changelog\n")

	if opts.GroupByType {
		order := []string{"feat", "fix", "refactor", "perf", "docs", "test", "chore", "ci", "style", "build", "other"}
		for _, t := range order {
			cs, ok := grouped[t]
			if !ok {
				continue
			}

			label := typeLabels[t]
			if label == "" {
				label = "Other"
			}

			fmt.Fprintf(&sb, "\n## %s\n", label)

			for _, c := range cs {
				dateShort := c.date
				if len(dateShort) >= 10 {
					dateShort = dateShort[:10]
				}

				fmt.Fprintf(&sb, "- %s %s (%s, %s)\n", c.shortHash, c.subject, c.author, dateShort)
			}
		}
	} else {
		sb.WriteString("\n")

		for _, c := range commits {
			dateShort := c.date
			if len(dateShort) >= 10 {
				dateShort = dateShort[:10]
			}

			fmt.Fprintf(&sb, "- %s %s (%s, %s)\n", c.shortHash, c.subject, c.author, dateShort)
		}
	}

	return &ChangelogResult{
		Changelog:    sb.String(),
		TotalCommits: len(commits),
		ByType:       byType,
	}, nil
}

// MergeOptions controls git merge behavior.
type MergeOptions struct {
	Branch  string
	NoFf    bool
	Squash  bool
	Message string
	Abort   bool
}

// Merge performs a git merge.
func Merge(ctx context.Context, dir string, opts MergeOptions) (*MergeResult, error) {
	if dir == "" {
		return nil, fmt.Errorf("project_dir required")
	}

	if opts.Abort {
		out, err := git(ctx, dir, "merge", "--abort")
		if err != nil {
			return nil, err
		}

		return &MergeResult{Success: true, Output: strings.TrimSpace(out)}, nil
	}

	if opts.Branch == "" {
		return nil, fmt.Errorf("branch required")
	}

	args := []string{"merge"}
	if opts.NoFf {
		args = append(args, "--no-ff")
	}

	if opts.Squash {
		args = append(args, "--squash")
	}

	if opts.Message != "" {
		args = append(args, "-m", opts.Message)
	}

	args = append(args, opts.Branch)

	resp := &MergeResult{}

	out, err := git(ctx, dir, args...)
	if err != nil {
		errMsg := err.Error()

		resp.Output = errMsg
		if strings.Contains(errMsg, "CONFLICT") || strings.Contains(errMsg, "conflict") {
			resp.HasConflicts = true

			if conflictOut, cerr := git(ctx, dir, "diff", "--name-only", "--diff-filter=U"); cerr == nil {
				for line := range strings.SplitSeq(strings.TrimSpace(conflictOut), "\n") {
					if line != "" {
						resp.ConflictFiles = append(resp.ConflictFiles, line)
					}
				}
			}

			return resp, nil
		}

		return nil, err
	}

	resp.Success = true
	resp.Output = strings.TrimSpace(out)

	if hash, herr := git(ctx, dir, "rev-parse", "HEAD"); herr == nil {
		resp.MergeHash = strings.TrimSpace(hash)
	}

	return resp, nil
}

// RebaseOptions controls git rebase behavior.
type RebaseOptions struct {
	Onto     string
	Abort    bool
	Continue bool
	Skip     bool
}

// Rebase performs a git rebase.
func Rebase(ctx context.Context, dir string, opts RebaseOptions) (*RebaseResult, error) {
	if dir == "" {
		return nil, fmt.Errorf("project_dir required")
	}

	var args []string

	switch {
	case opts.Abort:
		args = []string{"rebase", "--abort"}
	case opts.Continue:
		args = []string{"rebase", "--continue"}
	case opts.Skip:
		args = []string{"rebase", "--skip"}
	default:
		if opts.Onto == "" {
			return nil, fmt.Errorf("onto ref required for rebase")
		}

		args = []string{"rebase", opts.Onto}
	}

	resp := &RebaseResult{}

	out, err := git(ctx, dir, args...)
	if err != nil {
		errMsg := err.Error()

		resp.Output = errMsg
		if strings.Contains(errMsg, "CONFLICT") || strings.Contains(errMsg, "conflict") {
			resp.HasConflicts = true

			if conflictOut, cerr := git(ctx, dir, "diff", "--name-only", "--diff-filter=U"); cerr == nil {
				for line := range strings.SplitSeq(strings.TrimSpace(conflictOut), "\n") {
					if line != "" {
						resp.ConflictFiles = append(resp.ConflictFiles, line)
					}
				}
			}
		}
	} else {
		resp.Success = true
		resp.Output = strings.TrimSpace(out)
	}

	if gitDir, gerr := git(ctx, dir, "rev-parse", "--git-dir"); gerr == nil {
		gitDirPath := strings.TrimSpace(gitDir)
		if !filepath.IsAbs(gitDirPath) {
			gitDirPath = filepath.Join(dir, gitDirPath)
		}

		rebaseMerge := filepath.Join(gitDirPath, "rebase-merge")

		rebaseApply := filepath.Join(gitDirPath, "rebase-apply")
		if info, serr := os.Stat(rebaseMerge); serr == nil && info.IsDir() {
			resp.InProgress = true
		} else if info, serr := os.Stat(rebaseApply); serr == nil && info.IsDir() {
			resp.InProgress = true
		}
	}

	if err != nil && !resp.HasConflicts {
		return nil, err
	}

	return resp, nil
}

// ListConflicts returns files and hunks with merge conflicts.
func ListConflicts(ctx context.Context, dir string) (*ListConflictsResult, error) {
	if dir == "" {
		return nil, fmt.Errorf("project_dir required")
	}

	resp := &ListConflictsResult{}

	out, err := git(ctx, dir, "diff", "--name-only", "--diff-filter=U")
	if err != nil {
		return resp, nil //nolint:nilerr // graceful degradation on git errors
	}

	for line := range strings.SplitSeq(strings.TrimSpace(out), "\n") {
		if line != "" {
			resp.Files = append(resp.Files, line)
		}
	}

	for _, file := range resp.Files {
		absPath := filepath.Join(dir, file)

		data, rerr := os.ReadFile(absPath)
		if rerr != nil {
			continue
		}

		hunks := parseConflictMarkers(string(data), file)
		resp.Hunks = append(resp.Hunks, hunks...)
	}

	resp.TotalConflicts = len(resp.Hunks)

	gitDir, gerr := git(ctx, dir, "rev-parse", "--git-dir")
	if gerr == nil {
		gitDirPath := strings.TrimSpace(gitDir)
		if !filepath.IsAbs(gitDirPath) {
			gitDirPath = filepath.Join(dir, gitDirPath)
		}

		rebaseMerge := filepath.Join(gitDirPath, "rebase-merge")
		rebaseApply := filepath.Join(gitDirPath, "rebase-apply")

		mergeHead := filepath.Join(gitDirPath, "MERGE_HEAD")
		if info, serr := os.Stat(rebaseMerge); serr == nil && info.IsDir() {
			resp.MergeType = "rebase"
		} else if info, serr := os.Stat(rebaseApply); serr == nil && info.IsDir() {
			resp.MergeType = "rebase"
		} else if _, serr := os.Stat(mergeHead); serr == nil {
			resp.MergeType = "merge"
		}
	}

	return resp, nil
}

func parseConflictMarkers(content string, filename string) []ConflictHunk {
	var hunks []ConflictHunk

	lines := strings.Split(content, "\n")

	var (
		inConflict bool
		section    string
		startLine  int
		ours       strings.Builder
		theirs     strings.Builder
		ancestor   strings.Builder
	)

	for i, line := range lines {
		lineNum := i + 1

		if strings.HasPrefix(line, "<<<<<<<") {
			inConflict = true
			section = "ours"
			startLine = lineNum

			ours.Reset()
			theirs.Reset()
			ancestor.Reset()

			continue
		}

		if !inConflict {
			continue
		}

		if strings.HasPrefix(line, "|||||||") {
			section = "ancestor"
			continue
		}

		if strings.TrimRight(line, "\r") == "=======" {
			section = "theirs"
			continue
		}

		if strings.HasPrefix(line, ">>>>>>>") {
			hunks = append(hunks, ConflictHunk{
				File:      filename,
				StartLine: startLine,
				EndLine:   lineNum,
				Ours:      ours.String(),
				Theirs:    theirs.String(),
				Ancestor:  ancestor.String(),
			})
			inConflict = false
			section = ""

			continue
		}

		switch section {
		case "ours":
			if ours.Len() > 0 {
				ours.WriteByte('\n')
			}

			ours.WriteString(line)
		case "ancestor":
			if ancestor.Len() > 0 {
				ancestor.WriteByte('\n')
			}

			ancestor.WriteString(line)
		case "theirs":
			if theirs.Len() > 0 {
				theirs.WriteByte('\n')
			}

			theirs.WriteString(line)
		}
	}

	return hunks
}

// StashOptions controls git stash behavior.
type StashOptions struct {
	Action  string // "list", "show", "save", "pop", "drop"
	Index   int
	Message string
}

// Stash performs git stash operations.
func Stash(ctx context.Context, dir string, opts StashOptions) (*StashResult, error) {
	if dir == "" {
		return nil, fmt.Errorf("project_dir required")
	}

	resp := &StashResult{}

	action := opts.Action
	if action == "" {
		action = "list"
	}

	switch action {
	case "list":
		out, err := git(ctx, dir, "stash", "list")
		if err != nil {
			return nil, err
		}

		for line := range strings.SplitSeq(strings.TrimSpace(out), "\n") {
			if line == "" {
				continue
			}

			entry := StashEntry{}

			if idx := strings.Index(line, ": "); idx > 0 {
				ref := line[:idx]
				rest := line[idx+2:]

				if n, err := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(ref, "stash@{"), "}")); err == nil {
					entry.Index = n
				}

				entry.Message = rest
			}

			resp.Entries = append(resp.Entries, entry)
		}

	case "show":
		idx := fmt.Sprintf("stash@{%d}", opts.Index)

		out, err := git(ctx, dir, "stash", "show", "-p", idx)
		if err != nil {
			return nil, err
		}

		resp.Diff = out

	case "save":
		args := []string{"stash", "push"}
		if opts.Message != "" {
			args = append(args, "-m", opts.Message)
		}

		out, err := git(ctx, dir, args...)
		if err != nil {
			return nil, err
		}

		resp.Success = true
		resp.Output = out

	case "pop":
		idx := fmt.Sprintf("stash@{%d}", opts.Index)

		out, err := git(ctx, dir, "stash", "pop", idx)
		if err != nil {
			return nil, err
		}

		resp.Success = true
		resp.Output = out

	case "drop":
		idx := fmt.Sprintf("stash@{%d}", opts.Index)

		out, err := git(ctx, dir, "stash", "drop", idx)
		if err != nil {
			return nil, err
		}

		resp.Success = true
		resp.Output = out

	default:
		return nil, fmt.Errorf("unknown stash action: %s (use list, show, save, pop, drop)", action)
	}

	return resp, nil
}
