package security

import "testing"

func TestCustomPoliciesFromSettings_Empty(t *testing.T) {
	gitP, ghP := CustomPoliciesFromSettings(nil)
	if gitP != nil {
		t.Errorf("expected nil gitPolicy for nil input, got %v", gitP)
	}

	if ghP != nil {
		t.Errorf("expected nil ghPolicy for nil input, got %v", ghP)
	}
}

func TestCustomPoliciesFromSettings_NoOverrides(t *testing.T) {
	policies := []SecurityPolicy{
		{Allow: []string{"echo *"}, Deny: []string{"rm -rf"}},
	}

	gitP, ghP := CustomPoliciesFromSettings(policies)
	if gitP != nil {
		t.Errorf("expected nil gitPolicy when no overrides, got %v", gitP)
	}

	if ghP != nil {
		t.Errorf("expected nil ghPolicy when no overrides, got %v", ghP)
	}
}

func TestCustomPoliciesFromSettings_GitOverride(t *testing.T) {
	policies := []SecurityPolicy{
		{DangerousGitOverrides: []string{"push --force", "reset --hard"}},
	}

	gitP, ghP := CustomPoliciesFromSettings(policies)
	if gitP == nil {
		t.Fatal("expected non-nil gitPolicy")
	}

	if len(gitP.Patterns) != 2 {
		t.Errorf("expected 2 git patterns, got %d", len(gitP.Patterns))
	}

	if ghP != nil {
		t.Errorf("expected nil ghPolicy, got %v", ghP)
	}
}

func TestCustomPoliciesFromSettings_GhOverride(t *testing.T) {
	policies := []SecurityPolicy{
		{DangerousGhOverrides: []string{"repo delete"}},
	}

	gitP, ghP := CustomPoliciesFromSettings(policies)
	if gitP != nil {
		t.Errorf("expected nil gitPolicy, got %v", gitP)
	}

	if ghP == nil {
		t.Fatal("expected non-nil ghPolicy")
	}

	if len(ghP.Patterns) != 1 {
		t.Errorf("expected 1 gh pattern, got %d", len(ghP.Patterns))
	}
}

func TestCustomPoliciesFromSettings_BothOverrides(t *testing.T) {
	policies := []SecurityPolicy{
		{
			DangerousGitOverrides: []string{"push --force"},
			DangerousGhOverrides:  []string{"repo delete", "pr close"},
		},
	}

	gitP, ghP := CustomPoliciesFromSettings(policies)
	if gitP == nil {
		t.Fatal("expected non-nil gitPolicy")
	}

	if ghP == nil {
		t.Fatal("expected non-nil ghPolicy")
	}

	if len(gitP.Patterns) != 1 {
		t.Errorf("git patterns = %d, want 1", len(gitP.Patterns))
	}

	if len(ghP.Patterns) != 2 {
		t.Errorf("gh patterns = %d, want 2", len(ghP.Patterns))
	}
}

func TestCustomPoliciesFromSettings_FirstWins(t *testing.T) {
	policies := []SecurityPolicy{
		{DangerousGitOverrides: []string{"push --force"}},
		{DangerousGitOverrides: []string{"reset --hard", "clean -f"}},
	}

	gitP, _ := CustomPoliciesFromSettings(policies)
	if gitP == nil {
		t.Fatal("expected non-nil gitPolicy")
	}

	// First non-empty wins.
	if len(gitP.Patterns) != 1 || gitP.Patterns[0] != "push --force" {
		t.Errorf("expected first policy to win, got %v", gitP.Patterns)
	}
}

func TestCustomPoliciesFromSettings_SkipsEmptyThenTakesNext(t *testing.T) {
	policies := []SecurityPolicy{
		{DangerousGitOverrides: nil},
		{DangerousGitOverrides: []string{"rebase"}},
	}

	gitP, _ := CustomPoliciesFromSettings(policies)
	if gitP == nil {
		t.Fatal("expected non-nil gitPolicy from second policy")
	}

	if len(gitP.Patterns) != 1 || gitP.Patterns[0] != "rebase" {
		t.Errorf("expected rebase pattern, got %v", gitP.Patterns)
	}
}
