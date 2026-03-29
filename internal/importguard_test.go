package internal_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDBAccessGuard enforces that only approved packages may import database
// packages (internal/db, internal/store, internal/session) or the SQLite driver.
// Approved: internal/mcp, internal/hooks, internal/delegate.
func TestDBAccessGuard(t *testing.T) {
	forbidden := []string{
		"github.com/inovacc/thimble/internal/db",
		"github.com/inovacc/thimble/internal/store",
		"github.com/inovacc/thimble/internal/session",
		"github.com/inovacc/thimble/internal/hooklog",
		"modernc.org/sqlite",
	}

	// Packages that must NOT import database packages.
	restricted := []string{
		"routing",
		"platform",
		"fetch",
		"security",
		"model",
		"parameters",
	}

	root, err := findModuleRoot()
	if err != nil {
		t.Fatalf("find module root: %v", err)
	}

	// Also check cmd/.
	dirs := make([]string, 0, len(restricted))

	for _, pkg := range restricted {
		dirs = append(dirs, filepath.Join(root, "internal", pkg))
	}

	fset := token.NewFileSet()

	for _, dir := range dirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			continue
		}

		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if info.IsDir() || !strings.HasSuffix(path, ".go") {
				return nil
			}
			// Skip test files — they may import anything for testing.
			if strings.HasSuffix(path, "_test.go") {
				return nil
			}

			f, parseErr := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
			if parseErr != nil {
				t.Errorf("parse %s: %v", path, parseErr)
				return nil
			}

			rel, _ := filepath.Rel(root, path)

			for _, imp := range f.Imports {
				importPath := strings.Trim(imp.Path.Value, `"`)
				for _, fb := range forbidden {
					if importPath == fb || strings.HasPrefix(importPath, fb+"/") {
						t.Errorf("import guard violation: %s imports %s (restricted package may not access database packages)",
							filepath.ToSlash(rel), importPath)
					}
				}
			}

			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", dir, err)
		}
	}
}

func findModuleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", os.ErrNotExist
		}

		dir = parent
	}
}
