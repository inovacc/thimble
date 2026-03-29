package plugin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRemoveFromScope_ProjectScope_VerifyFileRemoved(t *testing.T) {
	projectDir := t.TempDir()
	pluginsDir := filepath.Join(projectDir, ".thimble", "plugins")

	if err := os.MkdirAll(pluginsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	content := `{"name":"proj-rm","version":"1.0.0","tools":[{"name":"ctx_prm","description":"t","command":"echo"}]}`
	if err := os.WriteFile(filepath.Join(pluginsDir, "proj-rm.json"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	err := RemoveFromScope("proj-rm", ScopeProject, projectDir)
	if err != nil {
		t.Fatalf("RemoveFromScope project: %v", err)
	}

	if _, err := os.Stat(filepath.Join(pluginsDir, "proj-rm.json")); !os.IsNotExist(err) {
		t.Error("expected project plugin file to be removed")
	}
}

func TestRemoveFromScope_LocalScope_VerifyFileRemoved(t *testing.T) {
	projectDir := t.TempDir()
	pluginsDir := filepath.Join(projectDir, ".thimble", "plugins.local")

	if err := os.MkdirAll(pluginsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	content := `{"name":"local-rm","version":"1.0.0","tools":[{"name":"ctx_lrm","description":"t","command":"echo"}]}`
	if err := os.WriteFile(filepath.Join(pluginsDir, "local-rm.json"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	err := RemoveFromScope("local-rm", ScopeLocal, projectDir)
	if err != nil {
		t.Fatalf("RemoveFromScope local: %v", err)
	}

	if _, err := os.Stat(filepath.Join(pluginsDir, "local-rm.json")); !os.IsNotExist(err) {
		t.Error("expected local plugin file to be removed")
	}
}

func TestRemoveFromScope_ProjectScope_NotFoundError(t *testing.T) {
	projectDir := t.TempDir()
	pluginsDir := filepath.Join(projectDir, ".thimble", "plugins")

	if err := os.MkdirAll(pluginsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	err := RemoveFromScope("ghost", ScopeProject, projectDir)
	if err == nil {
		t.Fatal("expected error for nonexistent plugin in project scope")
	}
}

func TestLoadAllScopes_ProjectAndLocalMerge(t *testing.T) {
	projectDir := t.TempDir()

	// Project-scope plugin.
	projDir := filepath.Join(projectDir, ".thimble", "plugins")
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatal(err)
	}

	contentA := `{"name":"merge-a","version":"1.0.0","tools":[{"name":"ctx_ma","description":"t","command":"echo a"}]}`
	if err := os.WriteFile(filepath.Join(projDir, "merge-a.json"), []byte(contentA), 0o644); err != nil {
		t.Fatal(err)
	}

	// Local-scope plugin (different name).
	localDir := filepath.Join(projectDir, ".thimble", "plugins.local")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatal(err)
	}

	contentB := `{"name":"merge-b","version":"2.0.0","tools":[{"name":"ctx_mb","description":"t","command":"echo b"}]}`
	if err := os.WriteFile(filepath.Join(localDir, "merge-b.json"), []byte(contentB), 0o644); err != nil {
		t.Fatal(err)
	}

	results, err := LoadAllScopes(projectDir)
	if err != nil {
		t.Fatalf("LoadAllScopes: %v", err)
	}

	names := make(map[string]Scope)
	for _, sp := range results {
		names[sp.Name] = sp.Scope
	}

	if _, ok := names["merge-a"]; !ok {
		t.Error("expected merge-a from project scope")
	}

	if scope, ok := names["merge-b"]; !ok {
		t.Error("expected merge-b from local scope")
	} else if scope != ScopeLocal {
		t.Errorf("merge-b scope = %v, want local", scope)
	}
}

func TestLoadAllScopes_LocalOverridesProjectSameName(t *testing.T) {
	projectDir := t.TempDir()

	// Same-named plugin in both project and local.
	projDir := filepath.Join(projectDir, ".thimble", "plugins")
	localDir := filepath.Join(projectDir, ".thimble", "plugins.local")

	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatal(err)
	}

	projContent := `{"name":"shared","version":"1.0.0","tools":[{"name":"ctx_sh","description":"project","command":"echo proj"}]}`
	localContent := `{"name":"shared","version":"3.0.0","tools":[{"name":"ctx_sh","description":"local","command":"echo local"}]}`

	if err := os.WriteFile(filepath.Join(projDir, "shared.json"), []byte(projContent), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(localDir, "shared.json"), []byte(localContent), 0o644); err != nil {
		t.Fatal(err)
	}

	results, err := LoadAllScopes(projectDir)
	if err != nil {
		t.Fatalf("LoadAllScopes: %v", err)
	}

	for _, sp := range results {
		if sp.Name == "shared" {
			if sp.Version != "3.0.0" {
				t.Errorf("expected local version 3.0.0, got %s", sp.Version)
			}

			if sp.Scope != ScopeLocal {
				t.Errorf("expected local scope, got %v", sp.Scope)
			}

			return
		}
	}

	t.Error("expected to find shared plugin")
}
