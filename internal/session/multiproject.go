package session

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ErrNoWorkspaceProjects is returned when a workspace marker is found but no projects are resolved.
var ErrNoWorkspaceProjects = errors.New("workspace marker found but no projects resolved")

// WorkspaceType indicates the kind of multi-project workspace detected.
type WorkspaceType string

const (
	WorkspaceSingle    WorkspaceType = "single"
	WorkspacePNPM      WorkspaceType = "pnpm"
	WorkspaceVSCode    WorkspaceType = "vscode"
	WorkspaceGoWork    WorkspaceType = "go-work"
)

// Workspace describes a multi-project workspace (monorepo or multi-root).
type Workspace struct {
	RootDir  string        `json:"root_dir"`
	Projects []string      `json:"projects"`
	Type     WorkspaceType `json:"type"`
}

// IsMonorepo returns true if the workspace contains more than one project.
func (w *Workspace) IsMonorepo() bool {
	return len(w.Projects) > 1
}

// DetectWorkspace inspects dir for workspace markers and returns the detected
// workspace. If no multi-project marker is found, falls back to a single-project
// workspace rooted at dir.
func DetectWorkspace(dir string) (*Workspace, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}

	// 1. pnpm-workspace.yaml
	if ws, err := detectPNPM(abs); err == nil && ws != nil {
		return ws, nil
	}

	// 2. *.code-workspace (VS Code multi-root)
	if ws, err := detectVSCode(abs); err == nil && ws != nil {
		return ws, nil
	}

	// 3. go.work (Go workspace)
	if ws, err := detectGoWork(abs); err == nil && ws != nil {
		return ws, nil
	}

	// Fallback: single project.
	return &Workspace{
		RootDir:  abs,
		Projects: []string{abs},
		Type:     WorkspaceSingle,
	}, nil
}

// ProjectKey produces a stable hash key for a workspace. For single-project
// workspaces, this matches the legacy per-project hash so existing sessions
// continue to work.
func ProjectKey(ws *Workspace) string {
	if len(ws.Projects) <= 1 {
		// Backward-compatible: hash the single project dir exactly as
		// paths.ProjectDataDir does.
		dir := ws.RootDir
		if len(ws.Projects) == 1 {
			dir = ws.Projects[0]
		}

		h := sha256.Sum256([]byte(dir))

		return hex.EncodeToString(h[:8])
	}

	sorted := make([]string, len(ws.Projects))
	copy(sorted, ws.Projects)
	sort.Strings(sorted)

	h := sha256.Sum256([]byte(strings.Join(sorted, "\x00")))

	return hex.EncodeToString(h[:8])
}

// detectPNPM checks for pnpm-workspace.yaml and extracts package globs.
func detectPNPM(dir string) (*Workspace, error) {
	path := filepath.Join(dir, "pnpm-workspace.yaml")

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Simple YAML parsing: extract lines under "packages:" that start with "- ".
	var projects []string

	lines := strings.Split(string(data), "\n")
	inPackages := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "packages:" {
			inPackages = true

			continue
		}

		if inPackages {
			if after, ok := strings.CutPrefix(trimmed, "- "); ok {
				glob := strings.Trim(after, "\"' ")
				// Resolve globs to actual directories.
				matches, err := filepath.Glob(filepath.Join(dir, glob))
				if err == nil {
					for _, m := range matches {
						if info, err := os.Stat(m); err == nil && info.IsDir() {
							projects = append(projects, m)
						}
					}
				}
			} else if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
				break // End of packages section.
			}
		}
	}

	if len(projects) == 0 {
		// Marker exists but no packages resolved — still report as pnpm.
		return &Workspace{
			RootDir:  dir,
			Projects: []string{dir},
			Type:     WorkspacePNPM,
		}, nil
	}

	return &Workspace{
		RootDir:  dir,
		Projects: projects,
		Type:     WorkspacePNPM,
	}, nil
}

// codeWorkspaceFolders is the minimal structure of a .code-workspace file.
type codeWorkspaceFolders struct {
	Folders []struct {
		Path string `json:"path"`
	} `json:"folders"`
}

// detectVSCode checks for *.code-workspace files.
func detectVSCode(dir string) (*Workspace, error) {
	matches, err := filepath.Glob(filepath.Join(dir, "*.code-workspace"))
	if err != nil || len(matches) == 0 {
		return nil, err
	}

	data, err := os.ReadFile(matches[0])
	if err != nil {
		return nil, err
	}

	var wsFile codeWorkspaceFolders
	if err := json.Unmarshal(data, &wsFile); err != nil {
		return nil, err
	}

	var projects []string

	for _, f := range wsFile.Folders {
		p := f.Path
		if !filepath.IsAbs(p) {
			p = filepath.Join(dir, p)
		}

		if info, err := os.Stat(p); err == nil && info.IsDir() {
			projects = append(projects, p)
		}
	}

	if len(projects) == 0 {
		return nil, ErrNoWorkspaceProjects
	}

	return &Workspace{
		RootDir:  dir,
		Projects: projects,
		Type:     WorkspaceVSCode,
	}, nil
}

// detectGoWork checks for go.work and extracts "use" directives.
func detectGoWork(dir string) (*Workspace, error) {
	path := filepath.Join(dir, "go.work")

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var projects []string

	lines := strings.Split(string(data), "\n")
	inUse := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Single-line: use ./path
		if strings.HasPrefix(trimmed, "use ") && !strings.HasPrefix(trimmed, "use (") {
			p := strings.TrimSpace(strings.TrimPrefix(trimmed, "use"))
			if abs := resolveDir(dir, p); abs != "" {
				projects = append(projects, abs)
			}

			continue
		}

		if trimmed == "use (" {
			inUse = true

			continue
		}

		if inUse {
			if trimmed == ")" {
				inUse = false

				continue
			}

			if trimmed != "" && !strings.HasPrefix(trimmed, "//") {
				if abs := resolveDir(dir, trimmed); abs != "" {
					projects = append(projects, abs)
				}
			}
		}
	}

	if len(projects) == 0 {
		return nil, ErrNoWorkspaceProjects
	}

	return &Workspace{
		RootDir:  dir,
		Projects: projects,
		Type:     WorkspaceGoWork,
	}, nil
}

// resolveDir resolves a possibly-relative path against base and returns the
// absolute path if it exists as a directory, otherwise "".
func resolveDir(base, rel string) string {
	p := rel
	if !filepath.IsAbs(p) {
		p = filepath.Join(base, p)
	}

	p = filepath.Clean(p)

	if info, err := os.Stat(p); err == nil && info.IsDir() {
		return p
	}

	return ""
}
