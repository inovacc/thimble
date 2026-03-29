package gitops

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// initTestRepo creates a git repo in t.TempDir with user config set.
func initTestRepo(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()

	run(t, dir, "git", "init")
	run(t, dir, "git", "config", "user.name", "Test User")
	run(t, dir, "git", "config", "user.email", "test@example.com")

	return dir
}

// initTestRepoWithCommit creates a repo with one initial commit.
func initTestRepoWithCommit(t *testing.T) string {
	t.Helper()

	dir := initTestRepo(t)
	writeFile(t, dir, "README.md", "# Test\n")
	run(t, dir, "git", "add", "README.md")
	run(t, dir, "git", "commit", "-m", "feat: initial commit")

	return dir
}

func run(t *testing.T, dir string, name string, args ...string) string { //nolint:unparam // name kept generic for test flexibility
	t.Helper()

	cmd := exec.Command(name, args...)
	cmd.Dir = dir

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v failed: %v\n%s", name, args, err, out)
	}

	return string(out)
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()

	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// ── ValidateBranch ──

func TestValidateBranch(t *testing.T) {
	tests := []struct {
		name       string
		branch     string
		wantValid  bool
		wantSubstr string // substring expected in Suggestion (if invalid)
	}{
		// Valid names
		{name: "feature prefix", branch: "feat/add-login", wantValid: true},
		{name: "fix prefix", branch: "fix/bug-123", wantValid: true},
		{name: "main branch", branch: "main", wantValid: true},
		{name: "master branch", branch: "master", wantValid: true},
		{name: "develop branch", branch: "develop", wantValid: true},
		{name: "docs prefix", branch: "docs/update-readme", wantValid: true},
		{name: "chore prefix", branch: "chore/cleanup", wantValid: true},
		{name: "refactor prefix", branch: "refactor/split-module", wantValid: true},
		{name: "release prefix", branch: "release/v1.2.3", wantValid: true},
		{name: "hotfix prefix", branch: "hotfix/urgent-fix", wantValid: true},
		{name: "test prefix", branch: "test/add-unit-tests", wantValid: true},

		// Invalid names
		{name: "empty string", branch: "", wantValid: false, wantSubstr: "must not be empty"},
		{name: "has spaces", branch: "my branch", wantValid: false, wantSubstr: "invalid characters"},
		{name: "has double dots", branch: "feat/a..b", wantValid: false, wantSubstr: "invalid characters"},
		{name: "has tilde", branch: "feat/a~b", wantValid: false, wantSubstr: "invalid characters"},
		{name: "has colon", branch: "feat/a:b", wantValid: false, wantSubstr: "invalid characters"},
		{name: "uppercase chars", branch: "Feature/Add-Login", wantValid: false, wantSubstr: "invalid characters"},
		{name: "no conventional prefix", branch: "add-login", wantValid: false, wantSubstr: "conventional prefix"},
		{name: "empty after prefix", branch: "feat/", wantValid: false, wantSubstr: "invalid characters"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateBranch(tt.branch)
			if result.Valid != tt.wantValid {
				t.Errorf("ValidateBranch(%q).Valid = %v, want %v (suggestion: %s)", tt.branch, result.Valid, tt.wantValid, result.Suggestion)
			}

			if !tt.wantValid && tt.wantSubstr != "" {
				if !strings.Contains(result.Suggestion, tt.wantSubstr) {
					t.Errorf("ValidateBranch(%q).Suggestion = %q, want substring %q", tt.branch, result.Suggestion, tt.wantSubstr)
				}
			}
		})
	}
}

func TestSuggestBranchName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"My Feature", "my-feature"},
		{"hello_world", "hello-world"},
		{"a..b", "a..b"},
		{"", "my-branch"},
		{"feat/good-name", "feat/good-name"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := suggestBranchName(tt.input)
			if got != tt.want {
				t.Errorf("suggestBranchName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ── LintCommit ──

func TestLintCommit(t *testing.T) {
	tests := []struct {
		name      string
		message   string
		wantValid bool
		wantIssue string // substring in Issues (if invalid)
	}{
		// Valid messages
		{name: "valid feat", message: "feat: add login", wantValid: true},
		{name: "valid fix with scope", message: "fix(auth): resolve token issue", wantValid: true},
		{name: "valid docs", message: "docs: update README", wantValid: true},
		{name: "valid docs lower", message: "docs: update readme", wantValid: true},
		{name: "valid chore", message: "chore: bump deps", wantValid: true},
		{name: "valid with body", message: "feat: add login\n\nThis adds a login page.", wantValid: true},

		// Invalid messages
		{name: "empty message", message: "", wantValid: false, wantIssue: "must not be empty"},
		{name: "no type prefix", message: "add login feature", wantValid: false, wantIssue: "conventional format"},
		{name: "too long subject", message: "feat: " + strings.Repeat("a", 80), wantValid: false, wantIssue: "should be"},
		{name: "ends with period", message: "feat: add login.", wantValid: false, wantIssue: "period"},
		{name: "unknown type", message: "yolo: do stuff", wantValid: false, wantIssue: "unknown commit type"},
		{name: "empty description", message: "feat: ", wantValid: false, wantIssue: "must not be empty"},
		{name: "body no blank line", message: "feat: add login\nsome body text", wantValid: false, wantIssue: "blank line"},
		{name: "uppercase description", message: "feat: Add login", wantValid: false, wantIssue: "lowercase"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := LintCommit(tt.message)
			if result.Valid != tt.wantValid {
				t.Errorf("LintCommit(%q).Valid = %v, want %v (issues: %v)", tt.message, result.Valid, tt.wantValid, result.Issues)
			}

			if !tt.wantValid && tt.wantIssue != "" {
				found := false

				for _, issue := range result.Issues {
					if strings.Contains(issue, tt.wantIssue) {
						found = true
						break
					}
				}

				if !found {
					t.Errorf("LintCommit(%q).Issues = %v, want issue containing %q", tt.message, result.Issues, tt.wantIssue)
				}
			}
		})
	}
}

func TestLintCommitSuggestion(t *testing.T) {
	result := LintCommit("feat: Add login.")
	if result.Suggested == "" {
		t.Fatal("expected a suggestion for invalid message")
	}

	if strings.HasSuffix(result.Suggested, ".") {
		t.Errorf("suggestion should not end with period, got %q", result.Suggested)
	}
}

// ── Status ──

func TestStatus_CleanRepo(t *testing.T) {
	dir := initTestRepoWithCommit(t)
	ctx := context.Background()

	result, err := Status(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}

	if !result.Clean {
		t.Error("expected clean repo")
	}

	if result.Branch != "main" && result.Branch != "master" {
		t.Errorf("unexpected branch: %s", result.Branch)
	}
}

func TestStatus_WithChanges(t *testing.T) {
	dir := initTestRepoWithCommit(t)
	ctx := context.Background()

	// Create untracked file
	writeFile(t, dir, "untracked.txt", "hello")

	// Modify tracked file (unstaged)
	writeFile(t, dir, "README.md", "# Modified\n")

	// Create and stage a file
	writeFile(t, dir, "staged.txt", "staged content")
	run(t, dir, "git", "add", "staged.txt")

	result, err := Status(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}

	if result.Clean {
		t.Error("expected dirty repo")
	}

	if len(result.Untracked) != 1 || result.Untracked[0] != "untracked.txt" {
		t.Errorf("untracked = %v, want [untracked.txt]", result.Untracked)
	}

	if len(result.Staged) != 1 {
		t.Errorf("staged = %d, want 1", len(result.Staged))
	}

	if len(result.Unstaged) != 1 {
		t.Errorf("unstaged = %d, want 1", len(result.Unstaged))
	}
}

func TestStatus_EmptyDir(t *testing.T) {
	_, err := Status(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty dir")
	}
}

// ── Diff ──

func TestDiff_NoChanges(t *testing.T) {
	dir := initTestRepoWithCommit(t)
	ctx := context.Background()

	result, err := Diff(ctx, dir, DiffOptions{})
	if err != nil {
		t.Fatal(err)
	}

	if result.Diff != "" {
		t.Errorf("expected empty diff, got %q", result.Diff)
	}
}

func TestDiff_UnstagedChanges(t *testing.T) {
	dir := initTestRepoWithCommit(t)
	ctx := context.Background()

	writeFile(t, dir, "README.md", "# Changed\nNew content\n")

	result, err := Diff(ctx, dir, DiffOptions{})
	if err != nil {
		t.Fatal(err)
	}

	if result.Diff == "" {
		t.Fatal("expected non-empty diff")
	}

	if !strings.Contains(result.Diff, "Changed") {
		t.Error("diff should contain changed content")
	}

	if result.FilesChanged != 1 {
		t.Errorf("FilesChanged = %d, want 1", result.FilesChanged)
	}
}

func TestDiff_StagedChanges(t *testing.T) {
	dir := initTestRepoWithCommit(t)
	ctx := context.Background()

	writeFile(t, dir, "README.md", "# Staged change\n")
	run(t, dir, "git", "add", "README.md")

	// Unstaged diff should be empty (changes are staged)
	unstaged, err := Diff(ctx, dir, DiffOptions{})
	if err != nil {
		t.Fatal(err)
	}

	if unstaged.Diff != "" {
		t.Error("expected empty unstaged diff")
	}

	// Staged diff should show changes
	staged, err := Diff(ctx, dir, DiffOptions{Staged: true})
	if err != nil {
		t.Fatal(err)
	}

	if staged.Diff == "" {
		t.Fatal("expected non-empty staged diff")
	}
}

func TestDiff_EmptyDir(t *testing.T) {
	_, err := Diff(context.Background(), "", DiffOptions{})
	if err == nil {
		t.Fatal("expected error for empty dir")
	}
}

// ── Log ──

func TestLog_ReturnsCommits(t *testing.T) {
	dir := initTestRepoWithCommit(t)
	ctx := context.Background()

	// Add more commits
	writeFile(t, dir, "a.txt", "a")
	run(t, dir, "git", "add", "a.txt")
	run(t, dir, "git", "commit", "-m", "feat: second commit")

	writeFile(t, dir, "b.txt", "b")
	run(t, dir, "git", "add", "b.txt")
	run(t, dir, "git", "commit", "-m", "fix: third commit")

	result, err := Log(ctx, dir, LogOptions{})
	if err != nil {
		t.Fatal(err)
	}

	if result.Total != 3 {
		t.Errorf("Total = %d, want 3", result.Total)
	}

	// Most recent first
	if result.Entries[0].Subject != "fix: third commit" {
		t.Errorf("first entry subject = %q, want %q", result.Entries[0].Subject, "fix: third commit")
	}

	// Check fields are populated
	for i, e := range result.Entries {
		if e.Hash == "" {
			t.Errorf("entry[%d].Hash is empty", i)
		}

		if e.ShortHash == "" {
			t.Errorf("entry[%d].ShortHash is empty", i)
		}

		if e.Author == "" {
			t.Errorf("entry[%d].Author is empty", i)
		}

		if e.Date == "" {
			t.Errorf("entry[%d].Date is empty", i)
		}
	}
}

func TestLog_LimitOption(t *testing.T) {
	dir := initTestRepoWithCommit(t)
	ctx := context.Background()

	writeFile(t, dir, "a.txt", "a")
	run(t, dir, "git", "add", "a.txt")
	run(t, dir, "git", "commit", "-m", "feat: second")

	writeFile(t, dir, "b.txt", "b")
	run(t, dir, "git", "add", "b.txt")
	run(t, dir, "git", "commit", "-m", "feat: third")

	result, err := Log(ctx, dir, LogOptions{Limit: 2})
	if err != nil {
		t.Fatal(err)
	}

	if result.Total != 2 {
		t.Errorf("Total = %d, want 2", result.Total)
	}
}

func TestLog_EmptyDir(t *testing.T) {
	_, err := Log(context.Background(), "", LogOptions{})
	if err == nil {
		t.Fatal("expected error for empty dir")
	}
}

// ── Commit ──

func TestCommit_Success(t *testing.T) {
	dir := initTestRepoWithCommit(t)
	ctx := context.Background()

	writeFile(t, dir, "new.txt", "new content")

	result, err := Commit(ctx, dir, CommitOptions{
		Message: "feat: add new file",
		Files:   []string{"new.txt"},
	})
	if err != nil {
		t.Fatal(err)
	}

	if !result.Success {
		t.Error("expected success")
	}

	if result.Hash == "" {
		t.Error("expected hash")
	}

	if result.ShortHash == "" {
		t.Error("expected short hash")
	}

	if result.Subject != "feat: add new file" {
		t.Errorf("Subject = %q, want %q", result.Subject, "feat: add new file")
	}
}

func TestCommit_EmptyMessage(t *testing.T) {
	dir := initTestRepoWithCommit(t)
	ctx := context.Background()

	_, err := Commit(ctx, dir, CommitOptions{Message: ""})
	if err == nil {
		t.Fatal("expected error for empty message")
	}

	if !strings.Contains(err.Error(), "message required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCommit_EmptyDir(t *testing.T) {
	_, err := Commit(context.Background(), "", CommitOptions{Message: "test"})
	if err == nil {
		t.Fatal("expected error for empty dir")
	}
}

func TestCommit_WithAll(t *testing.T) {
	dir := initTestRepoWithCommit(t)
	ctx := context.Background()

	// Modify existing tracked file
	writeFile(t, dir, "README.md", "# Modified\n")

	result, err := Commit(ctx, dir, CommitOptions{
		Message: "feat: modify readme",
		All:     true,
	})
	if err != nil {
		t.Fatal(err)
	}

	if !result.Success {
		t.Error("expected success")
	}
}

// ── BranchList ──

func TestBranchList_SingleBranch(t *testing.T) {
	dir := initTestRepoWithCommit(t)
	ctx := context.Background()

	result, err := BranchList(ctx, dir, BranchListOptions{})
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Branches) != 1 {
		t.Fatalf("Branches = %d, want 1", len(result.Branches))
	}

	b := result.Branches[0]
	if !b.Current {
		t.Error("expected current branch")
	}

	if b.Hash == "" {
		t.Error("expected hash")
	}

	if result.Current == "" {
		t.Error("expected current branch name")
	}
}

func TestBranchList_MultipleBranches(t *testing.T) {
	dir := initTestRepoWithCommit(t)
	ctx := context.Background()

	run(t, dir, "git", "checkout", "-b", "feat/test-branch")
	run(t, dir, "git", "checkout", "-b", "fix/another-branch")

	result, err := BranchList(ctx, dir, BranchListOptions{})
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Branches) != 3 {
		t.Errorf("Branches = %d, want 3", len(result.Branches))
	}

	if result.Current != "fix/another-branch" {
		t.Errorf("Current = %q, want %q", result.Current, "fix/another-branch")
	}

	// Verify we find all branch names
	names := make(map[string]bool)
	for _, b := range result.Branches {
		names[b.Name] = true
	}

	for _, want := range []string{"feat/test-branch", "fix/another-branch"} {
		if !names[want] {
			t.Errorf("missing branch %q in %v", want, names)
		}
	}
}

func TestBranchList_EmptyDir(t *testing.T) {
	_, err := BranchList(context.Background(), "", BranchListOptions{})
	if err == nil {
		t.Fatal("expected error for empty dir")
	}
}

// ── Stash ──

func TestStash_SaveAndList(t *testing.T) {
	dir := initTestRepoWithCommit(t)
	ctx := context.Background()

	// Make a change to stash
	writeFile(t, dir, "README.md", "# Stashed change\n")

	// Save stash
	result, err := Stash(ctx, dir, StashOptions{Action: "save", Message: "test stash"})
	if err != nil {
		t.Fatal(err)
	}

	if !result.Success {
		t.Error("expected success on save")
	}

	// List stash
	listResult, err := Stash(ctx, dir, StashOptions{Action: "list"})
	if err != nil {
		t.Fatal(err)
	}

	if len(listResult.Entries) != 1 {
		t.Fatalf("Entries = %d, want 1", len(listResult.Entries))
	}

	if !strings.Contains(listResult.Entries[0].Message, "test stash") {
		t.Errorf("stash message = %q, want to contain %q", listResult.Entries[0].Message, "test stash")
	}
}

func TestStash_Pop(t *testing.T) {
	dir := initTestRepoWithCommit(t)
	ctx := context.Background()

	writeFile(t, dir, "README.md", "# Will be stashed\n")

	_, err := Stash(ctx, dir, StashOptions{Action: "save", Message: "pop test"})
	if err != nil {
		t.Fatal(err)
	}

	// Verify working tree is clean after stash
	status, err := Status(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}

	if !status.Clean {
		t.Error("expected clean after stash save")
	}

	// Pop stash
	popResult, err := Stash(ctx, dir, StashOptions{Action: "pop", Index: 0})
	if err != nil {
		t.Fatal(err)
	}

	if !popResult.Success {
		t.Error("expected success on pop")
	}

	// Verify changes are back
	status2, err := Status(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}

	if status2.Clean {
		t.Error("expected dirty repo after pop")
	}
}

func TestStash_EmptyList(t *testing.T) {
	dir := initTestRepoWithCommit(t)
	ctx := context.Background()

	result, err := Stash(ctx, dir, StashOptions{Action: "list"})
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Entries) != 0 {
		t.Errorf("expected empty stash list, got %d", len(result.Entries))
	}
}

func TestStash_DefaultActionIsList(t *testing.T) {
	dir := initTestRepoWithCommit(t)
	ctx := context.Background()

	result, err := Stash(ctx, dir, StashOptions{})
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Entries) != 0 {
		t.Errorf("expected empty stash list, got %d", len(result.Entries))
	}
}

func TestStash_UnknownAction(t *testing.T) {
	dir := initTestRepoWithCommit(t)
	ctx := context.Background()

	_, err := Stash(ctx, dir, StashOptions{Action: "invalid"})
	if err == nil {
		t.Fatal("expected error for unknown action")
	}

	if !strings.Contains(err.Error(), "unknown stash action") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestStash_EmptyDir(t *testing.T) {
	_, err := Stash(context.Background(), "", StashOptions{})
	if err == nil {
		t.Fatal("expected error for empty dir")
	}
}

// ── statusChar ──

func TestStatusChar(t *testing.T) {
	tests := []struct {
		input byte
		want  string
	}{
		{'M', "modified"},
		{'A', "added"},
		{'D', "deleted"},
		{'R', "renamed"},
		{'C', "copied"},
		{'X', "X"},
	}

	for _, tt := range tests {
		t.Run(string(tt.input), func(t *testing.T) {
			if got := statusChar(tt.input); got != tt.want {
				t.Errorf("statusChar(%c) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
