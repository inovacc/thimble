package plugin

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestDefaultSandbox(t *testing.T) {
	sb := DefaultSandbox()

	if sb == nil {
		t.Fatal("DefaultSandbox() returned nil")
	}

	if len(sb.AllowedCommands) == 0 {
		t.Error("expected non-empty AllowedCommands")
	}

	if sb.MaxTimeout != 30*time.Second {
		t.Errorf("expected 30s timeout, got %v", sb.MaxTimeout)
	}
}

func TestSandboxFromPlugin_NilSandbox(t *testing.T) {
	def := &PluginDef{Name: "test", Sandbox: nil}
	sb := SandboxFromPlugin(def)

	if sb == nil {
		t.Fatal("SandboxFromPlugin returned nil for nil config")
	}

	// Should be the default sandbox.
	if len(sb.AllowedCommands) != len(defaultAllowedCommands) {
		t.Errorf("expected default allowlist length %d, got %d",
			len(defaultAllowedCommands), len(sb.AllowedCommands))
	}
}

func TestSandboxFromPlugin_CustomConfig(t *testing.T) {
	def := &PluginDef{
		Name: "custom",
		Sandbox: &SandboxConfig{
			AllowCommands: []string{"docker *", "kubectl *"},
			DenyPaths:     []string{"*.env", "secrets/*"},
			MaxTimeoutSec: 60,
		},
	}

	sb := SandboxFromPlugin(def)

	if len(sb.AllowedCommands) != 2 {
		t.Errorf("expected 2 allowed commands, got %d", len(sb.AllowedCommands))
	}

	if len(sb.DeniedPaths) != 2 {
		t.Errorf("expected 2 denied paths, got %d", len(sb.DeniedPaths))
	}

	if sb.MaxTimeout != 60*time.Second {
		t.Errorf("expected 60s timeout, got %v", sb.MaxTimeout)
	}
}

func TestSandboxFromPlugin_EmptyAllowlist(t *testing.T) {
	def := &PluginDef{
		Name: "empty-allow",
		Sandbox: &SandboxConfig{
			AllowCommands: []string{},
		},
	}

	sb := SandboxFromPlugin(def)

	// Empty allowlist should fall back to defaults.
	if len(sb.AllowedCommands) != len(defaultAllowedCommands) {
		t.Errorf("expected default allowlist, got %d commands", len(sb.AllowedCommands))
	}
}

func TestValidateCommand_DefaultAllowed(t *testing.T) {
	sb := DefaultSandbox()

	tests := []struct {
		cmd     string
		allowed bool
	}{
		{"echo hello", true},
		{"cat file.txt", true},
		{"git status", true},
		{"go test ./...", true},
		{"python script.py", true},
		{"python3 script.py", true},
		{"node index.js", true},
		{"rm -rf /", false},
		{"curl https://evil.com", false},
		{"wget https://evil.com", false},
		{"sudo rm -rf /", false},
		{"bash -c 'evil'", false},
	}

	for _, tt := range tests {
		err := ValidateCommand(sb, tt.cmd)
		if tt.allowed && err != nil {
			t.Errorf("expected %q to be allowed, got error: %v", tt.cmd, err)
		}

		if !tt.allowed && err == nil {
			t.Errorf("expected %q to be denied, got nil", tt.cmd)
		}
	}
}

func TestValidateCommand_CustomAllowlist(t *testing.T) {
	sb := &PluginSandbox{
		AllowedCommands: []string{"docker *", "kubectl *"},
		MaxTimeout:      30 * time.Second,
	}

	tests := []struct {
		cmd     string
		allowed bool
	}{
		{"docker build .", true},
		{"kubectl get pods", true},
		{"git status", false},
		{"echo hello", false},
	}

	for _, tt := range tests {
		err := ValidateCommand(sb, tt.cmd)
		if tt.allowed && err != nil {
			t.Errorf("expected %q to be allowed, got error: %v", tt.cmd, err)
		}

		if !tt.allowed && err == nil {
			t.Errorf("expected %q to be denied, got nil", tt.cmd)
		}
	}
}

func TestValidateCommand_EmptyCommand(t *testing.T) {
	sb := DefaultSandbox()

	err := ValidateCommand(sb, "")
	if err == nil {
		t.Error("expected error for empty command")
	}
}

func TestValidateCommand_NilSandbox(t *testing.T) {
	// Nil sandbox should use the default.
	err := ValidateCommand(nil, "echo hello")
	if err != nil {
		t.Errorf("expected nil sandbox to allow echo, got: %v", err)
	}
}

func TestValidateCommand_WithEnvPrefix(t *testing.T) {
	sb := DefaultSandbox()

	// Commands with env var prefix should be validated after stripping the prefix.
	err := ValidateCommand(sb, "FOO=bar echo hello")
	if err != nil {
		t.Errorf("expected env-prefixed echo to be allowed, got: %v", err)
	}

	err = ValidateCommand(sb, "FOO=bar BAZ=qux git status")
	if err != nil {
		t.Errorf("expected env-prefixed git to be allowed, got: %v", err)
	}

	err = ValidateCommand(sb, "FOO=bar rm -rf /")
	if err == nil {
		t.Error("expected env-prefixed rm to be denied")
	}
}

func TestValidatePath(t *testing.T) {
	sb := &PluginSandbox{
		AllowedCommands: defaultAllowedCommands,
		DeniedPaths:     []string{".env", "secrets/*", "*.key"},
		MaxTimeout:      30 * time.Second,
	}

	tests := []struct {
		path    string
		allowed bool
	}{
		{"src/main.go", true},
		{".env", false},
		{"secrets/api.json", false},
		{"server.key", false},
		{"config/app.yaml", true},
	}

	for _, tt := range tests {
		err := ValidatePath(sb, tt.path)
		if tt.allowed && err != nil {
			t.Errorf("expected path %q to be allowed, got: %v", tt.path, err)
		}

		if !tt.allowed && err == nil {
			t.Errorf("expected path %q to be denied, got nil", tt.path)
		}
	}
}

func TestValidatePath_NilSandbox(t *testing.T) {
	err := ValidatePath(nil, "/etc/passwd")
	if err != nil {
		t.Errorf("nil sandbox should allow all paths, got: %v", err)
	}
}

func TestValidatePath_EmptyDenyList(t *testing.T) {
	sb := &PluginSandbox{DeniedPaths: nil}

	err := ValidatePath(sb, ".env")
	if err != nil {
		t.Errorf("empty deny list should allow all paths, got: %v", err)
	}
}

func TestStripEnvPrefix(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"echo hello", "echo hello"},
		{"FOO=bar echo hello", "echo hello"},
		{"FOO=bar BAZ=qux git status", "git status"},
		{"A=1 B=2 C=3 node app.js", "node app.js"},
		{"-flag value", "-flag value"},
		{"not=env=var something", "something"}, // still valid KEY=VALUE
	}

	for _, tt := range tests {
		got := stripEnvPrefix(tt.input)
		if got != tt.expected {
			t.Errorf("stripEnvPrefix(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestIsEnvVarName(t *testing.T) {
	tests := []struct {
		input string
		valid bool
	}{
		{"FOO", true},
		{"_BAR", true},
		{"foo123", true},
		{"123foo", false},
		{"", false},
		{"FOO-BAR", false},
	}

	for _, tt := range tests {
		got := isEnvVarName(tt.input)
		if got != tt.valid {
			t.Errorf("isEnvVarName(%q) = %v, want %v", tt.input, got, tt.valid)
		}
	}
}

func TestMatchCommandGlob(t *testing.T) {
	tests := []struct {
		pattern string
		cmd     string
		match   bool
	}{
		{"echo *", "echo hello", true},
		{"echo *", "echo", true},
		{"git *", "git status", true},
		{"git *", "git", true},
		{"git *", "gitx status", false},
		{"docker *", "docker build .", true},
	}

	for _, tt := range tests {
		got := matchCommandGlob(tt.pattern, tt.cmd)
		if got != tt.match {
			t.Errorf("matchCommandGlob(%q, %q) = %v, want %v", tt.pattern, tt.cmd, got, tt.match)
		}
	}
}

func TestValidateToolPermissions_NilPerms(t *testing.T) {
	sb := DefaultSandbox()

	// Nil permissions = restrictive defaults, so network commands should be denied.
	tests := []struct {
		cmd     string
		allowed bool
	}{
		{"echo hello", true},
		{"git status", true},
		{"curl https://example.com", false},
		{"wget https://example.com", false},
		{"echo hello > file.txt", false},
		{"echo hello >> file.txt", false},
		{"cat foo | grep bar", false},
		{"echo $(whoami)", false},
		{"echo `whoami`", false},
	}

	for _, tt := range tests {
		err := ValidateToolPermissions(sb, nil, tt.cmd)
		if tt.allowed && err != nil {
			t.Errorf("nil perms: expected %q to be allowed, got: %v", tt.cmd, err)
		}

		if !tt.allowed && err == nil {
			t.Errorf("nil perms: expected %q to be denied", tt.cmd)
		}
	}
}

func TestValidateToolPermissions_AllowNetwork(t *testing.T) {
	sb := DefaultSandbox()
	perms := &ToolPermissions{AllowNetwork: true}

	if err := ValidateToolPermissions(sb, perms, "curl https://example.com"); err != nil {
		t.Errorf("expected curl to be allowed with AllowNetwork=true, got: %v", err)
	}

	if err := ValidateToolPermissions(sb, perms, "wget https://example.com"); err != nil {
		t.Errorf("expected wget to be allowed with AllowNetwork=true, got: %v", err)
	}

	// Pipes should still be denied.
	if err := ValidateToolPermissions(sb, perms, "curl foo | cat"); err == nil {
		t.Error("expected pipe to be denied even with AllowNetwork=true")
	}
}

func TestValidateToolPermissions_AllowFileWrite(t *testing.T) {
	sb := DefaultSandbox()
	perms := &ToolPermissions{AllowFileWrite: true}

	if err := ValidateToolPermissions(sb, perms, "echo hello > file.txt"); err != nil {
		t.Errorf("expected redirect to be allowed with AllowFileWrite=true, got: %v", err)
	}

	if err := ValidateToolPermissions(sb, perms, "echo hello >> file.txt"); err != nil {
		t.Errorf("expected append to be allowed with AllowFileWrite=true, got: %v", err)
	}

	// Network still denied.
	if err := ValidateToolPermissions(sb, perms, "curl https://example.com"); err == nil {
		t.Error("expected curl to be denied with only AllowFileWrite")
	}
}

func TestValidateToolPermissions_AllowSubprocess(t *testing.T) {
	sb := DefaultSandbox()
	perms := &ToolPermissions{AllowSubprocess: true}

	if err := ValidateToolPermissions(sb, perms, "cat foo | grep bar"); err != nil {
		t.Errorf("expected pipe to be allowed with AllowSubprocess=true, got: %v", err)
	}

	if err := ValidateToolPermissions(sb, perms, "echo $(whoami)"); err != nil {
		t.Errorf("expected $() to be allowed with AllowSubprocess=true, got: %v", err)
	}

	if err := ValidateToolPermissions(sb, perms, "echo `whoami`"); err != nil {
		t.Errorf("expected backtick to be allowed with AllowSubprocess=true, got: %v", err)
	}

	// Redirect still denied.
	if err := ValidateToolPermissions(sb, perms, "echo hello > file.txt"); err == nil {
		t.Error("expected redirect to be denied with only AllowSubprocess")
	}
}

func TestValidateToolPermissions_AllPermsEnabled(t *testing.T) {
	sb := DefaultSandbox()
	perms := &ToolPermissions{
		AllowNetwork:    true,
		AllowFileWrite:  true,
		AllowSubprocess: true,
	}

	cmds := []string{
		"curl https://example.com",
		"wget https://example.com",
		"echo hello > file.txt",
		"echo hello >> file.txt",
		"cat foo | grep bar",
		"echo $(whoami)",
		"echo `whoami`",
	}

	for _, cmd := range cmds {
		if err := ValidateToolPermissions(sb, perms, cmd); err != nil {
			t.Errorf("all perms enabled: expected %q to be allowed, got: %v", cmd, err)
		}
	}
}

func TestValidateToolPermissions_EmptyCommand(t *testing.T) {
	sb := DefaultSandbox()

	if err := ValidateToolPermissions(sb, nil, ""); err != nil {
		t.Errorf("expected empty command to pass (caught elsewhere), got: %v", err)
	}

	if err := ValidateToolPermissions(sb, nil, "   "); err != nil {
		t.Errorf("expected whitespace command to pass, got: %v", err)
	}
}

func TestValidateToolPermissions_EnvPrefixStripping(t *testing.T) {
	sb := DefaultSandbox()

	// curl behind env prefix should still be caught.
	if err := ValidateToolPermissions(sb, nil, "FOO=bar curl https://evil.com"); err == nil {
		t.Error("expected env-prefixed curl to be denied")
	}
}

func TestEffectiveTimeout_PermsOverride(t *testing.T) {
	sb := &PluginSandbox{MaxTimeout: 30 * time.Second}

	// No perms: use sandbox default.
	if got := EffectiveTimeout(sb, nil); got != 30*time.Second {
		t.Errorf("expected 30s, got %v", got)
	}

	// Perms with MaxTimeout: use perms value.
	perms := &ToolPermissions{MaxTimeout: 120}
	if got := EffectiveTimeout(sb, perms); got != 120*time.Second {
		t.Errorf("expected 120s, got %v", got)
	}

	// Perms with zero MaxTimeout: fall back to sandbox.
	perms = &ToolPermissions{MaxTimeout: 0}
	if got := EffectiveTimeout(sb, perms); got != 30*time.Second {
		t.Errorf("expected 30s, got %v", got)
	}

	// Nil sandbox: use defaultMaxTimeout.
	if got := EffectiveTimeout(nil, nil); got != defaultMaxTimeout {
		t.Errorf("expected default %v, got %v", defaultMaxTimeout, got)
	}
}

func TestToolPermissions_JSONRoundtrip(t *testing.T) {
	td := ToolDef{
		Name:        "ctx_test_tool",
		Description: "test tool",
		Command:     "echo hello",
		InputSchema: map[string]InputFieldDef{},
		Permissions: &ToolPermissions{
			AllowNetwork:    true,
			AllowFileWrite:  false,
			AllowSubprocess: false,
			MaxTimeout:      60,
		},
	}

	data, err := json.Marshal(td)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded ToolDef
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.Permissions == nil {
		t.Fatal("expected permissions to survive round-trip")
	}

	if !decoded.Permissions.AllowNetwork {
		t.Error("expected AllowNetwork=true")
	}

	if decoded.Permissions.AllowFileWrite {
		t.Error("expected AllowFileWrite=false")
	}

	if decoded.Permissions.MaxTimeout != 60 {
		t.Errorf("expected MaxTimeout=60, got %d", decoded.Permissions.MaxTimeout)
	}
}

func TestToolPermissions_NilOmittedInJSON(t *testing.T) {
	td := ToolDef{
		Name:        "ctx_test_tool",
		Description: "test tool",
		Command:     "echo hello",
		InputSchema: map[string]InputFieldDef{},
	}

	data, err := json.Marshal(td)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// "permissions" key should not appear in JSON output.
	if strings.Contains(string(data), "permissions") {
		t.Errorf("nil permissions should be omitted from JSON, got: %s", string(data))
	}
}

func TestRedirectPattern(t *testing.T) {
	tests := []struct {
		cmd    string
		hasRdr bool
	}{
		{"echo hello", false},
		{"echo hello > file.txt", true},
		{"echo hello >> file.txt", true},
		{"git status", false},
		{"cat file.txt", false},
	}

	for _, tt := range tests {
		got := redirectPattern(tt.cmd)
		if got != tt.hasRdr {
			t.Errorf("redirectPattern(%q) = %v, want %v", tt.cmd, got, tt.hasRdr)
		}
	}
}
