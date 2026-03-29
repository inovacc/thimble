package mcp

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// skipIfNoGit skips the test if git is not on PATH.
func skipIfNoGit(t *testing.T) {
	t.Helper()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found on PATH")
	}
}

// initTempGitRepo creates a temp directory, initialises a git repo with an
// initial commit, and returns the directory path.
func initTempGitRepo(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()

	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test User"},
	}

	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Stdout = os.Stderr
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			t.Fatalf("init repo (%v): %v", args, err)
		}
	}

	// Create an initial commit so HEAD exists.
	readmePath := filepath.Join(dir, "README.md")
	if err := os.WriteFile(readmePath, []byte("# test\n"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}

	for _, args := range [][]string{
		{"git", "add", "."},
		{"git", "commit", "-m", "initial commit"},
	} {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Stdout = os.Stderr
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			t.Fatalf("initial commit (%v): %v", args, err)
		}
	}

	return dir
}

// gitExec runs a git command in the given directory and fails on error.
func gitExec(t *testing.T, dir string, args ...string) {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
}

// ── handleGitStatus integration ──

func TestIntegration_GitStatus_CleanRepo(t *testing.T) {
	skipIfNoGit(t)

	repoDir := initTempGitRepo(t)
	b := newTestBridge(t)
	ctx := context.Background()

	result, _, err := b.handleGitStatus(ctx, nil, gitStatusInput{
		ProjectDir: repoDir,
	})
	if err != nil {
		t.Fatalf("handleGitStatus: %v", err)
	}

	if result.IsError {
		t.Fatalf("handleGitStatus error: %s", resultText(t, result))
	}

	text := resultText(t, result)
	if !strings.Contains(text, "Working tree clean") {
		t.Errorf("expected clean working tree, got: %s", text)
	}
}

func TestIntegration_GitStatus_UntrackedFile(t *testing.T) {
	skipIfNoGit(t)

	repoDir := initTempGitRepo(t)
	b := newTestBridge(t)
	ctx := context.Background()

	// Create an untracked file.
	if err := os.WriteFile(filepath.Join(repoDir, "new.txt"), []byte("new file\n"), 0o644); err != nil {
		t.Fatalf("write new.txt: %v", err)
	}

	result, _, err := b.handleGitStatus(ctx, nil, gitStatusInput{
		ProjectDir: repoDir,
	})
	if err != nil {
		t.Fatalf("handleGitStatus: %v", err)
	}

	text := resultText(t, result)
	if !strings.Contains(text, "Untracked") {
		t.Errorf("expected Untracked section, got: %s", text)
	}

	if !strings.Contains(text, "new.txt") {
		t.Errorf("expected new.txt in output, got: %s", text)
	}
}

func TestIntegration_GitStatus_StagedFile(t *testing.T) {
	skipIfNoGit(t)

	repoDir := initTempGitRepo(t)
	b := newTestBridge(t)
	ctx := context.Background()

	// Create and stage a file.
	if err := os.WriteFile(filepath.Join(repoDir, "staged.txt"), []byte("staged\n"), 0o644); err != nil {
		t.Fatalf("write staged.txt: %v", err)
	}

	gitExec(t, repoDir, "add", "staged.txt")

	result, _, err := b.handleGitStatus(ctx, nil, gitStatusInput{
		ProjectDir: repoDir,
	})
	if err != nil {
		t.Fatalf("handleGitStatus: %v", err)
	}

	text := resultText(t, result)
	if !strings.Contains(text, "Staged") {
		t.Errorf("expected Staged section, got: %s", text)
	}

	if !strings.Contains(text, "staged.txt") {
		t.Errorf("expected staged.txt in output, got: %s", text)
	}
}

// ── handleGitDiff integration ──

func TestIntegration_GitDiff_UnstagedChange(t *testing.T) {
	skipIfNoGit(t)

	repoDir := initTempGitRepo(t)
	b := newTestBridge(t)
	ctx := context.Background()

	// Modify the README (tracked file).
	readmePath := filepath.Join(repoDir, "README.md")
	if err := os.WriteFile(readmePath, []byte("# test\nmodified line\n"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}

	result, _, err := b.handleGitDiff(ctx, nil, gitDiffInput{
		ProjectDir: repoDir,
	})
	if err != nil {
		t.Fatalf("handleGitDiff: %v", err)
	}

	if result.IsError {
		t.Fatalf("handleGitDiff error: %s", resultText(t, result))
	}

	text := resultText(t, result)
	if !strings.Contains(text, "modified line") {
		t.Errorf("expected diff to contain 'modified line', got: %s", text)
	}
}

func TestIntegration_GitDiff_StagedChange(t *testing.T) {
	skipIfNoGit(t)

	repoDir := initTempGitRepo(t)
	b := newTestBridge(t)
	ctx := context.Background()

	// Stage a modification.
	readmePath := filepath.Join(repoDir, "README.md")
	if err := os.WriteFile(readmePath, []byte("# test\nstaged change\n"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}

	gitExec(t, repoDir, "add", "README.md")

	result, _, err := b.handleGitDiff(ctx, nil, gitDiffInput{
		ProjectDir: repoDir,
		Staged:     true,
	})
	if err != nil {
		t.Fatalf("handleGitDiff: %v", err)
	}

	text := resultText(t, result)
	if !strings.Contains(text, "staged change") {
		t.Errorf("expected staged diff to contain 'staged change', got: %s", text)
	}
}

func TestIntegration_GitDiff_NoDifferences(t *testing.T) {
	skipIfNoGit(t)

	repoDir := initTempGitRepo(t)
	b := newTestBridge(t)
	ctx := context.Background()

	result, _, err := b.handleGitDiff(ctx, nil, gitDiffInput{
		ProjectDir: repoDir,
	})
	if err != nil {
		t.Fatalf("handleGitDiff: %v", err)
	}

	text := resultText(t, result)
	if !strings.Contains(text, "No differences") {
		t.Errorf("expected 'No differences found', got: %s", text)
	}
}

func TestIntegration_GitDiff_StatOnly(t *testing.T) {
	skipIfNoGit(t)

	repoDir := initTempGitRepo(t)
	b := newTestBridge(t)
	ctx := context.Background()

	readmePath := filepath.Join(repoDir, "README.md")
	if err := os.WriteFile(readmePath, []byte("# test\nstat only change\n"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}

	result, _, err := b.handleGitDiff(ctx, nil, gitDiffInput{
		ProjectDir: repoDir,
		StatOnly:   true,
	})
	if err != nil {
		t.Fatalf("handleGitDiff: %v", err)
	}

	text := resultText(t, result)
	if !strings.Contains(text, "README.md") {
		t.Errorf("expected stat output to mention README.md, got: %s", text)
	}
}

// ── handleGitLog integration ──

func TestIntegration_GitLog_InitialCommit(t *testing.T) {
	skipIfNoGit(t)

	repoDir := initTempGitRepo(t)
	b := newTestBridge(t)
	ctx := context.Background()

	result, _, err := b.handleGitLog(ctx, nil, gitLogInput{
		ProjectDir: repoDir,
	})
	if err != nil {
		t.Fatalf("handleGitLog: %v", err)
	}

	if result.IsError {
		t.Fatalf("handleGitLog error: %s", resultText(t, result))
	}

	text := resultText(t, result)
	if !strings.Contains(text, "initial commit") {
		t.Errorf("expected 'initial commit' in log, got: %s", text)
	}
}

func TestIntegration_GitLog_MultipleCommits(t *testing.T) {
	skipIfNoGit(t)

	repoDir := initTempGitRepo(t)
	b := newTestBridge(t)
	ctx := context.Background()

	// Create two more commits.
	for i, msg := range []string{"feat: add feature alpha", "fix: resolve bug beta"} {
		fname := filepath.Join(repoDir, strings.ReplaceAll(msg, " ", "_")+".txt")
		if err := os.WriteFile(fname, []byte(msg), 0o644); err != nil {
			t.Fatalf("write file %d: %v", i, err)
		}

		gitExec(t, repoDir, "add", ".")
		gitExec(t, repoDir, "commit", "-m", msg)
	}

	result, _, err := b.handleGitLog(ctx, nil, gitLogInput{
		ProjectDir: repoDir,
		Limit:      10,
	})
	if err != nil {
		t.Fatalf("handleGitLog: %v", err)
	}

	text := resultText(t, result)
	if !strings.Contains(text, "feature alpha") {
		t.Errorf("expected 'feature alpha' in log, got: %s", text)
	}

	if !strings.Contains(text, "bug beta") {
		t.Errorf("expected 'bug beta' in log, got: %s", text)
	}
}

func TestIntegration_GitLog_Oneline(t *testing.T) {
	skipIfNoGit(t)

	repoDir := initTempGitRepo(t)
	b := newTestBridge(t)
	ctx := context.Background()

	result, _, err := b.handleGitLog(ctx, nil, gitLogInput{
		ProjectDir: repoDir,
		Oneline:    true,
	})
	if err != nil {
		t.Fatalf("handleGitLog: %v", err)
	}

	text := resultText(t, result)
	// Oneline format: short hash + subject on one line.
	lines := strings.Split(strings.TrimSpace(text), "\n")
	if len(lines) < 1 {
		t.Fatal("expected at least one log line")
	}

	if !strings.Contains(lines[0], "initial commit") {
		t.Errorf("expected oneline to contain 'initial commit', got: %s", lines[0])
	}
}

func TestIntegration_GitLog_LimitOne(t *testing.T) {
	skipIfNoGit(t)

	repoDir := initTempGitRepo(t)
	b := newTestBridge(t)
	ctx := context.Background()

	// Add a second commit.
	if err := os.WriteFile(filepath.Join(repoDir, "extra.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatalf("write extra.txt: %v", err)
	}

	gitExec(t, repoDir, "add", ".")
	gitExec(t, repoDir, "commit", "-m", "second commit")

	result, _, err := b.handleGitLog(ctx, nil, gitLogInput{
		ProjectDir: repoDir,
		Limit:      1,
		Oneline:    true,
	})
	if err != nil {
		t.Fatalf("handleGitLog: %v", err)
	}

	text := resultText(t, result)

	lines := strings.Split(strings.TrimSpace(text), "\n")
	if len(lines) != 1 {
		t.Errorf("expected exactly 1 log line with limit=1, got %d lines", len(lines))
	}

	if !strings.Contains(text, "second commit") {
		t.Errorf("expected most recent commit, got: %s", text)
	}
}

// ── handleGitBranches integration ──

func TestIntegration_GitBranches_DefaultBranch(t *testing.T) {
	skipIfNoGit(t)

	repoDir := initTempGitRepo(t)
	b := newTestBridge(t)
	ctx := context.Background()

	result, _, err := b.handleGitBranches(ctx, nil, gitBranchInput{
		ProjectDir: repoDir,
	})
	if err != nil {
		t.Fatalf("handleGitBranches: %v", err)
	}

	if result.IsError {
		t.Fatalf("handleGitBranches error: %s", resultText(t, result))
	}

	text := resultText(t, result)
	// Should show the current branch with the * marker.
	if !strings.Contains(text, "*") {
		t.Errorf("expected current branch marker (*), got: %s", text)
	}
}

func TestIntegration_GitBranches_MultipleBranches(t *testing.T) {
	skipIfNoGit(t)

	repoDir := initTempGitRepo(t)
	b := newTestBridge(t)
	ctx := context.Background()

	// Create additional branches.
	gitExec(t, repoDir, "branch", "feat/login")
	gitExec(t, repoDir, "branch", "fix/typo")

	result, _, err := b.handleGitBranches(ctx, nil, gitBranchInput{
		ProjectDir: repoDir,
	})
	if err != nil {
		t.Fatalf("handleGitBranches: %v", err)
	}

	text := resultText(t, result)
	if !strings.Contains(text, "feat/login") {
		t.Errorf("expected 'feat/login' branch, got: %s", text)
	}

	if !strings.Contains(text, "fix/typo") {
		t.Errorf("expected 'fix/typo' branch, got: %s", text)
	}
}

func TestIntegration_GitBranches_CurrentBranchIndicator(t *testing.T) {
	skipIfNoGit(t)

	repoDir := initTempGitRepo(t)
	b := newTestBridge(t)
	ctx := context.Background()

	// Create and checkout a new branch.
	gitExec(t, repoDir, "checkout", "-b", "feature-branch")

	result, _, err := b.handleGitBranches(ctx, nil, gitBranchInput{
		ProjectDir: repoDir,
	})
	if err != nil {
		t.Fatalf("handleGitBranches: %v", err)
	}

	text := resultText(t, result)
	// The current branch should have the * marker.
	for line := range strings.SplitSeq(text, "\n") {
		if strings.Contains(line, "feature-branch") {
			if !strings.HasPrefix(line, "* ") {
				t.Errorf("expected feature-branch to be marked as current, got: %s", line)
			}
		}
	}
}
