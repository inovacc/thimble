package paths

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// setDataDirEnv overrides the env vars that DataDir() reads, per-platform.
func setDataDirEnv(t *testing.T, dir string) {
	t.Helper()

	switch runtime.GOOS {
	case "windows":
		t.Setenv("LOCALAPPDATA", dir)
	case "darwin":
		t.Setenv("HOME", dir)
	default:
		t.Setenv("HOME", dir)
	}
}

func TestMigrateProjectsToSessions_NoOldDir(t *testing.T) {
	// When projects/ doesn't exist, MigrateProjectsToSessions should be a no-op.
	// We just ensure it doesn't panic.
	MigrateProjectsToSessions()
}

func TestMigrateProjectsToSessions_SessionsAlreadyExists(t *testing.T) {
	tmpDir := t.TempDir()
	// Override LOCALAPPDATA so DataDir() returns tmpDir\Thimble.
	setDataDirEnv(t, tmpDir)

	base := DataDir()
	oldBase := filepath.Join(base, "projects")
	newBase := filepath.Join(base, "sessions")

	if err := os.MkdirAll(oldBase, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(newBase, 0o755); err != nil {
		t.Fatal(err)
	}

	// Write a file in projects/ that should NOT be migrated.
	if err := os.WriteFile(filepath.Join(oldBase, "test.txt"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	MigrateProjectsToSessions()

	// projects/ should still exist (not renamed since sessions/ already existed).
	if _, err := os.Stat(oldBase); err != nil {
		t.Errorf("projects/ should still exist when sessions/ already exists: %v", err)
	}
}

func TestMigrateProjectsToSessions_SuccessfulMigration(t *testing.T) {
	tmpDir := t.TempDir()
	setDataDirEnv(t, tmpDir)

	base := DataDir()
	oldBase := filepath.Join(base, "projects")
	newBase := filepath.Join(base, "sessions")

	if err := os.MkdirAll(oldBase, 0o755); err != nil {
		t.Fatal(err)
	}

	// Write a file in projects/.
	if err := os.WriteFile(filepath.Join(oldBase, "data.txt"), []byte("migrated"), 0o644); err != nil {
		t.Fatal(err)
	}

	MigrateProjectsToSessions()

	// projects/ should be gone.
	if _, err := os.Stat(oldBase); !os.IsNotExist(err) {
		t.Errorf("projects/ should be renamed away, but stat returned: %v", err)
	}

	// sessions/ should exist with the file.
	data, err := os.ReadFile(filepath.Join(newBase, "data.txt"))
	if err != nil {
		t.Fatalf("sessions/data.txt should exist: %v", err)
	}

	if string(data) != "migrated" {
		t.Errorf("data.txt content = %q, want 'migrated'", string(data))
	}
}

func TestMigrateProjectsToSessions_OldIsFile(t *testing.T) {
	tmpDir := t.TempDir()
	setDataDirEnv(t, tmpDir)

	base := DataDir()
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}

	oldBase := filepath.Join(base, "projects")

	// projects is a file, not a directory.
	if err := os.WriteFile(oldBase, []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Should not panic, should be a no-op.
	MigrateProjectsToSessions()

	// File should still be there.
	if _, err := os.Stat(oldBase); err != nil {
		t.Errorf("projects file should remain: %v", err)
	}
}

func TestDataDir_FallbackNoEnv(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Setenv("LOCALAPPDATA", "")
	}

	dir := DataDir()
	if dir == "" {
		t.Error("DataDir should not return empty")
	}
}

func TestConfigDir_FallbackNoEnv(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Setenv("APPDATA", "")
	}

	dir := ConfigDir()
	if dir == "" {
		t.Error("ConfigDir should not return empty")
	}
}
