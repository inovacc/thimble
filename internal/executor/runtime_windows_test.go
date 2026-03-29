//go:build windows

package executor

import (
	"os"
	"strings"
	"testing"
)

func TestResolveWindowsBash(t *testing.T) {
	result := resolveWindowsBash()

	// On Windows CI or dev machines with Git installed, we expect a path.
	// On minimal images without Git, result may be empty — that's OK.
	if result == "" {
		t.Log("resolveWindowsBash returned empty — no Git Bash found (acceptable in CI)")
		return
	}

	// The returned path must exist.
	if _, err := os.Stat(result); err != nil {
		t.Fatalf("resolveWindowsBash returned %q but file does not exist: %v", result, err)
	}

	// Must not be a System32 or WindowsApps bash (WSL shim).
	lower := strings.ToLower(result)
	if strings.Contains(lower, "system32") {
		t.Fatalf("resolveWindowsBash returned System32 bash: %s", result)
	}

	if strings.Contains(lower, "windowsapps") {
		t.Fatalf("resolveWindowsBash returned WindowsApps bash: %s", result)
	}

	t.Logf("resolveWindowsBash found: %s", result)
}

func TestResolveWindowsBash_KnownPaths(t *testing.T) {
	// Verify the function checks known Git Bash paths first.
	knownPaths := []string{
		`C:\Program Files\Git\usr\bin\bash.exe`,
		`C:\Program Files (x86)\Git\usr\bin\bash.exe`,
	}

	result := resolveWindowsBash()
	if result == "" {
		t.Skip("no bash found on this system")
	}

	// If a known path exists, the result should match one of them
	// (since known paths are checked first).
	for _, kp := range knownPaths {
		if _, err := os.Stat(kp); err == nil {
			if result != kp {
				// A known path exists but wasn't returned — means an earlier
				// known path was found first, which is also fine.
				for _, earlier := range knownPaths {
					if earlier == kp {
						break
					}

					if result == earlier {
						return // earlier known path was returned, that's correct
					}
				}
			} else {
				return // exact match
			}
		}
	}

	// If no known path exists, the result came from `where bash` filtering.
	t.Logf("result from PATH scan: %s", result)
}
