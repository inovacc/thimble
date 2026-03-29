package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/inovacc/thimble/internal/platform"
)

// ── Root command tests ──

func TestRootCmd_LongDescription(t *testing.T) {
	if rootCmd.Long == "" {
		t.Error("rootCmd.Long should not be empty")
	}

	if !strings.Contains(rootCmd.Long, "MCP") {
		t.Error("rootCmd.Long should mention MCP")
	}
}

func TestRootCmd_RunEIsSet(t *testing.T) {
	if rootCmd.RunE == nil {
		t.Error("rootCmd.RunE should be set (MCP bridge)")
	}
}

func TestRootCmd_AllExpectedSubcommands(t *testing.T) {
	want := map[string]bool{
		"doctor":         false,
		"hook":           false,
		"setup":          false,
		"report":         false,
		"upgrade":        false,
		"version":        false,
		"lint":           false,
		"hooklog":        false,
		"plugin":         false,
		"publish":        false,
		"publish-status": false,
		"release-notes":  false,
	}

	for _, c := range rootCmd.Commands() {
		if _, ok := want[c.Name()]; ok {
			want[c.Name()] = true
		}
	}

	for name, found := range want {
		if !found {
			t.Errorf("expected subcommand %q not found on rootCmd", name)
		}
	}
}

func TestRootCmd_HelpOutput(t *testing.T) {
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"--help"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("help should not error: %v", err)
	}
}

// ── Version command tests ──

func TestGetVersionInfo_AllFieldsPopulated(t *testing.T) {
	info := GetVersionInfo()

	fields := map[string]string{
		"Version":   info.Version,
		"GitHash":   info.GitHash,
		"BuildTime": info.BuildTime,
		"BuildHash": info.BuildHash,
		"GoVersion": info.GoVersion,
		"GoOS":      info.GoOS,
		"GoArch":    info.GoArch,
	}

	for name, val := range fields {
		if val == "" {
			t.Errorf("VersionInfo.%s should not be empty", name)
		}
	}
}

func TestGetVersionJSON_ValidJSONAllKeys(t *testing.T) {
	s := GetVersionJSON()

	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		t.Fatalf("GetVersionJSON is not valid JSON: %v", err)
	}

	requiredKeys := []string{"version", "git_hash", "build_time", "build_hash", "go_version", "goos", "goarch"}
	for _, key := range requiredKeys {
		if _, ok := m[key]; !ok {
			t.Errorf("GetVersionJSON missing key %q", key)
		}
	}
}

func TestVersionCmd_Registered(t *testing.T) {
	if versionCmd.Parent() == nil {
		t.Error("versionCmd should be registered on rootCmd")
	}
}

func TestVersionCmd_ShortDescription(t *testing.T) {
	if versionCmd.Short == "" {
		t.Error("versionCmd.Short should not be empty")
	}
}

func TestVersionCmd_LongDescription(t *testing.T) {
	if versionCmd.Long == "" {
		t.Error("versionCmd.Long should not be empty")
	}
}

// ── Doctor command tests ──

func TestDoctorCmd_Registered(t *testing.T) {
	if doctorCmd.Parent() == nil {
		t.Error("doctorCmd should be registered on rootCmd")
	}
}

func TestDoctorCmd_HasReportFlag(t *testing.T) {
	f := doctorCmd.Flags().Lookup("report")
	if f == nil {
		t.Fatal("doctor command missing --report flag")
	}

	if f.DefValue != "false" {
		t.Errorf("report flag default = %q, want %q", f.DefValue, "false")
	}
}

func TestCheckResult_StatusIcons(t *testing.T) {
	tests := []struct {
		status   string
		wantIcon string
	}{
		{"pass", "[OK]"},
		{"fail", "[FAIL]"},
		{"warn", "[WARN]"},
		{"unknown", "[??]"},
		{"", "[??]"},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			r := checkResult{name: "test", status: tt.status, message: "msg"}
			got := r.String()

			if !strings.Contains(got, tt.wantIcon) {
				t.Errorf("status %q: String() = %q, want icon %q", tt.status, got, tt.wantIcon)
			}
		})
	}
}

func TestCheckServer_Architecture(t *testing.T) {
	r := checkServer()
	if r.status != "pass" {
		t.Errorf("checkServer status = %q, want pass", r.status)
	}

	if !strings.Contains(r.message, "single-binary") {
		t.Errorf("checkServer message = %q, want to contain 'single-binary'", r.message)
	}
}

func TestCheckPlatform_HasConfidence(t *testing.T) {
	r := checkPlatform()
	if r.name != "Platform" {
		t.Errorf("name = %q, want Platform", r.name)
	}

	if !strings.Contains(r.message, "confidence") {
		t.Errorf("platform message = %q, should contain 'confidence'", r.message)
	}
}

func TestCheckResult_FormatsNameAndMessage(t *testing.T) {
	r := checkResult{name: "MyCheck", status: "pass", message: "all good"}
	s := r.String()

	if !strings.Contains(s, "MyCheck") {
		t.Error("String() should contain name")
	}

	if !strings.Contains(s, "all good") {
		t.Error("String() should contain message")
	}
}

// ── Hook command tests ──

func TestHookCmd_RequiresMinimumArgs(t *testing.T) {
	if hookCmd.Args == nil {
		t.Fatal("hookCmd.Args should be set")
	}

	if err := hookCmd.Args(hookCmd, nil); err == nil {
		t.Error("hookCmd.Args should reject 0 args")
	}

	if err := hookCmd.Args(hookCmd, []string{"one"}); err == nil {
		t.Error("hookCmd.Args should reject 1 arg")
	}

	if err := hookCmd.Args(hookCmd, []string{"one", "two"}); err != nil {
		t.Errorf("hookCmd.Args should accept 2 args, got: %v", err)
	}

	if err := hookCmd.Args(hookCmd, []string{"one", "two", "three"}); err != nil {
		t.Errorf("hookCmd.Args should accept 3+ args, got: %v", err)
	}
}

func TestHookCmd_OneArgViaExecute(t *testing.T) {
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"hook", "claude-code"})

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("hook with 1 arg should fail (needs platform + event)")
	}
}

func TestResolveEvent_AllValidEvents(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"pretooluse", "PreToolUse"},
		{"posttooluse", "PostToolUse"},
		{"precompact", "PreCompact"},
		{"sessionstart", "SessionStart"},
		{"userpromptsubmit", "UserPromptSubmit"},
		{"beforetool", "PreToolUse"},
		{"aftertool", "PostToolUse"},
		{"precompress", "PreCompact"},
		{"PreToolUse", "PreToolUse"},
		{"PostToolUse", "PostToolUse"},
		{"PreCompact", "PreCompact"},
		{"SessionStart", "SessionStart"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, ok := resolveEvent(tt.input)
			if !ok {
				t.Fatalf("resolveEvent(%q) returned not-ok", tt.input)
			}

			if got != tt.want {
				t.Errorf("resolveEvent(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestResolveEvent_InvalidEvents(t *testing.T) {
	invalids := []string{
		"",
		"nonexistent",
		"PRE_TOOL_USE",
		"pre-tool-use",
		"random123",
	}

	for _, input := range invalids {
		t.Run("invalid_"+input, func(t *testing.T) {
			_, ok := resolveEvent(input)
			if ok {
				t.Errorf("resolveEvent(%q) should return not-ok for invalid event", input)
			}
		})
	}
}

func TestResolveEvent_CaseInsensitiveViaLowercase(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"PRETOOLUSE", "PreToolUse"},
		{"Pretooluse", "PreToolUse"},
		{"SESSIONSTART", "SessionStart"},
		{"PostToolUse", "PostToolUse"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, ok := resolveEvent(tt.input)
			if !ok {
				t.Fatalf("resolveEvent(%q) returned not-ok", tt.input)
			}

			if got != tt.want {
				t.Errorf("resolveEvent(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestBuildHookPayload_WithToolCall(t *testing.T) {
	raw := []byte(`{"toolName":"Bash","command":"ls"}`)

	normalized := platform.NormalizedEvent{
		ToolName:   "Bash",
		ToolInput:  map[string]any{"command": "ls"},
		SessionID:  "sess-1",
		ProjectDir: "/tmp/proj",
	}

	result := buildHookPayload(normalized, raw)
	if result == nil {
		t.Fatal("buildHookPayload returned nil")
	}

	var m map[string]any
	if err := json.Unmarshal(result, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	tc, ok := m["toolCall"].(map[string]any)
	if !ok {
		t.Fatal("expected toolCall in payload")
	}

	if tc["toolName"] != "Bash" {
		t.Errorf("toolName = %v, want Bash", tc["toolName"])
	}
}

func TestBuildHookPayload_EmptyNormalized(t *testing.T) {
	result := buildHookPayload(platform.NormalizedEvent{}, []byte(`{}`))
	if result == nil {
		t.Fatal("buildHookPayload returned nil for empty payload")
	}

	var m map[string]any
	if err := json.Unmarshal(result, &m); err != nil {
		t.Fatal(err)
	}

	if _, ok := m["sessionId"]; !ok {
		t.Error("missing sessionId")
	}

	if _, ok := m["projectDir"]; !ok {
		t.Error("missing projectDir")
	}

	// ToolCall should not be present when ToolName is empty.
	if _, ok := m["toolCall"]; ok {
		t.Error("toolCall should not be present when ToolName is empty")
	}
}

func TestBuildHookPayload_InvalidRawJSON(t *testing.T) {
	result := buildHookPayload(platform.NormalizedEvent{SessionID: "s1"}, []byte("not json"))
	if result == nil {
		t.Fatal("buildHookPayload returned nil for invalid JSON")
	}

	var m map[string]any
	if err := json.Unmarshal(result, &m); err != nil {
		t.Fatal(err)
	}

	if _, ok := m["extra"]; ok {
		t.Error("extra should not be present for invalid JSON input")
	}
}

func TestBuildHookPayload_WithSource(t *testing.T) {
	normalized := platform.NormalizedEvent{
		SessionID: "s1",
		Source:    "startup",
	}

	result := buildHookPayload(normalized, []byte(`{}`))

	var m map[string]any
	if err := json.Unmarshal(result, &m); err != nil {
		t.Fatal(err)
	}

	if m["source"] != "startup" {
		t.Errorf("source = %v, want startup", m["source"])
	}
}

func TestBuildHookPayload_WithToolOutput(t *testing.T) {
	normalized := platform.NormalizedEvent{
		ToolName:   "Read",
		ToolInput:  map[string]any{"path": "/tmp/file"},
		ToolOutput: "file contents here",
		IsError:    true,
		SessionID:  "s1",
	}

	result := buildHookPayload(normalized, []byte(`{}`))

	var m map[string]any
	if err := json.Unmarshal(result, &m); err != nil {
		t.Fatal(err)
	}

	tc := m["toolCall"].(map[string]any)
	if tc["toolResponse"] != "file contents here" {
		t.Errorf("toolResponse = %v", tc["toolResponse"])
	}

	if tc["isError"] != true {
		t.Errorf("isError = %v, want true", tc["isError"])
	}
}

func TestHookDebugger_WriteOutput(t *testing.T) {
	tmp := t.TempDir()
	dbg := &hookDebugger{dir: tmp, ts: "20260328_100000.000"}

	dbg.writeOutput("PreToolUse", `{"decision":"allow"}`, 42*time.Millisecond)

	entries, err := os.ReadDir(tmp)
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected 1 file, got %d", len(entries))
	}

	if !strings.Contains(entries[0].Name(), "PreToolUse") {
		t.Errorf("filename %q should contain event name", entries[0].Name())
	}

	// Verify content is valid JSON.
	data, _ := os.ReadFile(filepath.Join(tmp, entries[0].Name()))
	if !json.Valid(data) {
		t.Error("output file should contain valid JSON")
	}
}

func TestHookDebugger_WriteError(t *testing.T) {
	tmp := t.TempDir()
	dbg := &hookDebugger{dir: tmp, ts: "20260328_100000.000"}

	dbg.writeError("resolve_platform", errForTest("unknown platform"))

	entries, err := os.ReadDir(tmp)
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected 1 file, got %d", len(entries))
	}

	if !strings.Contains(entries[0].Name(), "resolve_platform") {
		t.Errorf("filename %q should contain phase", entries[0].Name())
	}

	data, _ := os.ReadFile(filepath.Join(tmp, entries[0].Name()))

	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal("error file should contain valid JSON")
	}

	if m["phase"] != "resolve_platform" {
		t.Errorf("phase = %v", m["phase"])
	}

	if m["error"] != "unknown platform" {
		t.Errorf("error = %v", m["error"])
	}
}

// ── Lint command tests ──

func TestLintCmd_Registered(t *testing.T) {
	found := false

	for _, c := range rootCmd.Commands() {
		if c.Name() == "lint" {
			found = true
			break
		}
	}

	if !found {
		t.Error("lint command not registered on rootCmd")
	}
}

func TestLintCmd_Flags(t *testing.T) {
	flags := []struct {
		name     string
		defValue string
	}{
		{"fix", "false"},
		{"fast", "false"},
		{"timeout", "300"},
	}

	for _, tt := range flags {
		f := lintCmd.Flags().Lookup(tt.name)
		if f == nil {
			t.Errorf("lint command missing --%s flag", tt.name)
			continue
		}

		if f.DefValue != tt.defValue {
			t.Errorf("--%s default = %q, want %q", tt.name, f.DefValue, tt.defValue)
		}
	}
}

func TestLintCmd_EnableFlagIsSlice(t *testing.T) {
	f := lintCmd.Flags().Lookup("enable")
	if f == nil {
		t.Fatal("lint command missing --enable flag")
	}

	if f.DefValue != "[]" {
		t.Errorf("--enable default = %q, want %q", f.DefValue, "[]")
	}
}

// ── Hooklog command tests ──

func TestHooklogCmd_Registered(t *testing.T) {
	found := false

	for _, c := range rootCmd.Commands() {
		if c.Name() == "hooklog" {
			found = true
			break
		}
	}

	if !found {
		t.Error("hooklog command not registered on rootCmd")
	}
}

func TestHooklogCmd_Flags(t *testing.T) {
	flags := []struct {
		name     string
		defValue string
	}{
		{"limit", "50"},
		{"platform", ""},
		{"event", ""},
		{"tool", ""},
		{"blocked", "false"},
		{"clear", "false"},
		{"report", "false"},
		{"debug", "false"},
	}

	for _, tt := range flags {
		f := hooklogCmd.Flags().Lookup(tt.name)
		if f == nil {
			t.Errorf("hooklog command missing --%s flag", tt.name)
			continue
		}

		if f.DefValue != tt.defValue {
			t.Errorf("hooklog --%s default = %q, want %q", tt.name, f.DefValue, tt.defValue)
		}
	}
}

func TestHooklogCmd_LimitShorthand(t *testing.T) {
	f := hooklogCmd.Flags().ShorthandLookup("n")
	if f == nil {
		t.Fatal("hooklog command missing -n shorthand for --limit")
	}

	if f.Name != "limit" {
		t.Errorf("-n maps to %q, want %q", f.Name, "limit")
	}
}

// ── Plugin command tests ──

func TestPluginCmd_AllSubcommands(t *testing.T) {
	want := map[string]bool{
		"list":     false,
		"dir":      false,
		"install":  false,
		"remove":   false,
		"search":   false,
		"update":   false,
		"validate": false,
	}

	for _, c := range pluginCmd.Commands() {
		if _, ok := want[c.Name()]; ok {
			want[c.Name()] = true
		}
	}

	for name, found := range want {
		if !found {
			t.Errorf("plugin subcommand %q not found", name)
		}
	}
}

func TestPluginInstallCmd_RequiresExactlyOneArg(t *testing.T) {
	if err := pluginInstallCmd.Args(pluginInstallCmd, nil); err == nil {
		t.Error("install should reject 0 args")
	}

	if err := pluginInstallCmd.Args(pluginInstallCmd, []string{"one"}); err != nil {
		t.Errorf("install should accept 1 arg: %v", err)
	}

	if err := pluginInstallCmd.Args(pluginInstallCmd, []string{"one", "two"}); err == nil {
		t.Error("install should reject 2 args")
	}
}

func TestPluginRemoveCmd_RequiresExactlyOneArg(t *testing.T) {
	if err := pluginRemoveCmd.Args(pluginRemoveCmd, nil); err == nil {
		t.Error("remove should reject 0 args")
	}

	if err := pluginRemoveCmd.Args(pluginRemoveCmd, []string{"one"}); err != nil {
		t.Errorf("remove should accept 1 arg: %v", err)
	}
}

func TestPluginValidateCmd_RequiresExactlyOneArg(t *testing.T) {
	if err := pluginValidateCmd.Args(pluginValidateCmd, nil); err == nil {
		t.Error("validate should reject 0 args")
	}

	if err := pluginValidateCmd.Args(pluginValidateCmd, []string{"path.json"}); err != nil {
		t.Errorf("validate should accept 1 arg: %v", err)
	}
}

func TestPluginUpdateCmd_AcceptsOptionalArg(t *testing.T) {
	if err := pluginUpdateCmd.Args(pluginUpdateCmd, nil); err != nil {
		t.Errorf("update should accept 0 args: %v", err)
	}

	if err := pluginUpdateCmd.Args(pluginUpdateCmd, []string{"docker"}); err != nil {
		t.Errorf("update should accept 1 arg: %v", err)
	}

	if err := pluginUpdateCmd.Args(pluginUpdateCmd, []string{"a", "b"}); err == nil {
		t.Error("update should reject 2 args")
	}
}

func TestPluginInstallCmd_HasScopeFlag(t *testing.T) {
	f := pluginInstallCmd.Flags().Lookup("scope")
	if f == nil {
		t.Fatal("install missing --scope flag")
	}

	if f.DefValue != "user" {
		t.Errorf("scope default = %q, want user", f.DefValue)
	}

	if f.Shorthand != "s" {
		t.Errorf("scope shorthand = %q, want s", f.Shorthand)
	}
}

func TestPluginUpdateCmd_HasCheckFlag(t *testing.T) {
	f := pluginUpdateCmd.Flags().Lookup("check")
	if f == nil {
		t.Fatal("update missing --check flag")
	}

	if f.DefValue != "false" {
		t.Errorf("check default = %q, want false", f.DefValue)
	}
}

// ── Report command tests ──

func TestReportCmd_Subcommands(t *testing.T) {
	want := map[string]bool{
		"list":   false,
		"show":   false,
		"delete": false,
	}

	for _, c := range reportCmd.Commands() {
		if _, ok := want[c.Name()]; ok {
			want[c.Name()] = true
		}
	}

	for name, found := range want {
		if !found {
			t.Errorf("report subcommand %q not found", name)
		}
	}
}

func TestReportShowCmd_RequiresExactlyOneArg(t *testing.T) {
	if err := reportShowCmd.Args(reportShowCmd, nil); err == nil {
		t.Error("report show should reject 0 args")
	}

	if err := reportShowCmd.Args(reportShowCmd, []string{"id-123"}); err != nil {
		t.Errorf("report show should accept 1 arg: %v", err)
	}
}

func TestReportDeleteCmd_RequiresExactlyOneArg(t *testing.T) {
	if err := reportDeleteCmd.Args(reportDeleteCmd, nil); err == nil {
		t.Error("report delete should reject 0 args")
	}

	if err := reportDeleteCmd.Args(reportDeleteCmd, []string{"id-123"}); err != nil {
		t.Errorf("report delete should accept 1 arg: %v", err)
	}
}

// ── Publish command tests ──

func TestParseVersion_ExtendedCases(t *testing.T) {
	tests := []struct {
		tag                 string
		major, minor, patch int
	}{
		{"v0.0.1", 0, 0, 1},
		{"v1.0.0", 1, 0, 0},
		{"v99.88.77", 99, 88, 77},
		{"2.3.4", 2, 3, 4},
	}

	for _, tt := range tests {
		t.Run(tt.tag, func(t *testing.T) {
			major, minor, patch, err := parseVersion(tt.tag)
			if err != nil {
				t.Fatal(err)
			}

			if major != tt.major || minor != tt.minor || patch != tt.patch {
				t.Errorf("got %d.%d.%d, want %d.%d.%d", major, minor, patch, tt.major, tt.minor, tt.patch)
			}
		})
	}
}

func TestParseVersion_MoreErrors(t *testing.T) {
	for _, bad := range []string{"vX.Y.Z", "v1", "v1.2", "hello", "v.1.2.3"} {
		_, _, _, err := parseVersion(bad)
		if err == nil {
			t.Errorf("parseVersion(%q) should error", bad)
		}
	}
}

func TestPublishOpts_Fields(t *testing.T) {
	opts := publishOpts{
		version:   "v2.0.0",
		message:   "release",
		dryRun:    true,
		skipTests: true,
		watch:     false,
	}

	if opts.version != "v2.0.0" {
		t.Error("version mismatch")
	}

	if !opts.dryRun {
		t.Error("dryRun should be true")
	}

	if !opts.skipTests {
		t.Error("skipTests should be true")
	}

	if opts.watch {
		t.Error("watch should be false")
	}
}

// ── Selfheal tests ──

func TestCheckVersionMismatch_NoThimbleEntry(t *testing.T) {
	dir := t.TempDir()
	manifest := map[string]pluginManifest{
		"other-plugin": {Version: "v1.0.0", Path: "/path"},
	}

	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}

	_ = os.WriteFile(filepath.Join(dir, "installed_plugins.json"), data, 0o644)

	warning := CheckVersionMismatch(dir)
	if warning != "" {
		t.Errorf("expected empty for no thimble entry, got %q", warning)
	}
}

func TestCheckVersionMismatch_EmptyVersion(t *testing.T) {
	dir := t.TempDir()
	manifest := map[string]pluginManifest{
		"thimble": {Version: "", Path: "/path"},
	}

	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}

	_ = os.WriteFile(filepath.Join(dir, "installed_plugins.json"), data, 0o644)

	warning := CheckVersionMismatch(dir)
	if warning != "" {
		t.Errorf("expected empty for empty version in manifest, got %q", warning)
	}
}

// ── prettyJSON tests ──

func TestPrettyJSON_ValidJSON(t *testing.T) {
	input := `{"key":"value","num":42}`
	got := prettyJSON(input)

	if !strings.Contains(got, "key") {
		t.Error("prettyJSON should contain the key")
	}

	if !strings.Contains(got, "\n") {
		t.Error("prettyJSON should produce multi-line output for objects")
	}
}

func TestPrettyJSON_InvalidJSON(t *testing.T) {
	input := "not valid json"
	got := prettyJSON(input)

	if got != input {
		t.Errorf("prettyJSON should return input as-is for invalid JSON, got %q", got)
	}
}

func TestPrettyJSON_EmptyString(t *testing.T) {
	got := prettyJSON("")
	if got != "" {
		t.Errorf("prettyJSON of empty string should return empty, got %q", got)
	}
}

func TestPrettyJSON_Array(t *testing.T) {
	input := `[1,2,3]`
	got := prettyJSON(input)

	if !strings.Contains(got, "1") {
		t.Error("prettyJSON of array should contain elements")
	}
}

// ── Setup command client aliases test ──

func TestClientAliases_Coverage(t *testing.T) {
	expectedAliases := []string{
		"claude", "gemini", "vscode", "copilot", "cursor", "opencode", "codex",
		"claude-code", "gemini-cli", "vscode-copilot",
	}

	for _, alias := range expectedAliases {
		if _, ok := clientAliases[alias]; !ok {
			t.Errorf("clientAliases missing %q", alias)
		}
	}
}

func TestClientAliases_ResolveToValidPlatformID(t *testing.T) {
	for alias, pid := range clientAliases {
		if pid == "" {
			t.Errorf("alias %q resolves to empty PlatformID", alias)
		}
	}
}

// ── conventionalRe regex tests ──

func TestConventionalRe_MatchesStandardCommits(t *testing.T) {
	tests := []struct {
		input    string
		wantType string
		wantDesc string
	}{
		{"feat: add login", "feat", "add login"},
		{"fix: resolve crash", "fix", "resolve crash"},
		{"chore(deps): update", "chore", "update"},
		{"refactor(core): simplify", "refactor", "simplify"},
		{"docs: update readme", "docs", "update readme"},
		{"ci: fix pipeline", "ci", "fix pipeline"},
		{"test: add coverage", "test", "add coverage"},
		{"perf: optimize query", "perf", "optimize query"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			m := conventionalRe.FindStringSubmatch(tt.input)
			if m == nil {
				t.Fatalf("no match for %q", tt.input)
			}

			if m[1] != tt.wantType {
				t.Errorf("type = %q, want %q", m[1], tt.wantType)
			}

			if m[2] != tt.wantDesc {
				t.Errorf("desc = %q, want %q", m[2], tt.wantDesc)
			}
		})
	}
}

func TestConventionalRe_NoMatch(t *testing.T) {
	noMatch := []string{
		"Update README",
		"123: invalid",
		"FEAT: uppercase type",
		"- bullet point",
		"",
	}

	for _, input := range noMatch {
		t.Run("nomatch_"+input, func(t *testing.T) {
			if m := conventionalRe.FindStringSubmatch(input); m != nil {
				t.Errorf("expected no match for %q, got %v", input, m)
			}
		})
	}
}

// ── Integration: execute subcommands via rootCmd ──

func TestExecute_DoctorViaRoot(t *testing.T) {
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"doctor"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("doctor should not error: %v", err)
	}
}

func TestExecute_VersionViaRoot(t *testing.T) {
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"version"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("version should not error: %v", err)
	}
}

func TestExecute_UnknownCommand(t *testing.T) {
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"nonexistent-command"})

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("unknown command should error")
	}
}

func TestExecute_HookInvalidPlatform(t *testing.T) {
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"hook", "invalid-platform", "PreToolUse"})

	// Provide empty stdin.
	oldStdin := os.Stdin
	r, w, _ := os.Pipe()
	_, _ = w.WriteString(`{}`)
	_ = w.Close()
	os.Stdin = r

	defer func() { os.Stdin = oldStdin }()

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("hook with invalid platform should fail")
	}

	if !strings.Contains(err.Error(), "unknown platform") {
		t.Errorf("error = %q, want to contain 'unknown platform'", err.Error())
	}
}

func TestExecute_HookInvalidEvent(t *testing.T) {
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"hook", "claude-code", "InvalidEvent"})

	oldStdin := os.Stdin
	r, w, _ := os.Pipe()
	_, _ = w.WriteString(`{}`)
	_ = w.Close()
	os.Stdin = r

	defer func() { os.Stdin = oldStdin }()

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("hook with invalid event should fail")
	}

	if !strings.Contains(err.Error(), "unknown event") {
		t.Errorf("error = %q, want to contain 'unknown event'", err.Error())
	}
}

// ── Event map completeness ──

func TestEventMap_AllCanonicalEventsReachable(t *testing.T) {
	canonicals := map[string]bool{
		"PreToolUse":       false,
		"PostToolUse":      false,
		"PreCompact":       false,
		"SessionStart":     false,
		"UserPromptSubmit": false,
	}

	for _, canonical := range eventMap {
		canonicals[canonical] = true
	}

	for name, found := range canonicals {
		if !found {
			t.Errorf("canonical event %q is not reachable from any eventMap entry", name)
		}
	}
}

// ── Helpers ──

type stringError string

func (e stringError) Error() string { return string(e) }

func errForTest(msg string) error {
	return stringError(msg)
}
