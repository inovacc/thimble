package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDetectWorkspaceSingleProject(t *testing.T) {
	dir := t.TempDir()

	ws, err := DetectWorkspace(dir)
	if err != nil {
		t.Fatalf("DetectWorkspace: %v", err)
	}

	if ws.Type != WorkspaceSingle {
		t.Errorf("expected type %q, got %q", WorkspaceSingle, ws.Type)
	}

	if len(ws.Projects) != 1 {
		t.Errorf("expected 1 project, got %d", len(ws.Projects))
	}

	if ws.IsMonorepo() {
		t.Error("single project should not be a monorepo")
	}
}

func TestDetectWorkspaceGoWork(t *testing.T) {
	dir := t.TempDir()

	// Create sub-modules.
	for _, sub := range []string{"svc-a", "svc-b", "lib"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	// Write go.work.
	goWork := `go 1.21

use (
	./svc-a
	./svc-b
	./lib
)
`
	if err := os.WriteFile(filepath.Join(dir, "go.work"), []byte(goWork), 0o644); err != nil {
		t.Fatal(err)
	}

	ws, err := DetectWorkspace(dir)
	if err != nil {
		t.Fatalf("DetectWorkspace: %v", err)
	}

	if ws.Type != WorkspaceGoWork {
		t.Errorf("expected type %q, got %q", WorkspaceGoWork, ws.Type)
	}

	if len(ws.Projects) != 3 {
		t.Errorf("expected 3 projects, got %d: %v", len(ws.Projects), ws.Projects)
	}

	if !ws.IsMonorepo() {
		t.Error("go.work with 3 modules should be a monorepo")
	}
}

func TestDetectWorkspaceGoWorkSingleUse(t *testing.T) {
	dir := t.TempDir()

	if err := os.MkdirAll(filepath.Join(dir, "mymod"), 0o755); err != nil {
		t.Fatal(err)
	}

	goWork := "go 1.21\n\nuse ./mymod\n"
	if err := os.WriteFile(filepath.Join(dir, "go.work"), []byte(goWork), 0o644); err != nil {
		t.Fatal(err)
	}

	ws, err := DetectWorkspace(dir)
	if err != nil {
		t.Fatalf("DetectWorkspace: %v", err)
	}

	if ws.Type != WorkspaceGoWork {
		t.Errorf("expected type %q, got %q", WorkspaceGoWork, ws.Type)
	}

	if len(ws.Projects) != 1 {
		t.Errorf("expected 1 project, got %d", len(ws.Projects))
	}
}

func TestDetectWorkspacePNPM(t *testing.T) {
	dir := t.TempDir()

	// Create package dirs.
	for _, pkg := range []string{"packages/core", "packages/cli"} {
		if err := os.MkdirAll(filepath.Join(dir, pkg), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	pnpm := `packages:
  - "packages/*"
`
	if err := os.WriteFile(filepath.Join(dir, "pnpm-workspace.yaml"), []byte(pnpm), 0o644); err != nil {
		t.Fatal(err)
	}

	ws, err := DetectWorkspace(dir)
	if err != nil {
		t.Fatalf("DetectWorkspace: %v", err)
	}

	if ws.Type != WorkspacePNPM {
		t.Errorf("expected type %q, got %q", WorkspacePNPM, ws.Type)
	}

	if len(ws.Projects) != 2 {
		t.Errorf("expected 2 projects, got %d: %v", len(ws.Projects), ws.Projects)
	}

	if !ws.IsMonorepo() {
		t.Error("pnpm workspace with 2 packages should be a monorepo")
	}
}

func TestDetectWorkspaceVSCode(t *testing.T) {
	dir := t.TempDir()

	// Create folder dirs.
	for _, folder := range []string{"frontend", "backend"} {
		if err := os.MkdirAll(filepath.Join(dir, folder), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	wsFile := codeWorkspaceFolders{
		Folders: []struct {
			Path string `json:"path"`
		}{
			{Path: "frontend"},
			{Path: "backend"},
		},
	}

	data, err := json.Marshal(wsFile)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(dir, "myproject.code-workspace"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	ws, err := DetectWorkspace(dir)
	if err != nil {
		t.Fatalf("DetectWorkspace: %v", err)
	}

	if ws.Type != WorkspaceVSCode {
		t.Errorf("expected type %q, got %q", WorkspaceVSCode, ws.Type)
	}

	if len(ws.Projects) != 2 {
		t.Errorf("expected 2 projects, got %d: %v", len(ws.Projects), ws.Projects)
	}
}

func TestProjectKeySingleBackwardCompat(t *testing.T) {
	dir := "/some/project"

	ws := &Workspace{
		RootDir:  dir,
		Projects: []string{dir},
		Type:     WorkspaceSingle,
	}

	key := ProjectKey(ws)

	// Must match the legacy hash from paths.ProjectDataDir.
	if len(key) != 16 {
		t.Errorf("expected 16-char hex key, got %q (len=%d)", key, len(key))
	}
}

func TestProjectKeyMultiIsDeterministic(t *testing.T) {
	ws1 := &Workspace{
		RootDir:  "/root",
		Projects: []string{"/root/a", "/root/b"},
		Type:     WorkspaceGoWork,
	}

	ws2 := &Workspace{
		RootDir:  "/root",
		Projects: []string{"/root/b", "/root/a"}, // Reversed order.
		Type:     WorkspaceGoWork,
	}

	k1 := ProjectKey(ws1)
	k2 := ProjectKey(ws2)

	if k1 != k2 {
		t.Errorf("project key should be order-independent, got %q and %q", k1, k2)
	}
}

func TestProjectKeyMultiDiffersFromSingle(t *testing.T) {
	single := &Workspace{
		RootDir:  "/root",
		Projects: []string{"/root"},
		Type:     WorkspaceSingle,
	}

	multi := &Workspace{
		RootDir:  "/root",
		Projects: []string{"/root/a", "/root/b"},
		Type:     WorkspaceGoWork,
	}

	k1 := ProjectKey(single)
	k2 := ProjectKey(multi)

	if k1 == k2 {
		t.Error("multi-project key should differ from single-project key")
	}
}
