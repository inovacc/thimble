package ghauth

import (
	"os"
	"path/filepath"
	"strings"
)

// Token returns the GitHub auth token by checking (in order):
// 1. GH_TOKEN or GITHUB_TOKEN env var
// 2. ~/.config/gh/hosts.yml (gh CLI auth config)
// 3. GITHUB_PERSONAL_ACCESS_TOKEN env var
func Token() string {
	if t := os.Getenv("GH_TOKEN"); t != "" {
		return t
	}

	if t := os.Getenv("GITHUB_TOKEN"); t != "" {
		return t
	}

	if t := os.Getenv("GITHUB_PERSONAL_ACCESS_TOKEN"); t != "" {
		return t
	}

	return readGhConfig()
}

// Host returns the GitHub host (default: github.com).
func Host() string {
	if h := os.Getenv("GH_HOST"); h != "" {
		return h
	}

	return "github.com"
}

func readGhConfig() string {
	configDir := os.Getenv("GH_CONFIG_DIR")
	if configDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}

		configDir = filepath.Join(home, ".config", "gh")
	}

	hostsFile := filepath.Join(configDir, "hosts.yml")

	data, err := os.ReadFile(hostsFile)
	if err != nil {
		return ""
	}

	// Simple YAML parsing — hosts.yml format:
	// github.com:
	//     oauth_token: gho_xxxx
	//     user: username
	// OR (newer format):
	// github.com:
	//     users:
	//         username:
	//             oauth_token: gho_xxxx
	lines := strings.SplitSeq(string(data), "\n")
	for line := range lines {
		trimmed := strings.TrimSpace(line)
		if after, ok := strings.CutPrefix(trimmed, "oauth_token:"); ok {
			token := after
			return strings.TrimSpace(token)
		}
	}

	return ""
}
