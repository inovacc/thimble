package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const (
	upgradeOwner = "inovacc"
	upgradeRepo  = "thimble"

	// maxBinarySize guards against decompression bombs (200 MB).
	maxBinarySize = 200 << 20
)

var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Self-update from GitHub Releases",
	RunE:  runUpgrade,
}

var upgradeForce bool

func init() {
	rootCmd.AddCommand(upgradeCmd)
	upgradeCmd.Flags().BoolVarP(&upgradeForce, "force", "f", false, "Force upgrade even if already up to date")
}

type githubRelease struct {
	TagName string        `json:"tag_name"`
	Assets  []githubAsset `json:"assets"`
}

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

func runUpgrade(_ *cobra.Command, _ []string) error {
	currentVersion := strings.TrimPrefix(Version, "v")
	_, _ = fmt.Fprintf(os.Stderr, "Current version: %s (%s/%s)\n", currentVersion, runtime.GOOS, runtime.GOARCH)

	_, _ = fmt.Fprintf(os.Stderr, "Checking for updates...\n")

	release, err := fetchLatestRelease()
	if err != nil {
		return fmt.Errorf("fetch latest release: %w", err)
	}

	latestVersion := strings.TrimPrefix(release.TagName, "v")
	_, _ = fmt.Fprintf(os.Stderr, "Latest version: %s\n", latestVersion)

	if latestVersion == currentVersion && !upgradeForce {
		_, _ = fmt.Fprintln(os.Stderr, "Already up to date.")
		return nil
	}

	assetName := buildAssetName()

	var asset *githubAsset

	for i := range release.Assets {
		if release.Assets[i].Name == assetName {
			asset = &release.Assets[i]
			break
		}
	}

	if asset == nil {
		_, _ = fmt.Fprintf(os.Stderr, "Available assets:\n")
		for _, a := range release.Assets {
			_, _ = fmt.Fprintf(os.Stderr, "  - %s\n", a.Name)
		}

		return fmt.Errorf("no asset found for %s/%s (expected %s)", runtime.GOOS, runtime.GOARCH, assetName)
	}

	_, _ = fmt.Fprintf(os.Stderr, "Downloading %s (%d bytes)...\n", asset.Name, asset.Size)

	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable path: %w", err)
	}

	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return fmt.Errorf("resolve symlinks: %w", err)
	}

	// Download archive to temp file.
	tmpArchive := execPath + ".archive"
	if err := downloadFile(asset.BrowserDownloadURL, tmpArchive); err != nil {
		_ = os.Remove(tmpArchive)
		return fmt.Errorf("download: %w", err)
	}

	// Verify archive checksum before extraction.
	actualHash, err := fileSHA256(tmpArchive)
	if err != nil {
		_ = os.Remove(tmpArchive)
		return fmt.Errorf("compute checksum: %w", err)
	}

	expectedHash, err := fetchExpectedChecksum(release, asset.Name)
	if err == nil {
		if actualHash != expectedHash {
			_ = os.Remove(tmpArchive)
			return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedHash, actualHash)
		}

		_, _ = fmt.Fprintf(os.Stderr, "Checksum verified: %s\n", actualHash[:16]+"...")
	} else {
		_, _ = fmt.Fprintf(os.Stderr, "SHA256: %s (no checksums file to verify against)\n", actualHash[:16]+"...")
	}

	// Extract binary from archive.
	tmpPath := execPath + ".new"
	if err := extractBinary(tmpArchive, tmpPath); err != nil {
		_ = os.Remove(tmpArchive)
		_ = os.Remove(tmpPath)

		return fmt.Errorf("extract binary: %w", err)
	}

	_ = os.Remove(tmpArchive)

	// Atomic swap.
	oldPath := execPath + ".old"
	_ = os.Remove(oldPath)

	if err := os.Rename(execPath, oldPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("backup current binary: %w", err)
	}

	if err := os.Rename(tmpPath, execPath); err != nil {
		_ = os.Rename(oldPath, execPath)
		return fmt.Errorf("install new binary: %w", err)
	}

	postInstallChmod(execPath)

	_ = os.Remove(oldPath)

	_, _ = fmt.Fprintf(os.Stderr, "Upgraded: %s -> %s\n", currentVersion, latestVersion)

	return nil
}

// buildAssetName returns the GoReleaser archive name for the current platform.
func buildAssetName() string {
	osName := goosToTitle(runtime.GOOS)
	archName := goarchToRelease(runtime.GOARCH)

	return fmt.Sprintf("thimble_%s_%s%s", osName, archName, archiveExt())
}

// goosToTitle maps GOOS to GoReleaser's {{ title .Os }}.
func goosToTitle(goos string) string {
	switch goos {
	case "linux":
		return "Linux"
	case "darwin":
		return "Darwin"
	case "windows":
		return "Windows"
	default:
		return goos
	}
}

// goarchToRelease maps GOARCH to GoReleaser's arch template.
func goarchToRelease(goarch string) string {
	switch goarch {
	case "amd64":
		return "x86_64"
	case "386":
		return "i386"
	default:
		return goarch
	}
}

// binaryName and archiveExt are in upgrade_windows.go / upgrade_unix.go.

// extractBinary extracts the thimble binary from an archive to destPath.
func extractBinary(archivePath, destPath string) error {
	if strings.HasSuffix(archivePath, ".zip") {
		return extractFromZip(archivePath, destPath)
	}

	return extractFromTarGz(archivePath, destPath)
}

func extractFromTarGz(archivePath, destPath string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}

	defer func() { _ = f.Close() }()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("gzip: %w", err)
	}

	defer func() { _ = gz.Close() }()

	tr := tar.NewReader(gz)
	target := binaryName()

	for {
		hdr, tarErr := tr.Next()
		if tarErr == io.EOF {
			break
		}

		if tarErr != nil {
			return fmt.Errorf("tar: %w", tarErr)
		}

		// Match by base name — archives may have directory prefixes.
		if filepath.Base(hdr.Name) != target {
			continue
		}

		out, createErr := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
		if createErr != nil {
			return createErr
		}

		_, copyErr := io.Copy(out, io.LimitReader(tr, maxBinarySize))
		_ = out.Close()

		if copyErr != nil {
			return fmt.Errorf("extract %s: %w", target, copyErr)
		}

		return nil
	}

	return fmt.Errorf("binary %q not found in archive", target)
}

func extractFromZip(archivePath, destPath string) error {
	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("zip: %w", err)
	}

	defer func() { _ = zr.Close() }()

	target := binaryName()

	for _, zf := range zr.File {
		if filepath.Base(zf.Name) != target {
			continue
		}

		rc, openErr := zf.Open()
		if openErr != nil {
			return openErr
		}

		out, createErr := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
		if createErr != nil {
			_ = rc.Close()
			return createErr
		}

		_, copyErr := io.Copy(out, io.LimitReader(rc, maxBinarySize))
		_ = rc.Close()
		_ = out.Close()

		if copyErr != nil {
			return fmt.Errorf("extract %s: %w", target, copyErr)
		}

		return nil
	}

	return fmt.Errorf("binary %q not found in archive", target)
}

// fetchLatestReleaseFunc is the function used to fetch the latest release.
// It can be overridden in tests.
var fetchLatestReleaseFunc = fetchLatestReleaseHTTP

func fetchLatestRelease() (*githubRelease, error) {
	return fetchLatestReleaseFunc()
}

func fetchLatestReleaseHTTP() (*githubRelease, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", upgradeOwner, upgradeRepo)

	httpClient := &http.Client{Timeout: 15 * time.Second}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "thimble/"+Version)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("GitHub API returned %d: %s", resp.StatusCode, string(body))
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &release, nil
}

func downloadFile(url, dest string) error {
	httpClient := &http.Client{Timeout: 5 * time.Minute}

	resp, err := httpClient.Get(url) //nolint:gosec // URL comes from GitHub API, not user input
	if err != nil {
		return err
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned %d", resp.StatusCode)
	}

	out, err := os.Create(dest)
	if err != nil {
		return err
	}

	defer func() { _ = out.Close() }()

	_, err = io.Copy(out, resp.Body)

	return err
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}

	defer func() { _ = f.Close() }()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

func fetchExpectedChecksum(release *githubRelease, assetName string) (string, error) {
	var checksumAsset *githubAsset

	for i := range release.Assets {
		name := strings.ToLower(release.Assets[i].Name)
		if strings.Contains(name, "checksum") || strings.Contains(name, "sha256") {
			checksumAsset = &release.Assets[i]
			break
		}
	}

	if checksumAsset == nil {
		return "", fmt.Errorf("no checksums asset found")
	}

	httpClient := &http.Client{Timeout: 15 * time.Second}

	resp, err := httpClient.Get(checksumAsset.BrowserDownloadURL) //nolint:gosec // URL from GitHub API
	if err != nil {
		return "", err
	}

	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return "", err
	}

	for line := range strings.SplitSeq(string(body), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[1] == assetName {
			return fields[0], nil
		}
	}

	return "", fmt.Errorf("no checksum found for %s", assetName)
}
