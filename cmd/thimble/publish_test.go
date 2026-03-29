package main

import (
	"strings"
	"testing"
)

func TestParseVersion_Valid(t *testing.T) {
	tests := []struct {
		tag                    string
		wantMajor, wantMinor   int
		wantPatch              int
	}{
		{"v1.2.3", 1, 2, 3},
		{"v0.0.0", 0, 0, 0},
		{"v10.20.30", 10, 20, 30},
		{"1.2.3", 1, 2, 3},   // no "v" prefix
		{"v0.7.3", 0, 7, 3},
	}

	for _, tt := range tests {
		t.Run(tt.tag, func(t *testing.T) {
			major, minor, patch, err := parseVersion(tt.tag)
			if err != nil {
				t.Fatalf("parseVersion(%q) error: %v", tt.tag, err)
			}

			if major != tt.wantMajor || minor != tt.wantMinor || patch != tt.wantPatch {
				t.Errorf("parseVersion(%q) = %d.%d.%d, want %d.%d.%d",
					tt.tag, major, minor, patch, tt.wantMajor, tt.wantMinor, tt.wantPatch)
			}
		})
	}
}

func TestParseVersion_Invalid(t *testing.T) {
	tests := []string{
		"",
		"not-a-version",
		"v1.2",
		"v1",
		"abc",
		"v.1.2.3",
		"latest",
	}

	for _, tag := range tests {
		t.Run(tag, func(t *testing.T) {
			_, _, _, err := parseVersion(tag)
			if err == nil {
				t.Errorf("parseVersion(%q) should return error", tag)
			}

			if !strings.Contains(err.Error(), "invalid semver") {
				t.Errorf("error = %v, want mention of invalid semver", err)
			}
		})
	}
}

func TestNewPublishCmd_Valid(t *testing.T) {
	cmd := newPublishCmd()
	if cmd == nil {
		t.Fatal("newPublishCmd returned nil")
	}

	if cmd.Use != "publish" {
		t.Errorf("Use = %q, want publish", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("Short description should not be empty")
	}

	// Verify flags exist.
	flags := []string{"version", "bump", "message", "dry-run", "skip-tests", "watch"}
	for _, name := range flags {
		f := cmd.Flags().Lookup(name)
		if f == nil {
			t.Errorf("expected flag %q to exist", name)
		}
	}
}

func TestNewPublishStatusCmd_Valid(t *testing.T) {
	cmd := newPublishStatusCmd()
	if cmd == nil {
		t.Fatal("newPublishStatusCmd returned nil")
	}

	if cmd.Use != "publish-status" {
		t.Errorf("Use = %q, want publish-status", cmd.Use)
	}

	f := cmd.Flags().Lookup("limit")
	if f == nil {
		t.Fatal("expected --limit flag to exist")
	}

	if f.DefValue != "5" {
		t.Errorf("limit default = %q, want 5", f.DefValue)
	}
}

func TestNewReleaseNotesCmd_Valid(t *testing.T) {
	cmd := newReleaseNotesCmd()
	if cmd == nil {
		t.Fatal("newReleaseNotesCmd returned nil")
	}

	if cmd.Use != "release-notes" {
		t.Errorf("Use = %q, want release-notes", cmd.Use)
	}

	sinceFlag := cmd.Flags().Lookup("since")
	if sinceFlag == nil {
		t.Error("expected --since flag")
	}

	formatFlag := cmd.Flags().Lookup("format")
	if formatFlag == nil {
		t.Fatal("expected --format flag")
	}

	if formatFlag.DefValue != "github" {
		t.Errorf("format default = %q, want github", formatFlag.DefValue)
	}
}

func TestPublishOpts_Struct(t *testing.T) {
	opts := publishOpts{
		version:   "v1.0.0",
		message:   "test release",
		dryRun:    true,
		skipTests: false,
		watch:     true,
	}

	if opts.version != "v1.0.0" {
		t.Errorf("version = %q", opts.version)
	}

	if !opts.dryRun {
		t.Error("dryRun should be true")
	}
}
