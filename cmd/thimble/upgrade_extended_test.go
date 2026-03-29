package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestExtractBinary_DispatchesTarGz(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("tar.gz extraction tested on non-Windows")
	}

	dir := t.TempDir()
	archivePath := filepath.Join(dir, "test.tar.gz")
	destPath := filepath.Join(dir, "thimble")

	payload := []byte("binary content")
	createTarGz(t, archivePath, "thimble", payload)

	if err := extractBinary(archivePath, destPath); err != nil {
		t.Fatalf("extractBinary(.tar.gz) error: %v", err)
	}

	got, _ := os.ReadFile(destPath)
	if string(got) != string(payload) {
		t.Errorf("content = %q, want %q", got, payload)
	}
}

func TestExtractBinary_DispatchesZip(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "test.zip")
	destPath := filepath.Join(dir, binaryName())

	payload := []byte("binary content")
	createZipArchive(t, archivePath, binaryName(), payload)

	if err := extractBinary(archivePath, destPath); err != nil {
		t.Fatalf("extractBinary(.zip) error: %v", err)
	}

	got, _ := os.ReadFile(destPath)
	if string(got) != string(payload) {
		t.Errorf("content = %q, want %q", got, payload)
	}
}

func TestFileSHA256_Upgrade(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.bin")

	if err := os.WriteFile(path, []byte("hello world"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	hash, err := fileSHA256(path)
	if err != nil {
		t.Fatalf("fileSHA256 error: %v", err)
	}

	// SHA256 of "hello world" is well-known.
	want := "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9"
	if hash != want {
		t.Errorf("fileSHA256 = %q, want %q", hash, want)
	}
}

func TestFileSHA256_NotFound_Upgrade(t *testing.T) {
	_, err := fileSHA256("/nonexistent/file")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestGithubReleaseTypes(t *testing.T) {
	// Verify struct serialization.
	r := githubRelease{
		TagName: "v1.0.0",
		Assets: []githubAsset{
			{Name: "thimble_Linux_x86_64.tar.gz", BrowserDownloadURL: "https://example.com/asset", Size: 1024},
		},
	}

	if r.TagName != "v1.0.0" {
		t.Errorf("TagName = %q, want v1.0.0", r.TagName)
	}

	if len(r.Assets) != 1 {
		t.Fatalf("len(Assets) = %d, want 1", len(r.Assets))
	}

	if r.Assets[0].Size != 1024 {
		t.Errorf("Asset.Size = %d, want 1024", r.Assets[0].Size)
	}
}
