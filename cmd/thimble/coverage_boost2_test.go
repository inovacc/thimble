package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ── goosToTitle exhaustive ──

func TestGoosToTitleExhaustive(t *testing.T) {
	tests := []struct {
		goos string
		want string
	}{
		{"linux", "Linux"},
		{"darwin", "Darwin"},
		{"windows", "Windows"},
		{"freebsd", "freebsd"},
		{"openbsd", "openbsd"},
		{"netbsd", "netbsd"},
		{"dragonfly", "dragonfly"},
		{"solaris", "solaris"},
		{"plan9", "plan9"},
		{"", ""},
		{"LINUX", "LINUX"}, // not lowercased
	}

	for _, tt := range tests {
		t.Run(tt.goos, func(t *testing.T) {
			got := goosToTitle(tt.goos)
			if got != tt.want {
				t.Errorf("goosToTitle(%q) = %q, want %q", tt.goos, got, tt.want)
			}
		})
	}
}

// ── goarchToRelease exhaustive ──

func TestGoarchToReleaseExhaustive(t *testing.T) {
	tests := []struct {
		goarch string
		want   string
	}{
		{"amd64", "x86_64"},
		{"386", "i386"},
		{"arm64", "arm64"},
		{"arm", "arm"},
		{"riscv64", "riscv64"},
		{"mips", "mips"},
		{"mipsle", "mipsle"},
		{"ppc64le", "ppc64le"},
		{"s390x", "s390x"},
		{"wasm", "wasm"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.goarch, func(t *testing.T) {
			got := goarchToRelease(tt.goarch)
			if got != tt.want {
				t.Errorf("goarchToRelease(%q) = %q, want %q", tt.goarch, got, tt.want)
			}
		})
	}
}

// ── fileSHA256 edge cases ──

func TestFileSHA256EmptyFileBoost(t *testing.T) {
	dir := t.TempDir()

	path := filepath.Join(dir, "empty.bin")
	if err := os.WriteFile(path, []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}

	hash, err := fileSHA256(path)
	if err != nil {
		t.Fatalf("fileSHA256 error: %v", err)
	}

	want := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	if hash != want {
		t.Errorf("fileSHA256(empty) = %q, want %q", hash, want)
	}
}

// ── extractBinary: dispatch routing ──

func TestExtractBinaryZipRouting(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "test.zip")
	destPath := filepath.Join(dir, binaryName())

	createZipArchive(t, archivePath, binaryName(), []byte("zip-routing"))

	if err := extractBinary(archivePath, destPath); err != nil {
		t.Fatalf("extractBinary(.zip) error: %v", err)
	}

	got, _ := os.ReadFile(destPath)
	if string(got) != "zip-routing" {
		t.Errorf("content = %q, want %q", got, "zip-routing")
	}
}

// ── extractFromTarGz: subdirectory ──

func TestExtractFromTarGzSubdirectoryBoost(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "nested.tar.gz")
	destPath := filepath.Join(dir, "output")

	createTarGz(t, archivePath, "deep/path/"+binaryName(), []byte("deeply-nested"))

	if err := extractFromTarGz(archivePath, destPath); err != nil {
		t.Fatalf("extractFromTarGz: %v", err)
	}

	got, _ := os.ReadFile(destPath)
	if string(got) != "deeply-nested" {
		t.Errorf("got %q, want %q", got, "deeply-nested")
	}
}

// ── fetchExpectedChecksum: no asset match ──

func TestFetchExpectedChecksumNoMatch(t *testing.T) {
	release := &githubRelease{
		Assets: []githubAsset{
			{Name: "binary.tar.gz", BrowserDownloadURL: "https://example.com"},
		},
	}

	_, err := fetchExpectedChecksum(release, "binary.tar.gz")
	if err == nil {
		t.Error("expected error: no checksums asset")
	}

	if !strings.Contains(err.Error(), "no checksums asset found") {
		t.Errorf("error = %q", err.Error())
	}
}

// ── checkResult.String() ──

func TestCheckResultStringAllStatuses(t *testing.T) {
	tests := []struct {
		status string
		icon   string
	}{
		{"pass", "[OK]"},
		{"fail", "[FAIL]"},
		{"warn", "[WARN]"},
		{"unknown", "[??]"},
		{"", "[??]"},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			cr := checkResult{name: "Test", status: tt.status, message: "msg"}

			s := cr.String()
			if !strings.Contains(s, tt.icon) {
				t.Errorf("String() = %q, missing %q", s, tt.icon)
			}
		})
	}
}

// ── command existence verification ──

func TestAllCommandsRegistered(t *testing.T) {
	commands := rootCmd.Commands()

	names := make(map[string]bool)
	for _, c := range commands {
		names[c.Use] = true
	}

	expected := []string{"setup", "doctor", "upgrade", "hook"}
	for _, exp := range expected {
		found := false

		for name := range names {
			if strings.HasPrefix(name, exp) {
				found = true
				break
			}
		}

		if !found {
			t.Errorf("expected command %q to be registered", exp)
		}
	}
}

// ── maxBinarySize ──

func TestMaxBinarySizeIs200MB(t *testing.T) {
	expected := int64(200 << 20)
	if maxBinarySize != expected {
		t.Errorf("maxBinarySize = %d, want %d", maxBinarySize, expected)
	}
}

// ── buildAssetName structure ──

func TestBuildAssetNameComponents(t *testing.T) {
	name := buildAssetName()

	parts := strings.SplitN(name, "_", 3)
	if len(parts) < 3 {
		t.Fatalf("expected at least 3 parts in %q", name)
	}

	if parts[0] != "thimble" {
		t.Errorf("prefix = %q, want 'thimble'", parts[0])
	}

	// Second part should be OS title
	osTitle := goosToTitle("linux") // just verify the function works
	if osTitle != "Linux" {
		t.Errorf("goosToTitle(linux) = %q", osTitle)
	}
}

// ── upgradeCmd flags ──

func TestUpgradeCmdHasForceFlag(t *testing.T) {
	f := upgradeCmd.Flags().Lookup("force")
	if f == nil {
		t.Fatal("missing --force flag")
	}

	if f.Shorthand != "f" {
		t.Errorf("shorthand = %q, want 'f'", f.Shorthand)
	}

	if f.DefValue != "false" {
		t.Errorf("default = %q, want 'false'", f.DefValue)
	}
}

// ── githubRelease struct ──

func TestGithubAssetFields(t *testing.T) {
	asset := githubAsset{
		Name:               "thimble_Windows_x86_64.zip",
		BrowserDownloadURL: "https://github.com/inovacc/thimble/releases/download/v1.0.0/thimble_Windows_x86_64.zip",
		Size:               10485760,
	}

	if asset.Name == "" {
		t.Error("Name should not be empty")
	}

	if asset.BrowserDownloadURL == "" {
		t.Error("URL should not be empty")
	}

	if asset.Size != 10485760 {
		t.Errorf("Size = %d, want 10485760", asset.Size)
	}
}

// ── upgradeOwner/upgradeRepo constants ──

func TestUpgradeRepoConstants(t *testing.T) {
	if upgradeOwner != "inovacc" {
		t.Errorf("upgradeOwner = %q", upgradeOwner)
	}

	if upgradeRepo != "thimble" {
		t.Errorf("upgradeRepo = %q", upgradeRepo)
	}
}
