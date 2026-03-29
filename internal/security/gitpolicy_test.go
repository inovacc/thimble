package security

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestIsGitCommandDangerous(t *testing.T) {
	tests := []struct {
		command string
		want    string // empty = safe
	}{
		// Dangerous commands.
		{"git push --force origin main", "git push --force"},
		{"git push -f origin main", "git push -f"},
		{"git reset --hard HEAD~3", "git reset --hard"},
		{"git clean -f", "git clean -f"},
		{"git clean -fd", "git clean -f"},
		{"git clean -fx", "git clean -f"},
		{"git checkout .", "git checkout ."},
		{"git restore .", "git restore ."},
		{"git branch -D feature", "git branch -D"},
		{"git rebase main", "git rebase"},
		{"git filter-branch --env-filter", "git filter-branch"},

		// Safe commands.
		{"git status", ""},
		{"git diff", ""},
		{"git log --oneline", ""},
		{"git add file.go", ""},
		{"git commit -m 'test'", ""},
		{"git push origin main", ""},
		{"git branch feature", ""},
		{"git checkout feature", ""},
		{"git stash list", ""},
		{"git blame file.go", ""},
		{"git fetch origin", ""},
		{"git pull", ""},
		{"git merge feature", ""},
		{"git tag v1.0.0", ""},

		// Not git commands.
		{"ls -la", ""},
		{"echo git push --force", ""},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			got := IsGitCommandDangerous(tt.command)
			if got != tt.want {
				t.Errorf("IsGitCommandDangerous(%q) = %q, want %q", tt.command, got, tt.want)
			}
		})
	}
}

func TestIsGhCommandDangerous(t *testing.T) {
	tests := []struct {
		command string
		want    string
	}{
		// Dangerous.
		{"gh repo delete owner/repo", "gh repo delete"},
		{"gh repo archive owner/repo", "gh repo archive"},
		{"gh release delete v1.0", "gh release delete"},
		{"gh issue close 123", "gh issue close"},
		{"gh pr close 456", "gh pr close"},
		{"gh pr merge 789", "gh pr merge"},
		{"gh api --method DELETE /repos/foo/bar", "gh api --method DELETE"},
		{"gh api -X DELETE /repos/foo/bar", "gh api -X DELETE"},
		{"gh secret delete MY_SECRET", "gh secret delete"},

		// Safe.
		{"gh pr list", ""},
		{"gh pr view 123", ""},
		{"gh issue list", ""},
		{"gh issue create --title test", ""},
		{"gh run list", ""},
		{"gh repo view", ""},
		{"gh api /repos/foo/bar", ""},
		{"gh search repos go", ""},
		{"gh auth status", ""},
		{"gh release list", ""},

		// Not gh commands.
		{"git push --force", ""},
		{"echo gh repo delete", ""},

		// Edge cases: whitespace.
		{"  gh repo delete owner/repo  ", "gh repo delete"},
		{"gh variable delete VAR_NAME", "gh variable delete"},
		{"gh ssh-key delete 12345", "gh ssh-key delete"},
		{"gh gpg-key delete ABCDEF", "gh gpg-key delete"},
		{"gh issue delete 42", "gh issue delete"},
		{"gh repo rename new-name", "gh repo rename"},

		// Edge case: tab separator.
		{"gh repo delete\towner/repo", "gh repo delete"},

		// Edge case: exact match (no trailing args).
		{"gh pr merge", "gh pr merge"},

		// Edge case: empty and whitespace-only.
		{"", ""},
		{"   ", ""},
		{"gh", ""},
		{"gh ", ""},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			got := IsGhCommandDangerous(tt.command)
			if got != tt.want {
				t.Errorf("IsGhCommandDangerous(%q) = %q, want %q", tt.command, got, tt.want)
			}
		})
	}
}

func TestIsGitCommandDangerousEdgeCases(t *testing.T) {
	tests := []struct {
		command string
		want    string
	}{
		// Whitespace handling.
		{"  git push --force origin main  ", "git push --force"},
		// --force-with-lease matches "push --force" first due to flag-based HasPrefix matching.
		{"git push --force-with-lease origin main", "git push --force"},

		// Exact match without trailing args.
		{"git rebase", "git rebase"},
		{"git checkout .", "git checkout ."},

		// Tab separator.
		{"git reset --hard\tHEAD~1", "git reset --hard"},

		// gc with prune.
		{"git gc --prune=now", "git gc --prune=now"},
		{"git reflog expire --all", "git reflog expire"},

		// branch --delete --force (long form).
		{"git branch --delete --force feature", "git branch --delete --force"},

		// Empty / whitespace.
		{"", ""},
		{"   ", ""},
		{"git", ""},
		{"git ", ""},

		// Safe variants that look similar.
		{"git checkout -b newbranch", ""},
		{"git restore --staged file.go", ""},
		{"git push origin main", ""},
		{"git branch -d feature", ""},
		{"git clean -n", ""},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			got := IsGitCommandDangerous(tt.command)
			if got != tt.want {
				t.Errorf("IsGitCommandDangerous(%q) = %q, want %q", tt.command, got, tt.want)
			}
		})
	}
}

// ── Custom policy tests ──

func TestLoadCustomGitPolicy(t *testing.T) {
	t.Run("nil patterns returns nil", func(t *testing.T) {
		p := LoadCustomGitPolicy(nil)
		if p != nil {
			t.Error("expected nil for nil patterns")
		}
	})

	t.Run("empty patterns returns nil", func(t *testing.T) {
		p := LoadCustomGitPolicy([]string{})
		if p != nil {
			t.Error("expected nil for empty patterns")
		}
	})

	t.Run("non-empty patterns returns policy", func(t *testing.T) {
		p := LoadCustomGitPolicy([]string{"push --force"})
		if p == nil {
			t.Fatal("expected non-nil policy")
		}

		if len(p.Patterns) != 1 || p.Patterns[0] != "push --force" {
			t.Errorf("unexpected patterns: %v", p.Patterns)
		}
	})
}

func TestLoadCustomGhPolicy(t *testing.T) {
	t.Run("nil patterns returns nil", func(t *testing.T) {
		p := LoadCustomGhPolicy(nil)
		if p != nil {
			t.Error("expected nil for nil patterns")
		}
	})

	t.Run("non-empty patterns returns policy", func(t *testing.T) {
		p := LoadCustomGhPolicy([]string{"repo delete"})
		if p == nil {
			t.Fatal("expected non-nil policy")
		}

		if len(p.Patterns) != 1 {
			t.Errorf("unexpected patterns: %v", p.Patterns)
		}
	})
}

func TestIsGitCommandDangerousWithPolicy(t *testing.T) {
	t.Run("nil policy uses defaults", func(t *testing.T) {
		// "git rebase" is in defaults.
		got := IsGitCommandDangerousWithPolicy("git rebase main", nil)
		if got != "git rebase" {
			t.Errorf("expected 'git rebase', got %q", got)
		}
	})

	t.Run("custom policy replaces defaults", func(t *testing.T) {
		policy := &CustomGitPolicy{Patterns: []string{"stash drop"}}

		// "git rebase" is in defaults but NOT in custom policy.
		got := IsGitCommandDangerousWithPolicy("git rebase main", policy)
		if got != "" {
			t.Errorf("expected safe (empty), got %q", got)
		}

		// "git stash drop" is in custom policy.
		got = IsGitCommandDangerousWithPolicy("git stash drop", policy)
		if got != "git stash drop" {
			t.Errorf("expected 'git stash drop', got %q", got)
		}
	})

	t.Run("empty-patterns policy falls back to defaults", func(t *testing.T) {
		policy := &CustomGitPolicy{Patterns: []string{}}

		got := IsGitCommandDangerousWithPolicy("git rebase main", policy)
		if got != "git rebase" {
			t.Errorf("expected 'git rebase', got %q", got)
		}
	})

	t.Run("custom policy with flag-based matching", func(t *testing.T) {
		policy := &CustomGitPolicy{Patterns: []string{"clean -f"}}

		got := IsGitCommandDangerousWithPolicy("git clean -fd", policy)
		if got != "git clean -f" {
			t.Errorf("expected 'git clean -f', got %q", got)
		}

		got = IsGitCommandDangerousWithPolicy("git clean -n", policy)
		if got != "" {
			t.Errorf("expected safe (empty), got %q", got)
		}
	})
}

func TestIsGhCommandDangerousWithPolicy(t *testing.T) {
	t.Run("nil policy uses defaults", func(t *testing.T) {
		got := IsGhCommandDangerousWithPolicy("gh repo delete owner/repo", nil)
		if got != "gh repo delete" {
			t.Errorf("expected 'gh repo delete', got %q", got)
		}
	})

	t.Run("custom policy replaces defaults", func(t *testing.T) {
		policy := &CustomGhPolicy{Patterns: []string{"workflow disable"}}

		// "gh repo delete" is in defaults but NOT in custom policy.
		got := IsGhCommandDangerousWithPolicy("gh repo delete owner/repo", policy)
		if got != "" {
			t.Errorf("expected safe (empty), got %q", got)
		}

		// "gh workflow disable" is in custom policy.
		got = IsGhCommandDangerousWithPolicy("gh workflow disable my-workflow", policy)
		if got != "gh workflow disable" {
			t.Errorf("expected 'gh workflow disable', got %q", got)
		}
	})

	t.Run("empty-patterns policy falls back to defaults", func(t *testing.T) {
		policy := &CustomGhPolicy{Patterns: []string{}}

		got := IsGhCommandDangerousWithPolicy("gh pr merge 123", policy)
		if got != "gh pr merge" {
			t.Errorf("expected 'gh pr merge', got %q", got)
		}
	})
}

func TestIsGitCommandDangerousBackwardCompatible(t *testing.T) {
	// Ensure the original function still works identically.
	tests := []struct {
		command string
		want    string
	}{
		{"git push --force origin main", "git push --force"},
		{"git reset --hard HEAD~3", "git reset --hard"},
		{"git status", ""},
		{"git log --oneline", ""},
		{"ls -la", ""},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			got := IsGitCommandDangerous(tt.command)
			if got != tt.want {
				t.Errorf("IsGitCommandDangerous(%q) = %q, want %q", tt.command, got, tt.want)
			}
		})
	}
}

func TestSecurityPolicyOverridesFromSettings(t *testing.T) {
	tmpDir := t.TempDir()

	claudeDir := filepath.Join(tmpDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}

	settings := map[string]any{
		"permissions": map[string]any{
			"allow": []string{},
			"deny":  []string{},
			"ask":   []string{},
		},
		"dangerous_git_overrides": []string{"stash drop", "push --force"},
		"dangerous_gh_overrides":  []string{"workflow disable", "repo delete"},
	}

	data, err := json.Marshal(settings)
	if err != nil {
		t.Fatal(err)
	}

	settingsPath := filepath.Join(claudeDir, "settings.json")
	if err := os.WriteFile(settingsPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	policies := ReadBashPolicies(tmpDir, filepath.Join(tmpDir, "nonexistent.json"))
	if len(policies) != 1 {
		t.Fatalf("expected 1 policy, got %d", len(policies))
	}

	p := policies[0]

	// Verify git overrides.
	if len(p.DangerousGitOverrides) != 2 {
		t.Fatalf("expected 2 git overrides, got %d", len(p.DangerousGitOverrides))
	}

	if p.DangerousGitOverrides[0] != "stash drop" || p.DangerousGitOverrides[1] != "push --force" {
		t.Errorf("unexpected git overrides: %v", p.DangerousGitOverrides)
	}

	// Verify gh overrides.
	if len(p.DangerousGhOverrides) != 2 {
		t.Fatalf("expected 2 gh overrides, got %d", len(p.DangerousGhOverrides))
	}

	if p.DangerousGhOverrides[0] != "workflow disable" || p.DangerousGhOverrides[1] != "repo delete" {
		t.Errorf("unexpected gh overrides: %v", p.DangerousGhOverrides)
	}

	// Use the overrides with the policy functions.
	gitPolicy := LoadCustomGitPolicy(p.DangerousGitOverrides)
	ghPolicy := LoadCustomGhPolicy(p.DangerousGhOverrides)

	// "stash drop" should be dangerous with custom policy.
	if got := IsGitCommandDangerousWithPolicy("git stash drop", gitPolicy); got != "git stash drop" {
		t.Errorf("expected 'git stash drop', got %q", got)
	}

	// "rebase" is NOT in custom policy, so it should be safe.
	if got := IsGitCommandDangerousWithPolicy("git rebase main", gitPolicy); got != "" {
		t.Errorf("expected safe (empty), got %q", got)
	}

	// "workflow disable" should be dangerous with custom policy.
	if got := IsGhCommandDangerousWithPolicy("gh workflow disable my-wf", ghPolicy); got != "gh workflow disable" {
		t.Errorf("expected 'gh workflow disable', got %q", got)
	}

	// "pr merge" is NOT in custom policy, so it should be safe.
	if got := IsGhCommandDangerousWithPolicy("gh pr merge 123", ghPolicy); got != "" {
		t.Errorf("expected safe (empty), got %q", got)
	}
}

func TestSecurityPolicyNoOverrides(t *testing.T) {
	tmpDir := t.TempDir()

	claudeDir := filepath.Join(tmpDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Settings without override fields -- should result in nil/empty overrides.
	settings := map[string]any{
		"permissions": map[string]any{
			"allow": []string{"Bash(git status)"},
			"deny":  []string{},
			"ask":   []string{},
		},
	}

	data, err := json.Marshal(settings)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(claudeDir, "settings.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	policies := ReadBashPolicies(tmpDir, filepath.Join(tmpDir, "nonexistent.json"))
	if len(policies) != 1 {
		t.Fatalf("expected 1 policy, got %d", len(policies))
	}

	p := policies[0]

	if len(p.DangerousGitOverrides) != 0 {
		t.Errorf("expected no git overrides, got %v", p.DangerousGitOverrides)
	}

	if len(p.DangerousGhOverrides) != 0 {
		t.Errorf("expected no gh overrides, got %v", p.DangerousGhOverrides)
	}

	// With nil overrides, defaults should be used.
	gitPolicy := LoadCustomGitPolicy(p.DangerousGitOverrides)
	if gitPolicy != nil {
		t.Error("expected nil git policy for empty overrides")
	}

	// Default behavior: "rebase" is dangerous.
	if got := IsGitCommandDangerousWithPolicy("git rebase main", gitPolicy); got != "git rebase" {
		t.Errorf("expected 'git rebase', got %q", got)
	}
}
