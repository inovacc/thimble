package security

import "strings"

// DangerousGitSubcommands are git operations denied by default because they
// are destructive or hard to reverse.
var DangerousGitSubcommands = []string{
	"push --force",
	"push -f",
	"push --force-with-lease",
	"reset --hard",
	"clean -f",
	"checkout .",
	"restore .",
	"branch -D",
	"branch --delete --force",
	"rebase",       // interactive rebase can rewrite history
	"filter-branch",
	"reflog expire",
	"gc --prune=now",
}

// CustomGitPolicy holds user-configured dangerous git subcommand patterns.
// When non-nil and non-empty, these patterns replace the default DangerousGitSubcommands list.
type CustomGitPolicy struct {
	Patterns []string
}

// CustomGhPolicy holds user-configured dangerous gh subcommand patterns.
// When non-nil and non-empty, these patterns replace the default DangerousGhSubcommands list.
type CustomGhPolicy struct {
	Patterns []string
}

// LoadCustomGitPolicy creates a CustomGitPolicy from user-provided patterns.
// Returns nil if the patterns slice is nil or empty, indicating defaults should be used.
func LoadCustomGitPolicy(patterns []string) *CustomGitPolicy {
	if len(patterns) == 0 {
		return nil
	}

	return &CustomGitPolicy{Patterns: patterns}
}

// LoadCustomGhPolicy creates a CustomGhPolicy from user-provided patterns.
// Returns nil if the patterns slice is nil or empty, indicating defaults should be used.
func LoadCustomGhPolicy(patterns []string) *CustomGhPolicy {
	if len(patterns) == 0 {
		return nil
	}

	return &CustomGhPolicy{Patterns: patterns}
}

// CustomPoliciesFromSettings extracts CustomGitPolicy and CustomGhPolicy from
// a slice of SecurityPolicy. The first non-empty override list wins (highest
// precedence first, matching ReadBashPolicies order).
func CustomPoliciesFromSettings(policies []SecurityPolicy) (*CustomGitPolicy, *CustomGhPolicy) {
	var gitPolicy *CustomGitPolicy

	var ghPolicy *CustomGhPolicy

	for _, p := range policies {
		if gitPolicy == nil {
			gitPolicy = LoadCustomGitPolicy(p.DangerousGitOverrides)
		}

		if ghPolicy == nil {
			ghPolicy = LoadCustomGhPolicy(p.DangerousGhOverrides)
		}

		if gitPolicy != nil && ghPolicy != nil {
			break
		}
	}

	return gitPolicy, ghPolicy
}

// IsGitCommandDangerous checks if a git command contains a dangerous subcommand.
// Returns the matched dangerous pattern, or "" if safe.
// Uses the hardcoded DangerousGitSubcommands list.
func IsGitCommandDangerous(command string) string {
	return IsGitCommandDangerousWithPolicy(command, nil)
}

// IsGitCommandDangerousWithPolicy checks if a git command contains a dangerous subcommand
// using the provided custom policy. If policy is nil or has no patterns, the default
// DangerousGitSubcommands list is used.
// Returns the matched dangerous pattern, or "" if safe.
func IsGitCommandDangerousWithPolicy(command string, policy *CustomGitPolicy) string {
	normalized := strings.TrimSpace(command)

	// Must start with "git ".
	if !strings.HasPrefix(normalized, "git ") {
		return ""
	}

	rest := strings.TrimPrefix(normalized, "git ")
	rest = strings.TrimSpace(rest)

	patterns := DangerousGitSubcommands
	if policy != nil && len(policy.Patterns) > 0 {
		patterns = policy.Patterns
	}

	for _, dangerous := range patterns {
		if rest == dangerous || strings.HasPrefix(rest, dangerous+" ") || strings.HasPrefix(rest, dangerous+"\t") {
			return "git " + dangerous
		}
		// For flag-based patterns like "clean -f", also match extended flags like "clean -fd".
		if strings.Contains(dangerous, " -") && strings.HasPrefix(rest, dangerous) {
			return "git " + dangerous
		}
	}

	return ""
}

// DangerousGhSubcommands are gh CLI operations denied by default.
var DangerousGhSubcommands = []string{
	"repo delete",
	"repo archive",
	"repo rename",
	"api --method DELETE",
	"api -X DELETE",
	"release delete",
	"issue close",
	"issue delete",
	"pr close",
	"pr merge",
	"secret delete",
	"variable delete",
	"ssh-key delete",
	"gpg-key delete",
}

// IsGhCommandDangerous checks if a gh command contains a dangerous subcommand.
// Returns the matched dangerous pattern, or "" if safe.
// Uses the hardcoded DangerousGhSubcommands list.
func IsGhCommandDangerous(command string) string {
	return IsGhCommandDangerousWithPolicy(command, nil)
}

// IsGhCommandDangerousWithPolicy checks if a gh command contains a dangerous subcommand
// using the provided custom policy. If policy is nil or has no patterns, the default
// DangerousGhSubcommands list is used.
// Returns the matched dangerous pattern, or "" if safe.
func IsGhCommandDangerousWithPolicy(command string, policy *CustomGhPolicy) string {
	normalized := strings.TrimSpace(command)

	if !strings.HasPrefix(normalized, "gh ") {
		return ""
	}

	rest := strings.TrimPrefix(normalized, "gh ")
	rest = strings.TrimSpace(rest)

	patterns := DangerousGhSubcommands
	if policy != nil && len(policy.Patterns) > 0 {
		patterns = policy.Patterns
	}

	for _, dangerous := range patterns {
		if rest == dangerous || strings.HasPrefix(rest, dangerous+" ") || strings.HasPrefix(rest, dangerous+"\t") {
			return "gh " + dangerous
		}
	}

	return ""
}

