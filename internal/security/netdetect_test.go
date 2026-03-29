package security

import "testing"

func TestDetectNetworkCommandCurl(t *testing.T) {
	result := DetectNetworkCommand("curl https://example.com")
	if !result.IsCurlWget {
		t.Error("expected IsCurlWget=true for curl command")
	}

	if result.Advisory == "" {
		t.Error("expected non-empty advisory")
	}
}

func TestDetectNetworkCommandWget(t *testing.T) {
	result := DetectNetworkCommand("wget https://example.com/file.tar.gz")
	if !result.IsCurlWget {
		t.Error("expected IsCurlWget=true for wget command")
	}
}

func TestDetectNetworkCommandQuotedNotDetected(t *testing.T) {
	// curl inside quotes should not trigger detection.
	result := DetectNetworkCommand(`gh issue edit --body "text with curl example"`)
	if result.IsCurlWget {
		t.Error("expected IsCurlWget=false for curl in quotes")
	}
}

func TestDetectNetworkCommandInlineHTTP(t *testing.T) {
	tests := []struct {
		cmd  string
		want bool
	}{
		{"fetch('https://api.example.com')", true},
		{"requests.get('https://example.com')", true},
		{"http.get('https://example.com')", true},
		{"echo hello", false},
		{"ls -la", false},
	}
	for _, tt := range tests {
		result := DetectNetworkCommand(tt.cmd)
		if result.IsInlineHTTP != tt.want {
			t.Errorf("DetectNetworkCommand(%q).IsInlineHTTP = %v, want %v", tt.cmd, result.IsInlineHTTP, tt.want)
		}
	}
}

func TestDetectNetworkCommandSafeCommands(t *testing.T) {
	safe := []string{"echo hello", "ls -la", "git status", "go test ./..."}
	for _, cmd := range safe {
		result := DetectNetworkCommand(cmd)
		if result.IsCurlWget || result.IsInlineHTTP {
			t.Errorf("DetectNetworkCommand(%q) should not detect network usage", cmd)
		}
	}
}

func TestDetectBuildTool(t *testing.T) {
	tests := []struct {
		cmd  string
		want string
	}{
		{"gradle build", "gradle"},
		{"./gradlew test", "gradlew"},
		{"mvn clean install", "mvn"},
		{"./mvnw package", "mvnw"},
		{"go build ./...", ""},
		{"npm test", ""},
	}
	for _, tt := range tests {
		got := DetectBuildTool(tt.cmd)
		if got != tt.want {
			t.Errorf("DetectBuildTool(%q) = %q, want %q", tt.cmd, got, tt.want)
		}
	}
}

func TestDetectBuildToolQuoted(t *testing.T) {
	// gradle inside quotes should not trigger.
	got := DetectBuildTool(`echo "run gradle build"`)
	if got != "" {
		t.Errorf("DetectBuildTool with quoted gradle should return empty, got %q", got)
	}
}

func TestIsWebFetchBlocked(t *testing.T) {
	if !IsWebFetchBlocked("WebFetch") {
		t.Error("expected WebFetch to be blocked")
	}

	if IsWebFetchBlocked("Bash") {
		t.Error("expected Bash to not be blocked")
	}
}

func TestIsAgentOrTask(t *testing.T) {
	if !IsAgentOrTask("Agent") {
		t.Error("expected Agent to match")
	}

	if !IsAgentOrTask("Task") {
		t.Error("expected Task to match")
	}

	if IsAgentOrTask("Bash") {
		t.Error("expected Bash to not match")
	}

	if IsAgentOrTask("Read") {
		t.Error("expected Read to not match")
	}
}

func TestStripHeredocWithCurl(t *testing.T) {
	cmd := "cat <<EOF\ncurl https://example.com\nsome text with wget\nEOF\necho \"done\""

	result := DetectNetworkCommand(cmd)
	if result.IsCurlWget {
		t.Error("curl inside heredoc should be stripped and not detected")
	}
}

func TestStripHeredocWithHTTP(t *testing.T) {
	cmd := "cat <<'EOF'\nrequests.get(url)\nfetch(url)\nEOF\necho \"done\""

	result := DetectNetworkCommand(cmd)
	if result.IsInlineHTTP {
		t.Error("HTTP calls inside heredoc should be stripped and not detected")
	}
}

func TestStripHeredocNoFalsePositives(t *testing.T) {
	// Real curl outside heredoc should still be detected.
	cmd := "cat <<EOF\nsome text\nEOF\ncurl https://api.example.com"

	result := DetectNetworkCommand(cmd)
	if !result.IsCurlWget {
		t.Error("curl outside heredoc should still be detected")
	}
}

func TestInlineHTTPRequestsPost(t *testing.T) {
	result := DetectNetworkCommand("python3 -c requests.post( url )")
	if !result.IsInlineHTTP {
		t.Error("requests.post should be detected as inline HTTP")
	}
}

func TestInlineHTTPRequestsPut(t *testing.T) {
	result := DetectNetworkCommand("python3 -c requests.put( url )")
	if !result.IsInlineHTTP {
		t.Error("requests.put should be detected as inline HTTP")
	}
}

func TestInlineHTTPRequest(t *testing.T) {
	result := DetectNetworkCommand("node -e http.request( url )")
	if !result.IsInlineHTTP {
		t.Error("http.request should be detected as inline HTTP")
	}
}

func TestNormalizeToolNameGemini(t *testing.T) {
	tests := []struct{ input, want string }{
		{"run_shell_command", "Bash"},
		{"read_file", "Read"},
		{"grep_search", "Grep"},
		{"web_fetch", "WebFetch"},
	}
	for _, tt := range tests {
		if got := NormalizeToolName(tt.input); got != tt.want {
			t.Errorf("NormalizeToolName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestNormalizeToolNameOpenCode(t *testing.T) {
	tests := []struct{ input, want string }{
		{"bash", "Bash"},
		{"view", "Read"},
		{"grep", "Grep"},
		{"fetch", "WebFetch"},
		{"agent", "Agent"},
	}
	for _, tt := range tests {
		if got := NormalizeToolName(tt.input); got != tt.want {
			t.Errorf("NormalizeToolName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestNormalizeToolNameCodex(t *testing.T) {
	tests := []struct{ input, want string }{
		{"shell", "Bash"},
		{"shell_command", "Bash"},
		{"container.exec", "Bash"},
		{"grep_files", "Grep"},
	}
	for _, tt := range tests {
		if got := NormalizeToolName(tt.input); got != tt.want {
			t.Errorf("NormalizeToolName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestNormalizeToolNameKiro(t *testing.T) {
	tests := []struct{ input, want string }{
		{"fs_read", "Read"},
		{"fs_write", "Write"},
		{"execute_bash", "Bash"},
	}
	for _, tt := range tests {
		if got := NormalizeToolName(tt.input); got != tt.want {
			t.Errorf("NormalizeToolName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestNormalizeToolNameUnknownPassthrough(t *testing.T) {
	if got := NormalizeToolName("SomeRandomTool"); got != "SomeRandomTool" {
		t.Errorf("unknown tool should pass through, got %q", got)
	}
}

func TestNormalizeToolNameCaseInsensitive(t *testing.T) {
	if got := NormalizeToolName("Run_Shell_Command"); got != "Bash" {
		t.Errorf("NormalizeToolName(Run_Shell_Command) = %q, want Bash", got)
	}

	if got := NormalizeToolName("BASH"); got != "Bash" {
		t.Errorf("NormalizeToolName(BASH) = %q, want Bash", got)
	}
}
