package plugin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseScope_Valid(t *testing.T) {
	tests := []struct {
		input string
		want  Scope
	}{
		{"user", ScopeUser},
		{"project", ScopeProject},
		{"local", ScopeLocal},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseScope(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != tt.want {
				t.Errorf("ParseScope(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseScope_Invalid(t *testing.T) {
	_, err := ParseScope("global")
	if err == nil {
		t.Fatal("expected error for invalid scope")
	}
}

func TestScopeDir_User(t *testing.T) {
	dir := ScopeDir(ScopeUser, "/some/project")
	if dir != PluginDir() {
		t.Errorf("ScopeDir(user) = %q, want %q", dir, PluginDir())
	}
}

func TestScopeDir_Project(t *testing.T) {
	dir := ScopeDir(ScopeProject, "/some/project")
	want := filepath.Join("/some/project", ".thimble", "plugins")

	if dir != want {
		t.Errorf("ScopeDir(project) = %q, want %q", dir, want)
	}
}

func TestScopeDir_Local(t *testing.T) {
	dir := ScopeDir(ScopeLocal, "/some/project")
	want := filepath.Join("/some/project", ".thimble", "plugins.local")

	if dir != want {
		t.Errorf("ScopeDir(local) = %q, want %q", dir, want)
	}
}

// validPluginJSON returns a minimal valid plugin JSON for testing.
func validPluginJSON(name string) []byte {
	return []byte(`{"name":"` + name + `","version":"1.0.0","tools":[{"name":"ctx_` + name + `","description":"test","command":"echo ok"}]}`)
}

func TestLoadAllScopes_UserOnly(t *testing.T) {
	// Without a projectDir, only user scope is loaded.
	// Use an empty temp dir to avoid loading real plugins.
	// Since LoadAllScopes calls PluginDir() which uses the real data dir,
	// we test the merging logic with a projectDir that has no plugins.
	projectDir := t.TempDir()

	plugins, err := LoadAllScopes(projectDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should not error even if dirs don't exist.
	_ = plugins
}

func TestLoadAllScopes_EmptyProjectDir(t *testing.T) {
	// Empty projectDir means only user scope is loaded.
	plugins, err := LoadAllScopes("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_ = plugins
}

func TestLoadAllScopes_ScopeOverride(t *testing.T) {
	projectDir := t.TempDir()

	// Create user-scope plugin dir.
	userDir := t.TempDir()

	// Create project-scope plugin dir.
	projDir := filepath.Join(projectDir, ".thimble", "plugins")
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Write same-named plugin to both dirs with different versions.
	userPlugin := `{"name":"test","version":"1.0.0","tools":[{"name":"ctx_test","description":"user version","command":"echo user"}]}`
	projPlugin := `{"name":"test","version":"2.0.0","tools":[{"name":"ctx_test","description":"project version","command":"echo project"}]}`

	if err := os.WriteFile(filepath.Join(userDir, "test.json"), []byte(userPlugin), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(projDir, "test.json"), []byte(projPlugin), 0o644); err != nil {
		t.Fatal(err)
	}

	// Test the override by loading from user dir first, then project.
	// We can't override PluginDir() easily, so test the merging logic directly.
	userPlugins, err := LoadPlugins(userDir)
	if err != nil {
		t.Fatal(err)
	}

	projPlugins, err := LoadPlugins(projDir)
	if err != nil {
		t.Fatal(err)
	}

	// Simulate the merge: project should override user.
	seen := make(map[string]int)

	var result []ScopedPlugin

	for _, p := range userPlugins {
		sp := ScopedPlugin{PluginDef: p, Scope: ScopeUser}
		seen[p.Name] = len(result)
		result = append(result, sp)
	}

	for _, p := range projPlugins {
		sp := ScopedPlugin{PluginDef: p, Scope: ScopeProject}

		if idx, exists := seen[p.Name]; exists {
			result[idx] = sp
		} else {
			seen[p.Name] = len(result)
			result = append(result, sp)
		}
	}

	if len(result) != 1 {
		t.Fatalf("expected 1 merged plugin, got %d", len(result))
	}

	if result[0].Version != "2.0.0" {
		t.Errorf("expected project version 2.0.0, got %s", result[0].Version)
	}

	if result[0].Scope != ScopeProject {
		t.Errorf("expected project scope, got %s", result[0].Scope)
	}
}

func TestLoadAllScopes_LocalOverridesProject(t *testing.T) {
	projectDir := t.TempDir()

	// Create project and local scope dirs.
	projDir := filepath.Join(projectDir, ".thimble", "plugins")
	localDir := filepath.Join(projectDir, ".thimble", "plugins.local")

	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatal(err)
	}

	projPlugin := `{"name":"override","version":"1.0.0","tools":[{"name":"ctx_override","description":"project","command":"echo project"}]}`
	localPlugin := `{"name":"override","version":"3.0.0","tools":[{"name":"ctx_override","description":"local","command":"echo local"}]}`

	if err := os.WriteFile(filepath.Join(projDir, "override.json"), []byte(projPlugin), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(localDir, "override.json"), []byte(localPlugin), 0o644); err != nil {
		t.Fatal(err)
	}

	// LoadAllScopes should pick local over project.
	plugins, err := LoadAllScopes(projectDir)
	if err != nil {
		t.Fatal(err)
	}

	// Find the "override" plugin in results.
	var found *ScopedPlugin

	for i := range plugins {
		if plugins[i].Name == "override" {
			found = &plugins[i]
			break
		}
	}

	if found == nil {
		t.Fatal("override plugin not found in results")
	}

	if found.Version != "3.0.0" {
		t.Errorf("expected local version 3.0.0, got %s", found.Version)
	}

	if found.Scope != ScopeLocal {
		t.Errorf("expected local scope, got %s", found.Scope)
	}
}

func TestRemoveFromScope(t *testing.T) {
	projectDir := t.TempDir()
	projDir := filepath.Join(projectDir, ".thimble", "plugins")

	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(projDir, "myplugin.json"), validPluginJSON("myplugin"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Remove from project scope.
	if err := RemoveFromScope("myplugin", ScopeProject, projectDir); err != nil {
		t.Fatalf("remove failed: %v", err)
	}

	// Verify gone.
	if _, err := os.Stat(filepath.Join(projDir, "myplugin.json")); !os.IsNotExist(err) {
		t.Error("plugin file should be removed")
	}
}

func TestRemoveFromScope_NotFound(t *testing.T) {
	projectDir := t.TempDir()

	err := RemoveFromScope("nonexistent", ScopeProject, projectDir)
	if err == nil {
		t.Fatal("expected error for nonexistent plugin")
	}
}

func TestInstallToScope_ProjectDir(t *testing.T) {
	// We can't test actual HTTP downloads, but we can verify the directory is created.
	projectDir := t.TempDir()
	projDir := filepath.Join(projectDir, ".thimble", "plugins")

	// Verify dir doesn't exist yet.
	if _, err := os.Stat(projDir); !os.IsNotExist(err) {
		t.Fatal("project plugin dir should not exist yet")
	}

	// InstallToScope with an invalid source will fail on HTTP, but the dir should be created.
	_, _ = InstallToScope("nonexistent-plugin-that-wont-resolve", ScopeProject, projectDir)

	// The directory should have been created even though download failed.
	if _, err := os.Stat(projDir); os.IsNotExist(err) {
		t.Error("project plugin dir should have been created")
	}
}
