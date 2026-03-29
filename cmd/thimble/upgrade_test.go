package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestBuildAssetName(t *testing.T) {
	name := buildAssetName()

	if !strings.HasPrefix(name, "thimble_") {
		t.Errorf("buildAssetName() = %q, want prefix 'thimble_'", name)
	}

	switch runtime.GOOS {
	case "windows":
		if !strings.HasSuffix(name, ".zip") {
			t.Errorf("buildAssetName() = %q, want .zip suffix on windows", name)
		}
	default:
		if !strings.HasSuffix(name, ".tar.gz") {
			t.Errorf("buildAssetName() = %q, want .tar.gz suffix on %s", name, runtime.GOOS)
		}
	}
}

func TestGoosToTitle(t *testing.T) {
	tests := []struct {
		goos string
		want string
	}{
		{"linux", "Linux"},
		{"darwin", "Darwin"},
		{"windows", "Windows"},
		{"freebsd", "freebsd"},
	}

	for _, tt := range tests {
		if got := goosToTitle(tt.goos); got != tt.want {
			t.Errorf("goosToTitle(%q) = %q, want %q", tt.goos, got, tt.want)
		}
	}
}

func TestGoarchToRelease(t *testing.T) {
	tests := []struct {
		goarch string
		want   string
	}{
		{"amd64", "x86_64"},
		{"arm64", "arm64"},
		{"386", "i386"},
		{"riscv64", "riscv64"},
	}

	for _, tt := range tests {
		if got := goarchToRelease(tt.goarch); got != tt.want {
			t.Errorf("goarchToRelease(%q) = %q, want %q", tt.goarch, got, tt.want)
		}
	}
}

func TestBinaryName(t *testing.T) {
	name := binaryName()
	if runtime.GOOS == "windows" {
		if name != "thimble.exe" {
			t.Errorf("binaryName() = %q, want 'thimble.exe'", name)
		}
	} else {
		if name != "thimble" {
			t.Errorf("binaryName() = %q, want 'thimble'", name)
		}
	}
}

func TestExtractFromTarGz(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "test.tar.gz")
	destPath := filepath.Join(dir, "thimble")

	// Create a tar.gz with a fake binary.
	payload := []byte("#!/bin/sh\necho hello\n")
	createTarGz(t, archivePath, binaryName(), payload)

	if err := extractFromTarGz(archivePath, destPath); err != nil {
		t.Fatalf("extractFromTarGz() error = %v", err)
	}

	got, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("read extracted file: %v", err)
	}

	if string(got) != string(payload) {
		t.Errorf("extracted content = %q, want %q", got, payload)
	}
}

func TestExtractFromTarGz_NotFound(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "test.tar.gz")
	destPath := filepath.Join(dir, "thimble")

	createTarGz(t, archivePath, "other-binary", []byte("data"))

	err := extractFromTarGz(archivePath, destPath)
	if err == nil {
		t.Fatal("extractFromTarGz() = nil, want error for missing binary")
	}

	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want 'not found'", err.Error())
	}
}

func TestExtractFromZip(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "test.zip")
	destPath := filepath.Join(dir, binaryName())

	payload := []byte("MZ fake exe content")
	createZipArchive(t, archivePath, binaryName(), payload)

	if err := extractFromZip(archivePath, destPath); err != nil {
		t.Fatalf("extractFromZip() error = %v", err)
	}

	got, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("read extracted file: %v", err)
	}

	if string(got) != string(payload) {
		t.Errorf("extracted content = %q, want %q", got, payload)
	}
}

func TestExtractFromZip_NotFound(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "test.zip")
	destPath := filepath.Join(dir, "thimble.exe")

	createZipArchive(t, archivePath, "other.exe", []byte("data"))

	err := extractFromZip(archivePath, destPath)
	if err == nil {
		t.Fatal("extractFromZip() = nil, want error for missing binary")
	}

	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want 'not found'", err.Error())
	}
}

// --- helpers ---

func createTarGz(t *testing.T, path, name string, data []byte) {
	t.Helper()

	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}

	defer func() { _ = f.Close() }()

	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)

	if err := tw.WriteHeader(&tar.Header{
		Name: name,
		Size: int64(len(data)),
		Mode: 0o755,
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := tw.Write(data); err != nil {
		t.Fatal(err)
	}

	_ = tw.Close()
	_ = gw.Close()
}

func createZipArchive(t *testing.T, path, name string, data []byte) {
	t.Helper()

	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}

	defer func() { _ = f.Close() }()

	zw := zip.NewWriter(f)

	w, err := zw.Create(name)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := w.Write(data); err != nil {
		t.Fatal(err)
	}

	_ = zw.Close()
}
