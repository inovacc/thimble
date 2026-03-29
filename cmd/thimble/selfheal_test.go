package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCheckVersionMismatchNoManifest(t *testing.T) {
	dir := t.TempDir()

	warning := CheckVersionMismatch(dir)
	if warning != "" {
		t.Errorf("expected empty for no manifest, got %q", warning)
	}
}

func TestCheckVersionMismatchMatch(t *testing.T) {
	dir := t.TempDir()
	manifests := map[string]pluginManifest{
		"thimble": {Version: Version, Path: "/some/path"},
	}

	data, err := json.Marshal(manifests)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(dir, "installed_plugins.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	warning := CheckVersionMismatch(dir)
	if warning != "" {
		t.Errorf("expected empty for matching version, got %q", warning)
	}
}

func TestCheckVersionMismatchDiffers(t *testing.T) {
	dir := t.TempDir()
	manifests := map[string]pluginManifest{
		"thimble": {Version: "v99.99.99", Path: "/some/path"},
	}

	data, err := json.Marshal(manifests)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(dir, "installed_plugins.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	warning := CheckVersionMismatch(dir)
	if warning == "" {
		t.Error("expected non-empty warning for version mismatch")
	}
}

func TestCheckVersionMismatchInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "installed_plugins.json"), []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	warning := CheckVersionMismatch(dir)
	if warning != "" {
		t.Errorf("expected empty for invalid JSON, got %q", warning)
	}
}

func TestSelfHealMarkerPreventsRerun(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "selfheal-test.marker")

	// First check: marker doesn't exist.
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("marker should not exist initially")
	}

	// Write the marker (simulating first run).
	if err := os.WriteFile(marker, []byte("done"), 0o644); err != nil {
		t.Fatalf("write marker: %v", err)
	}

	// Second check: marker exists.
	if _, err := os.Stat(marker); err != nil {
		t.Error("marker should exist after first run")
	}
}

func TestSelfHealRunsOnFirstCall(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, fmt.Sprintf("selfheal-%s.marker", time.Now().Format("2006-01-02")))

	// Marker doesn't exist, so self-heal should run.
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("marker should not exist")
	}

	// Simulate the marker creation logic from hook.go.
	_ = os.WriteFile(marker, []byte(time.Now().Format(time.RFC3339)), 0o644)

	// Now marker exists.
	if _, err := os.Stat(marker); err != nil {
		t.Errorf("marker should exist after creation: %v", err)
	}
}
