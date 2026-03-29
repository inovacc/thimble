package ghauth

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTokenFromEnv(t *testing.T) {
	tests := []struct {
		name    string
		envVar  string
		envVal  string
		want    string
	}{
		{"GH_TOKEN", "GH_TOKEN", "gho_test123", "gho_test123"},
		{"GITHUB_TOKEN", "GITHUB_TOKEN", "ghp_test456", "ghp_test456"},
		{"GITHUB_PERSONAL_ACCESS_TOKEN", "GITHUB_PERSONAL_ACCESS_TOKEN", "ghp_test789", "ghp_test789"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear all token env vars.
			for _, env := range []string{"GH_TOKEN", "GITHUB_TOKEN", "GITHUB_PERSONAL_ACCESS_TOKEN"} {
				t.Setenv(env, "")
			}

			t.Setenv(tt.envVar, tt.envVal)

			got := Token()
			if got != tt.want {
				t.Errorf("Token() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTokenPriority(t *testing.T) {
	t.Setenv("GH_TOKEN", "first")
	t.Setenv("GITHUB_TOKEN", "second")
	t.Setenv("GITHUB_PERSONAL_ACCESS_TOKEN", "third")

	got := Token()
	if got != "first" {
		t.Errorf("Token() = %q, want %q (GH_TOKEN should have priority)", got, "first")
	}
}

func TestTokenFromGhConfig(t *testing.T) {
	dir := t.TempDir()
	hostsFile := filepath.Join(dir, "hosts.yml")
	content := "github.com:\n    oauth_token: gho_from_config\n    user: testuser\n"

	if err := os.WriteFile(hostsFile, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	// Clear env vars so config file is used.
	for _, env := range []string{"GH_TOKEN", "GITHUB_TOKEN", "GITHUB_PERSONAL_ACCESS_TOKEN"} {
		t.Setenv(env, "")
	}

	t.Setenv("GH_CONFIG_DIR", dir)

	got := Token()
	if got != "gho_from_config" {
		t.Errorf("Token() = %q, want %q", got, "gho_from_config")
	}
}

func TestTokenAllEmpty(t *testing.T) {
	// Clear all token env vars and point GH_CONFIG_DIR to an empty dir.
	for _, env := range []string{"GH_TOKEN", "GITHUB_TOKEN", "GITHUB_PERSONAL_ACCESS_TOKEN"} {
		t.Setenv(env, "")
	}

	t.Setenv("GH_CONFIG_DIR", t.TempDir()) // no hosts.yml

	got := Token()
	if got != "" {
		t.Errorf("Token() = %q, want empty string when nothing is set", got)
	}
}

func TestReadGhConfigNoOauthToken(t *testing.T) {
	dir := t.TempDir()
	// hosts.yml exists but contains no oauth_token line.
	content := "github.com:\n    user: testuser\n"

	if err := os.WriteFile(filepath.Join(dir, "hosts.yml"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	for _, env := range []string{"GH_TOKEN", "GITHUB_TOKEN", "GITHUB_PERSONAL_ACCESS_TOKEN"} {
		t.Setenv(env, "")
	}

	t.Setenv("GH_CONFIG_DIR", dir)

	got := Token()
	if got != "" {
		t.Errorf("Token() = %q, want empty string when hosts.yml has no oauth_token", got)
	}
}

func TestTokenPersonalAccessTokenFallback(t *testing.T) {
	// GH_TOKEN and GITHUB_TOKEN unset; GITHUB_PERSONAL_ACCESS_TOKEN is the only source.
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GITHUB_PERSONAL_ACCESS_TOKEN", "ghp_personal")
	t.Setenv("GH_CONFIG_DIR", t.TempDir()) // no hosts.yml

	got := Token()
	if got != "ghp_personal" {
		t.Errorf("Token() = %q, want %q", got, "ghp_personal")
	}
}

func TestHostDefault(t *testing.T) {
	t.Setenv("GH_HOST", "")

	got := Host()
	if got != "github.com" {
		t.Errorf("Host() = %q, want %q", got, "github.com")
	}
}

func TestHostFromEnv(t *testing.T) {
	t.Setenv("GH_HOST", "github.example.com")

	got := Host()
	if got != "github.example.com" {
		t.Errorf("Host() = %q, want %q", got, "github.example.com")
	}
}
