package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var (
	flagSince  string
	flagFormat string
)

func init() {
	releaseNotesCmd := newReleaseNotesCmd()
	rootCmd.AddCommand(releaseNotesCmd)
}

func newReleaseNotesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "release-notes",
		Short: "Generate release notes from git changelog between tags",
		Long: `Generate release notes from the git commit history between tags.

By default, generates notes from the previous tag to HEAD.
Use --since to specify a starting tag explicitly.
Use --format github to produce output suitable for gh release create --notes.`,
		RunE: runReleaseNotes,
	}

	cmd.Flags().StringVar(&flagSince, "since", "", "starting tag (default: tag before current HEAD)")
	cmd.Flags().StringVar(&flagFormat, "format", "github", "output format: github (default), plain")

	return cmd
}

const gitCmdTimeout = 30 * time.Second

// gitCmd runs a git command and returns stdout.
func gitCmd(ctx context.Context, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, gitCmdTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", args...)

	out, err := cmd.Output()
	if err != nil {
		ee := &exec.ExitError{}
		if errors.As(err, &ee) {
			return "", fmt.Errorf("git %s: %s", args[0], strings.TrimSpace(string(ee.Stderr)))
		}

		return "", fmt.Errorf("git %s: %w", args[0], err)
	}

	return strings.TrimSpace(string(out)), nil
}

// conventionalRe matches "type(scope): desc" or "type: desc".
var conventionalRe = regexp.MustCompile(`^([a-z]+)(?:\([^)]+\))?\s*:\s*(.*)$`)

// releaseCommit holds a parsed git commit for release notes.
type releaseCommit struct {
	shortHash   string
	subject     string
	commitType  string
	description string
}

func runReleaseNotes(_ *cobra.Command, _ []string) error {
	ctx := context.Background()

	since := flagSince

	// If no --since, find the previous tag.
	if since == "" {
		tag, err := gitCmd(ctx, "describe", "--tags", "--abbrev=0", "HEAD^")
		if err != nil {
			// Fallback: try to find the latest tag at all.
			tag, err = gitCmd(ctx, "describe", "--tags", "--abbrev=0")
			if err != nil {
				return fmt.Errorf("no tags found; use --since to specify a starting ref: %w", err)
			}
			// If the latest tag IS HEAD, there's nothing to diff.
			_, _ = fmt.Fprintf(os.Stderr, "warning: only tag found is %s (at HEAD); showing all commits since root\n", tag)
			since = ""
		} else {
			since = tag
		}
	}

	// Get current version tag (the one at or nearest HEAD).
	currentVersion, _ := gitCmd(ctx, "describe", "--tags", "--abbrev=0")
	if currentVersion == "" {
		currentVersion = "Unreleased"
	}

	notes, err := generateReleaseNotes(ctx, since, currentVersion)
	if err != nil {
		return err
	}

	_, _ = fmt.Fprint(os.Stdout, notes)

	return nil
}

func generateReleaseNotes(ctx context.Context, since, version string) (string, error) {
	const sep = "§"

	format := strings.Join([]string{"%h", "%s"}, sep)

	args := []string{"log", fmt.Sprintf("--format=%s", format)}
	if since != "" {
		args = append(args, since+"..HEAD")
	} else {
		args = append(args, "HEAD")
	}

	out, err := gitCmd(ctx, args...)
	if err != nil {
		return "", fmt.Errorf("git log: %w", err)
	}

	if out == "" {
		return fmt.Sprintf("# %s\n\nNo changes since %s.\n", version, since), nil
	}

	// Parse commits.
	var commits []releaseCommit

	for line := range strings.SplitSeq(out, "\n") {
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, sep, 2)
		if len(parts) < 2 {
			continue
		}

		c := releaseCommit{
			shortHash: parts[0],
			subject:   parts[1],
		}

		// Parse conventional commit type.
		if m := conventionalRe.FindStringSubmatch(c.subject); m != nil {
			c.commitType = m[1]
			c.description = m[2]
		} else {
			c.commitType = "other"
			c.description = c.subject
		}

		commits = append(commits, c)
	}

	if len(commits) == 0 {
		return fmt.Sprintf("# %s\n\nNo changes since %s.\n", version, since), nil
	}

	// Group by type.
	typeLabels := map[string]string{
		"feat":     "Features",
		"fix":      "Bug Fixes",
		"refactor": "Refactoring",
		"perf":     "Performance",
		"docs":     "Documentation",
		"test":     "Tests",
		"chore":    "Chores",
		"ci":       "CI",
		"build":    "Build",
		"style":    "Style",
	}

	grouped := make(map[string][]releaseCommit)
	for _, c := range commits {
		grouped[c.commitType] = append(grouped[c.commitType], c)
	}

	var sb strings.Builder

	if flagFormat == "github" {
		fmt.Fprintf(&sb, "# %s\n", version)
	} else {
		fmt.Fprintf(&sb, "# Release Notes — %s\n", version)
	}

	if since != "" {
		fmt.Fprintf(&sb, "\nChanges since **%s**\n", since)
	}

	// Ordered output for deterministic results.
	order := []string{"feat", "fix", "refactor", "perf", "docs", "test", "chore", "ci", "build", "style", "other"}
	for _, t := range order {
		cs, ok := grouped[t]
		if !ok {
			continue
		}

		label := typeLabels[t]
		if label == "" {
			label = "Other"
		}

		fmt.Fprintf(&sb, "\n## %s\n\n", label)

		for _, c := range cs {
			fmt.Fprintf(&sb, "- %s %s\n", c.shortHash, c.description)
		}
	}

	// Footer stats.
	fmt.Fprintf(&sb, "\n---\n\n**%d** commit(s)", len(commits))

	if since != "" {
		fmt.Fprintf(&sb, " since %s", since)
	}

	sb.WriteString("\n")

	return sb.String(), nil
}
