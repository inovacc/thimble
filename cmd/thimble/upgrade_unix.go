//go:build !windows

package main

import "os"

// binaryName returns the expected binary name inside the archive.
func binaryName() string {
	return "thimble"
}

// archiveExt returns the archive extension for Unix.
func archiveExt() string {
	return ".tar.gz"
}

// postInstallChmod sets executable permissions on Unix.
func postInstallChmod(path string) {
	_ = os.Chmod(path, 0o755)
}
