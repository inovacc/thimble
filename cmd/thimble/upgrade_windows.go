//go:build windows

package main

// binaryName returns the expected binary name inside the archive.
func binaryName() string {
	return "thimble.exe"
}

// archiveExt returns the archive extension for Windows.
func archiveExt() string {
	return ".zip"
}

// postInstallChmod is a no-op on Windows.
func postInstallChmod(_ string) {}
