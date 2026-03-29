package paths

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestProjectDataDir(t *testing.T) {
	dataDir := DataDir()

	tests := []struct {
		name       string
		projectDir string
	}{
		{"simple", "/home/user/project"},
		{"nested", "/a/b/c/d"},
		{"windows-style", `C:\Users\dev\project`},
		{"empty", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ProjectDataDir(tt.projectDir)

			// Must be under the global data dir.
			if !strings.HasPrefix(got, dataDir) {
				t.Errorf("ProjectDataDir(%q) = %q, want prefix %q", tt.projectDir, got, dataDir)
			}

			// Must contain "sessions" segment.
			if !strings.Contains(got, "sessions") {
				t.Errorf("ProjectDataDir(%q) = %q, want 'sessions' in path", tt.projectDir, got)
			}

			// Leaf must be the 16-char hex digest.
			leaf := filepath.Base(got)
			if len(leaf) != 16 {
				t.Errorf("ProjectDataDir(%q) leaf = %q, want 16-char hex digest", tt.projectDir, leaf)
			}

			// Digest must match SHA256 of the input.
			h := sha256.Sum256([]byte(tt.projectDir))

			want := hex.EncodeToString(h[:8])
			if leaf != want {
				t.Errorf("ProjectDataDir(%q) digest = %q, want %q", tt.projectDir, leaf, want)
			}
		})
	}
}

func TestProjectDataDir_Deterministic(t *testing.T) {
	a := ProjectDataDir("/some/project")
	b := ProjectDataDir("/some/project")

	if a != b {
		t.Errorf("not deterministic: %q != %q", a, b)
	}
}

func TestProjectDataDir_DifferentProjectsDifferentPaths(t *testing.T) {
	a := ProjectDataDir("/project-a")
	b := ProjectDataDir("/project-b")

	if a == b {
		t.Errorf("different projects should produce different paths: %q", a)
	}
}

func TestPluginDataDir_EnvOverride(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("THIMBLE_PLUGIN_DATA", tmpDir)

	got := PluginDataDir()
	if got != tmpDir {
		t.Errorf("PluginDataDir() = %q, want %q", got, tmpDir)
	}
}

func TestPluginDataDir_FallbackToDataDir(t *testing.T) {
	t.Setenv("THIMBLE_PLUGIN_DATA", "")

	got := PluginDataDir()
	want := DataDir()

	if got != want {
		t.Errorf("PluginDataDir() = %q, want DataDir() = %q", got, want)
	}
}

func TestProjectDataDir_UsesPluginDataDir(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("THIMBLE_PLUGIN_DATA", tmpDir)

	got := ProjectDataDir("/some/project")
	if !strings.HasPrefix(got, tmpDir) {
		t.Errorf("ProjectDataDir() = %q, want prefix %q (from THIMBLE_PLUGIN_DATA)", got, tmpDir)
	}
}

func TestDataDirReturnsNonEmpty(t *testing.T) {
	dir := DataDir()
	if dir == "" {
		t.Error("DataDir() returned empty string")
	}

	if runtime.GOOS == "windows" {
		if !filepath.IsAbs(dir) {
			t.Errorf("DataDir() on Windows should be absolute, got %q", dir)
		}
	}
}

func TestConfigDirReturnsNonEmpty(t *testing.T) {
	dir := ConfigDir()
	if dir == "" {
		t.Error("ConfigDir() returned empty string")
	}

	if runtime.GOOS == "windows" {
		if !filepath.IsAbs(dir) {
			t.Errorf("ConfigDir() on Windows should be absolute, got %q", dir)
		}
	}
}

func TestDataDirContainsThimble(t *testing.T) {
	dir := DataDir()
	base := filepath.Base(dir)

	// On all platforms, the leaf dir should reference thimble.
	switch runtime.GOOS {
	case "darwin":
		if base != "thimble" {
			t.Errorf("DataDir() leaf = %q, want thimble (macOS)", base)
		}
	case "windows":
		if base != "Thimble" {
			t.Errorf("DataDir() leaf = %q, want Thimble (Windows)", base)
		}
	default:
		if base != ".thimble" {
			t.Errorf("DataDir() leaf = %q, want .thimble (Linux)", base)
		}
	}
}

func TestConfigDirContainsThimble(t *testing.T) {
	dir := ConfigDir()
	base := filepath.Base(dir)

	switch runtime.GOOS {
	case "darwin":
		if base != "thimble" {
			t.Errorf("ConfigDir() leaf = %q, want thimble (macOS)", base)
		}
	case "windows":
		if base != "Thimble" {
			t.Errorf("ConfigDir() leaf = %q, want Thimble (Windows)", base)
		}
	default:
		if base != "thimble" {
			t.Errorf("ConfigDir() leaf = %q, want thimble (Linux)", base)
		}
	}
}

func TestDataDirPlatformSpecific(t *testing.T) {
	dir := DataDir()

	switch runtime.GOOS {
	case "darwin":
		if !strings.Contains(dir, filepath.Join("Library", "Application Support")) {
			t.Errorf("DataDir() on macOS should contain Library/Application Support, got %q", dir)
		}
	case "windows":
		// Should be under LOCALAPPDATA or fallback to ~/.thimble.
		if !filepath.IsAbs(dir) {
			t.Errorf("DataDir() on Windows should be absolute, got %q", dir)
		}
	default:
		home, _ := os.UserHomeDir()

		want := filepath.Join(home, ".thimble")
		if dir != want {
			t.Errorf("DataDir() on Linux = %q, want %q", dir, want)
		}
	}
}

func TestConfigDirPlatformSpecific(t *testing.T) {
	dir := ConfigDir()

	switch runtime.GOOS {
	case "darwin":
		if !strings.Contains(dir, filepath.Join("Library", "Application Support")) {
			t.Errorf("ConfigDir() on macOS should contain Library/Application Support, got %q", dir)
		}
	case "windows":
		if !filepath.IsAbs(dir) {
			t.Errorf("ConfigDir() on Windows should be absolute, got %q", dir)
		}
	default:
		home, _ := os.UserHomeDir()

		want := filepath.Join(home, ".config", "thimble")
		if dir != want {
			t.Errorf("ConfigDir() on Linux = %q, want %q", dir, want)
		}
	}
}

func TestIsGitWorktreeNormalRepo(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	if IsGitWorktree(dir) {
		t.Error("expected false for normal .git directory")
	}
}

func TestIsGitWorktreeActualWorktree(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".git"), []byte("gitdir: /some/path/.git/worktrees/branch\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if !IsGitWorktree(dir) {
		t.Error("expected true for .git file (worktree)")
	}
}

func TestIsGitWorktreeNoGit(t *testing.T) {
	dir := t.TempDir()
	if IsGitWorktree(dir) {
		t.Error("expected false when no .git exists")
	}
}

func TestWorktreeSessionSuffixNonWorktree(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	suffix := WorktreeSessionSuffix(dir)
	if suffix != "" {
		t.Errorf("expected empty suffix for normal repo, got %q", suffix)
	}
}

func TestWorktreeSessionSuffixWorktree(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".git"), []byte("gitdir: /some/path\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	suffix := WorktreeSessionSuffix(dir)
	if suffix == "" {
		t.Error("expected non-empty suffix for worktree")
	}

	if len(suffix) != 8 {
		t.Errorf("expected 8-char hex suffix, got %q", suffix)
	}
}
