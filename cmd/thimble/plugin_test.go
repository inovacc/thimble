package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/inovacc/thimble/internal/plugin"
)

func TestPluginDirNotEmpty(t *testing.T) {
	dir := plugin.PluginDir()
	if dir == "" {
		t.Fatal("PluginDir returned empty string")
	}
}

func TestPluginInitCmd_CreatesFile(t *testing.T) {
	dir := t.TempDir()

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { _ = os.Chdir(origDir) })

	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	cmd := pluginInitCmd
	if err := cmd.RunE(cmd, []string{"test-scaffold"}); err != nil {
		t.Fatalf("runPluginInit: %v", err)
	}

	path := filepath.Join(dir, "test-scaffold.json")

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read created file: %v", err)
	}

	var p plugin.PluginDef
	if err := json.Unmarshal(data, &p); err != nil {
		t.Fatalf("parse created file: %v", err)
	}

	if p.Name != "test-scaffold" {
		t.Errorf("Name = %q, want test-scaffold", p.Name)
	}

	if p.Version != "0.1.0" {
		t.Errorf("Version = %q, want 0.1.0", p.Version)
	}

	if len(p.Tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(p.Tools))
	}

	if p.Tools[0].Name != "ctx_test-scaffold_hello" {
		t.Errorf("tool Name = %q, want ctx_test-scaffold_hello", p.Tools[0].Name)
	}
}

func TestPluginInitCmd_RejectsExisting(t *testing.T) {
	dir := t.TempDir()

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { _ = os.Chdir(origDir) })

	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	// Create the file first.
	if err := os.WriteFile(filepath.Join(dir, "existing.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := pluginInitCmd
	if err := cmd.RunE(cmd, []string{"existing"}); err == nil {
		t.Fatal("expected error for existing file, got nil")
	}
}

func TestPluginCmdRegistered(t *testing.T) {
	// Verify the plugin command is registered on root.
	found := false

	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "plugin" {
			found = true
			// Check subcommands.
			subs := cmd.Commands()
			if len(subs) < 2 {
				t.Fatalf("expected at least 2 subcommands, got %d", len(subs))
			}

			var hasList, hasDir bool

			for _, sub := range subs {
				if sub.Use == "list" {
					hasList = true
				}

				if sub.Use == "dir" {
					hasDir = true
				}
			}

			if !hasList {
				t.Error("missing 'list' subcommand")
			}

			if !hasDir {
				t.Error("missing 'dir' subcommand")
			}

			break
		}
	}

	if !found {
		t.Fatal("plugin command not registered on rootCmd")
	}
}
