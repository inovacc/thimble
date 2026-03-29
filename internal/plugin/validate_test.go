package plugin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		json       string
		wantValid  bool
		wantErrors int
		wantWarns  int
	}{
		{
			name: "valid plugin with all metadata",
			json: `{
				"name": "test-plugin",
				"version": "1.0.0",
				"description": "A test plugin",
				"author": {"name": "Test Author", "email": "test@example.com"},
				"homepage": "https://example.com",
				"repository": "https://github.com/test/plugin",
				"license": "MIT",
				"keywords": ["test", "example"],
				"tools": [{"name": "ctx_test", "description": "test tool", "command": "echo hello"}]
			}`,
			wantValid:  true,
			wantErrors: 0,
			wantWarns:  0,
		},
		{
			name: "valid plugin with warnings for missing metadata",
			json: `{
				"name": "minimal",
				"tools": [{"name": "ctx_min", "description": "min tool", "command": "echo hi"}]
			}`,
			wantValid:  true,
			wantErrors: 0,
			wantWarns:  4,
		},
		{
			name:       "invalid JSON",
			json:       `{not valid json`,
			wantValid:  false,
			wantErrors: 1,
			wantWarns:  0,
		},
		{
			name:       "missing name",
			json:       `{"tools": [{"name": "ctx_t", "description": "d", "command": "c"}]}`,
			wantValid:  false,
			wantErrors: 1,
			wantWarns:  4, // version + description + author + license
		},
		{
			name:       "no tools",
			json:       `{"name": "empty", "version": "1.0.0"}`,
			wantValid:  false,
			wantErrors: 1,
			wantWarns:  3, // description + author + license
		},
		{
			name: "tool missing ctx_ prefix",
			json: `{
				"name": "bad-prefix",
				"version": "1.0.0",
				"tools": [{"name": "my_tool", "description": "d", "command": "echo"}]
			}`,
			wantValid:  false,
			wantErrors: 1,
			wantWarns:  3, // description + author + license
		},
		{
			name: "tool missing command",
			json: `{
				"name": "no-cmd",
				"version": "1.0.0",
				"tools": [{"name": "ctx_nocmd", "description": "d"}]
			}`,
			wantValid:  false,
			wantErrors: 1,
			wantWarns:  3, // description + author + license
		},
		{
			name: "duplicate tool names",
			json: `{
				"name": "dups",
				"version": "1.0.0",
				"tools": [
					{"name": "ctx_dup", "description": "first", "command": "echo 1"},
					{"name": "ctx_dup", "description": "second", "command": "echo 2"}
				]
			}`,
			wantValid:  false,
			wantErrors: 1,
			wantWarns:  3, // description + author + license
		},
		{
			name: "tool missing description warns",
			json: `{
				"name": "no-desc",
				"version": "1.0.0",
				"tools": [{"name": "ctx_nd", "command": "echo"}]
			}`,
			wantValid:  true,
			wantErrors: 0,
			wantWarns:  4, // description + author + license + tool description
		},
		{
			name: "valid hooks",
			json: `{
				"name": "with-hooks",
				"version": "1.0.0",
				"description": "plugin with hooks",
				"author": {"name": "Test"},
				"license": "MIT",
				"tools": [{"name": "ctx_hk", "description": "d", "command": "echo"}],
				"hooks": {
					"PostToolUse": [{"matcher": "Write|Edit", "command": "fmt --file"}],
					"SessionStart": [{"command": "echo loaded"}]
				}
			}`,
			wantValid:  true,
			wantErrors: 0,
			wantWarns:  0,
		},
		{
			name: "invalid hook event name",
			json: `{
				"name": "bad-event",
				"version": "1.0.0",
				"tools": [{"name": "ctx_be", "description": "d", "command": "echo"}],
				"hooks": {"BadEvent": [{"command": "echo"}]}
			}`,
			wantValid:  false,
			wantErrors: 1,
			wantWarns:  3, // description + author + license
		},
		{
			name: "hook missing command",
			json: `{
				"name": "no-cmd-hook",
				"version": "1.0.0",
				"tools": [{"name": "ctx_nc", "description": "d", "command": "echo"}],
				"hooks": {"PreToolUse": [{"matcher": "Bash"}]}
			}`,
			wantValid:  false,
			wantErrors: 1,
			wantWarns:  3, // description + author + license
		},
		{
			name: "hook invalid matcher regex",
			json: `{
				"name": "bad-regex",
				"version": "1.0.0",
				"tools": [{"name": "ctx_br", "description": "d", "command": "echo"}],
				"hooks": {"PreToolUse": [{"matcher": "[invalid", "command": "echo"}]}
			}`,
			wantValid:  false,
			wantErrors: 1,
			wantWarns:  3, // description + author + license
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			path := filepath.Join(t.TempDir(), "plugin.json")
			if err := os.WriteFile(path, []byte(tt.json), 0o644); err != nil {
				t.Fatal(err)
			}

			result, err := Validate(path)
			if err != nil {
				t.Fatalf("Validate returned error: %v", err)
			}

			if result.Valid != tt.wantValid {
				t.Errorf("Valid = %v, want %v", result.Valid, tt.wantValid)

				for _, issue := range result.Issues {
					t.Logf("  %s: %s", issue.Level, issue.Message)
				}
			}

			errors, warns := 0, 0

			for _, issue := range result.Issues {
				switch issue.Level {
				case "error":
					errors++
				case "warning":
					warns++
				}
			}

			if errors != tt.wantErrors {
				t.Errorf("errors = %d, want %d", errors, tt.wantErrors)

				for _, issue := range result.Issues {
					t.Logf("  %s: %s", issue.Level, issue.Message)
				}
			}

			if warns != tt.wantWarns {
				t.Errorf("warnings = %d, want %d", warns, tt.wantWarns)

				for _, issue := range result.Issues {
					t.Logf("  %s: %s", issue.Level, issue.Message)
				}
			}
		})
	}
}

func TestValidate_FileNotFound(t *testing.T) {
	t.Parallel()

	_, err := Validate("/nonexistent/path.json")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestPluginDef_MetadataFields(t *testing.T) {
	t.Parallel()

	json := `{
		"name": "full-meta",
		"version": "2.1.0",
		"description": "A plugin with full metadata",
		"author": {"name": "Jane Doe", "email": "jane@example.com", "url": "https://jane.dev"},
		"homepage": "https://example.com/full-meta",
		"repository": "https://github.com/jane/full-meta",
		"license": "Apache-2.0",
		"keywords": ["testing", "metadata"],
		"tools": [{"name": "ctx_meta", "description": "meta tool", "command": "echo meta"}]
	}`

	path := filepath.Join(t.TempDir(), "plugin.json")
	if err := os.WriteFile(path, []byte(json), 0o644); err != nil {
		t.Fatal(err)
	}

	p, err := LoadPluginFile(path)
	if err != nil {
		t.Fatalf("LoadPluginFile: %v", err)
	}

	if p.Description != "A plugin with full metadata" {
		t.Errorf("Description = %q", p.Description)
	}

	if p.Author == nil || p.Author.Name != "Jane Doe" {
		t.Errorf("Author = %+v", p.Author)
	}

	if p.Author.Email != "jane@example.com" {
		t.Errorf("Author.Email = %q", p.Author.Email)
	}

	if p.Homepage != "https://example.com/full-meta" {
		t.Errorf("Homepage = %q", p.Homepage)
	}

	if p.Repository != "https://github.com/jane/full-meta" {
		t.Errorf("Repository = %q", p.Repository)
	}

	if p.License != "Apache-2.0" {
		t.Errorf("License = %q", p.License)
	}

	if len(p.Keywords) != 2 || p.Keywords[0] != "testing" {
		t.Errorf("Keywords = %v", p.Keywords)
	}
}
