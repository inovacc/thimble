package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(newPublishCmd())
}

func newPublishCmd() *cobra.Command {
	var (
		message   string
		version   string
		bump      string
		dryRun    bool
		skipTests bool
		watch     bool
	)

	cmd := &cobra.Command{
		Use:   "publish",
		Short: "Commit, tag, push, and monitor the release pipeline",
		Long: `Full publish flow: commit staged changes, create a version tag, push to
GitHub, and monitor CI/release/Docker pipelines until completion.

Steps:
  1. Run tests (unless --skip-tests)
  2. Commit staged changes (or all if nothing staged)
  3. Create annotated git tag with release notes
  4. Push commit + tag to origin
  5. Monitor GitHub Actions until completion (if --watch)

Version can be set explicitly or auto-bumped:
  --version v2.5.0    explicit version
  --bump auto          auto-detect from conventional commits (feat→minor, fix→patch)
  --bump major         force major bump
  --bump minor         force minor bump
  --bump patch         force patch bump

Examples:
  thimble publish --bump auto                    # auto-detect version from commits
  thimble publish --bump auto --dry-run          # preview auto-detected version
  thimble publish --version v2.5.0 --watch       # explicit version + monitor
  thimble publish --bump minor -m "new feature"  # force minor bump`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()

			// Auto-detect version if --bump is used.
			if bump != "" && version == "" {
				detected, err := autoBumpVersion(ctx, bump)
				if err != nil {
					return fmt.Errorf("auto-bump: %w", err)
				}

				version = detected
			}

			if version == "" {
				return fmt.Errorf("--version or --bump is required (e.g., --version v2.5.0 or --bump auto)")
			}

			if !strings.HasPrefix(version, "v") {
				version = "v" + version
			}

			return runPublish(ctx, publishOpts{
				version:   version,
				message:   message,
				dryRun:    dryRun,
				skipTests: skipTests,
				watch:     watch,
			})
		},
	}

	cmd.Flags().StringVarP(&version, "version", "v", "", "version tag (e.g., v2.5.0)")
	cmd.Flags().StringVarP(&bump, "bump", "b", "", "auto-bump version: auto, major, minor, patch")
	cmd.Flags().StringVarP(&message, "message", "m", "", "commit message (auto-generated if empty)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show what would happen without executing")
	cmd.Flags().BoolVar(&skipTests, "skip-tests", false, "skip running tests before publish")
	cmd.Flags().BoolVarP(&watch, "watch", "w", false, "monitor GitHub Actions until pipelines complete")

	return cmd
}

type publishOpts struct {
	version   string
	message   string
	dryRun    bool
	skipTests bool
	watch     bool
}

func runPublish(ctx context.Context, opts publishOpts) error {
	// Step 0: Verify we're in a git repo and gh is available.
	if _, err := run(ctx, "git", "rev-parse", "--git-dir"); err != nil {
		return fmt.Errorf("not a git repository")
	}

	// Step 1: Run tests.
	if !opts.skipTests {
		stepf("Running tests...")

		if opts.dryRun {
			infof("  [dry-run] would run: go test ./... -short")
		} else {
			if _, err := run(ctx, "go", "test", "-short", "./..."); err != nil {
				return fmt.Errorf("tests failed — aborting publish: %w", err)
			}

			successf("Tests passed")
		}
	}

	// Step 2: Check for changes to commit.
	status, _ := run(ctx, "git", "status", "--porcelain")
	hasChanges := strings.TrimSpace(status) != ""

	if hasChanges {
		stepf("Committing changes...")

		commitMsg := opts.message
		if commitMsg == "" {
			commitMsg = fmt.Sprintf("chore: release %s", opts.version)
		}

		if opts.dryRun {
			infof("  [dry-run] would run: git add -A && git commit -m %q", commitMsg)
		} else {
			if _, err := run(ctx, "git", "add", "-A"); err != nil {
				return fmt.Errorf("git add: %w", err)
			}

			if _, err := run(ctx, "git", "commit", "-m", commitMsg); err != nil {
				return fmt.Errorf("git commit: %w", err)
			}

			successf("Committed: %s", commitMsg)
		}
	} else {
		infof("Working tree clean — no commit needed")
	}

	// Step 3: Generate release notes and create tag.
	stepf("Creating tag %s...", opts.version)

	notes, err := generateReleaseNotes(ctx, "", opts.version)
	if err != nil {
		notes = fmt.Sprintf("Release %s", opts.version)
	}

	if opts.dryRun {
		infof("  [dry-run] would create tag: %s", opts.version)
		infof("  Release notes preview:\n%s", notes)
	} else {
		if _, err := run(ctx, "git", "tag", "-a", opts.version, "-m", notes); err != nil {
			return fmt.Errorf("git tag: %w", err)
		}

		successf("Tagged %s", opts.version)
	}

	// Step 4: Push commit + tag.
	stepf("Pushing to origin...")

	if opts.dryRun {
		infof("  [dry-run] would run: git push origin main && git push origin %s", opts.version)
	} else {
		if _, err := run(ctx, "git", "push", "origin", "main"); err != nil {
			return fmt.Errorf("git push main: %w", err)
		}

		if _, err := run(ctx, "git", "push", "origin", opts.version); err != nil {
			return fmt.Errorf("git push tag: %w", err)
		}

		successf("Pushed main + %s to origin", opts.version)
	}

	// Step 5: Monitor pipelines.
	if opts.watch && !opts.dryRun {
		stepf("Monitoring GitHub Actions pipelines...")
		return watchPipelines(ctx, opts.version)
	}

	if !opts.dryRun {
		_, _ = fmt.Fprintln(os.Stdout)

		infof("Pipelines triggered. Run 'thimble publish-status' or 'gh run list' to check progress.")
	}

	return nil
}

func watchPipelines(ctx context.Context, version string) error {
	// Wait a moment for GitHub to create the runs.
	time.Sleep(3 * time.Second)

	// Poll every 15 seconds for up to 15 minutes.
	timeout := time.After(15 * time.Minute)

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout:
			warnf("Timed out after 15 minutes — check 'gh run list' manually")
			return nil
		case <-ticker.C:
			done, err := checkPipelineStatus(ctx, version)
			if err != nil {
				warnf("Error checking status: %v", err)
				continue
			}

			if done {
				return nil
			}
		}
	}
}

func checkPipelineStatus(ctx context.Context, version string) (bool, error) {
	// Get runs for this tag.
	out, err := run(ctx, "gh", "run", "list", "--limit", "10", "--json", "name,status,conclusion,headBranch,databaseId")
	if err != nil {
		return false, err
	}

	// Parse the JSON output to find runs related to our version/tag.
	// Simple approach: look at the most recent runs.
	lines := strings.Split(strings.TrimSpace(out), "\n")

	allDone := true
	anyFailed := false
	pipelineCount := 0

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || line == "[" || line == "]" {
			continue
		}

		// Check for our tag in the output.
		if !strings.Contains(line, version) && !strings.Contains(line, "main") {
			continue
		}

		pipelineCount++

		switch {
		case strings.Contains(line, `"in_progress"`) || strings.Contains(line, `"queued"`):
			allDone = false
		case strings.Contains(line, `"failure"`):
			anyFailed = true
		}
	}

	if pipelineCount == 0 {
		infof("  Waiting for pipelines to start...")
		return false, nil
	}

	if !allDone {
		infof("  Pipelines running... (%d tracked)", pipelineCount)
		return false, nil
	}

	// All done.
	_, _ = fmt.Fprintln(os.Stdout)

	if anyFailed {
		warnf("Some pipelines failed! Run 'gh run list' for details.")
	} else {
		successf("All pipelines completed successfully!")
	}

	// Show final status.
	listOut, _ := run(ctx, "gh", "run", "list", "--limit", "5")
	if listOut != "" {
		_, _ = fmt.Fprintln(os.Stdout, listOut)
	}

	return true, nil
}

// run executes a command and returns stdout.
func run(ctx context.Context, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)

	out, err := cmd.CombinedOutput()
	if err != nil {
		ee := &exec.ExitError{}
		if errors.As(err, &ee) {
			return string(out), fmt.Errorf("%s %s: exit %d: %s", name, strings.Join(args, " "), ee.ExitCode(), strings.TrimSpace(string(out)))
		}

		return "", err
	}

	return string(out), nil
}

func stepf(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stdout, "\n→ "+format+"\n", args...)
}

func successf(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stdout, "  ✓ "+format+"\n", args...)
}

func infof(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stdout, "  "+format+"\n", args...)
}

func warnf(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, "  ⚠ "+format+"\n", args...)
}

// ── Auto Version Bump ──

// autoBumpVersion determines the next version by analyzing commits since the
// latest tag. Uses conventional commit prefixes to decide the bump level:
//   - "feat:" or "feat(...):" → minor bump
//   - "fix:" or any other → patch bump
//   - BREAKING CHANGE in body or "!" after type → major bump
//   - --bump major/minor/patch forces the specified level
func autoBumpVersion(ctx context.Context, bumpFlag string) (string, error) {
	// Get latest tag from local + remote.
	latestTag, err := getLatestTag(ctx)
	if err != nil {
		return "", fmt.Errorf("no tags found: %w", err)
	}

	major, minor, patch, err := parseVersion(latestTag)
	if err != nil {
		return "", fmt.Errorf("parse version %q: %w", latestTag, err)
	}

	infof("Latest tag: %s (parsed: %d.%d.%d)", latestTag, major, minor, patch)

	var level string

	switch bumpFlag {
	case "major":
		level = "major"
	case "minor":
		level = "minor"
	case "patch":
		level = "patch"
	case "auto":
		level, err = detectBumpLevel(ctx, latestTag)
		if err != nil {
			level = "patch" // fallback

			infof("Could not detect bump level, defaulting to patch: %v", err)
		}
	default:
		return "", fmt.Errorf("unknown bump level %q (use: auto, major, minor, patch)", bumpFlag)
	}

	switch level {
	case "major":
		major++
		minor = 0
		patch = 0
	case "minor":
		minor++
		patch = 0
	case "patch":
		patch++
	}

	next := fmt.Sprintf("v%d.%d.%d", major, minor, patch)
	infof("Bump: %s → %s (%s)", latestTag, next, level)

	return next, nil
}

// getLatestTag finds the latest semver tag, checking both local and remote.
func getLatestTag(ctx context.Context) (string, error) {
	// Try local first.
	tag, err := gitCmd(ctx, "describe", "--tags", "--abbrev=0")
	if err == nil && tag != "" {
		return tag, nil
	}

	// Fetch tags from remote and try again.
	_, _ = run(ctx, "git", "fetch", "--tags", "origin")

	tag, err = gitCmd(ctx, "describe", "--tags", "--abbrev=0")
	if err == nil && tag != "" {
		return tag, nil
	}

	// Last resort: list all tags sorted.
	out, err := gitCmd(ctx, "tag", "-l", "--sort=-version:refname")
	if err != nil {
		return "", err
	}

	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) == 0 || lines[0] == "" {
		return "", fmt.Errorf("no tags found")
	}

	return lines[0], nil
}

// parseVersion extracts major.minor.patch from a version string like "v2.3.0".
func parseVersion(tag string) (major, minor, patch int, err error) {
	tag = strings.TrimPrefix(tag, "v")

	var n int

	n, err = fmt.Sscanf(tag, "%d.%d.%d", &major, &minor, &patch)
	if err != nil || n != 3 {
		return 0, 0, 0, fmt.Errorf("invalid semver: %q", tag)
	}

	return major, minor, patch, nil
}

// detectBumpLevel analyzes commits since the given tag and returns "major", "minor", or "patch".
func detectBumpLevel(ctx context.Context, sinceTag string) (string, error) {
	out, err := gitCmd(ctx, "log", "--format=%s%n%b", sinceTag+"..HEAD")
	if err != nil {
		return "", err
	}

	if strings.TrimSpace(out) == "" {
		return "patch", nil
	}

	hasBreaking := false
	hasFeat := false

	for rawLine := range strings.SplitSeq(out, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}

		lower := strings.ToLower(line)

		// Check for breaking changes.
		if strings.Contains(lower, "breaking change") || strings.Contains(lower, "breaking-change") {
			hasBreaking = true
		}
		// Check for "type!:" syntax.
		if m := conventionalRe.FindStringSubmatch(line); m != nil {
			if strings.HasSuffix(m[1], "!") || strings.Contains(line, "!:") {
				hasBreaking = true
			}
		}
		// Check for feat commits.
		if strings.HasPrefix(lower, "feat:") || strings.HasPrefix(lower, "feat(") {
			hasFeat = true
		}
	}

	switch {
	case hasBreaking:
		return "major", nil
	case hasFeat:
		return "minor", nil
	default:
		return "patch", nil
	}
}
