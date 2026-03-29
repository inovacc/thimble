package security

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// ── TestMatchesAnyPattern ──

func TestMatchesAnyPattern(t *testing.T) {
	tests := []struct {
		name            string
		command         string
		patterns        []string
		caseInsensitive bool
		wantMatch       bool
	}{
		{
			name:      "empty patterns returns no match",
			command:   "ls -la",
			patterns:  nil,
			wantMatch: false,
		},
		{
			name:      "empty slice returns no match",
			command:   "ls -la",
			patterns:  []string{},
			wantMatch: false,
		},
		{
			name:      "non-bash patterns are skipped",
			command:   "ls -la",
			patterns:  []string{"Read(.env)", "Write(*.go)"},
			wantMatch: false,
		},
		{
			name:      "wildcard matches",
			command:   "sudo rm -rf /",
			patterns:  []string{"Bash(sudo *)"},
			wantMatch: true,
		},
		{
			name:            "case insensitive match",
			command:         "SUDO rm -rf /",
			patterns:        []string{"Bash(sudo *)"},
			caseInsensitive: true,
			wantMatch:       true,
		},
		{
			name:            "case sensitive no match",
			command:         "SUDO rm -rf /",
			patterns:        []string{"Bash(sudo *)"},
			caseInsensitive: false,
			wantMatch:       false,
		},
		{
			name:      "exact match without wildcard",
			command:   "ls",
			patterns:  []string{"Bash(ls)"},
			wantMatch: true,
		},
		{
			name:      "colon pattern match",
			command:   "git push origin main",
			patterns:  []string{"Bash(git push:*)"},
			wantMatch: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MatchesAnyPattern(tt.command, tt.patterns, tt.caseInsensitive)
			if (got != "") != tt.wantMatch {
				t.Errorf("MatchesAnyPattern(%q, %v, %v) = %q, wantMatch=%v",
					tt.command, tt.patterns, tt.caseInsensitive, got, tt.wantMatch)
			}
		})
	}
}

// ── TestExtractShellCommands_AllLanguages ──

func TestExtractShellCommands_AllLanguages(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		language string
		wantMin  int
		wantAny  string // at least one command should contain this substring
	}{
		{
			name:     "ruby system single quotes",
			code:     `system('rm -rf /tmp/test')`,
			language: "ruby",
			wantMin:  1,
			wantAny:  "rm -rf",
		},
		{
			name:     "ruby system double quotes",
			code:     `system("whoami")`,
			language: "ruby",
			wantMin:  1,
			wantAny:  "whoami",
		},
		{
			name:     "ruby backtick execution",
			code:     "`uname -a`",
			language: "ruby",
			wantMin:  1,
			wantAny:  "uname",
		},
		{
			name:     "go exec.Command",
			code:     `exec.Command("ls", "-la")`,
			language: "go",
			wantMin:  1,
			wantAny:  "ls",
		},
		{
			name:     "php shell_exec",
			code:     `shell_exec('cat /etc/passwd')`,
			language: "php",
			wantMin:  1,
			wantAny:  "cat",
		},
		{
			name:     "php exec",
			code:     `exec('whoami')`,
			language: "php",
			wantMin:  1,
			wantAny:  "whoami",
		},
		{
			name:     "php system",
			code:     `system("id")`,
			language: "php",
			wantMin:  1,
			wantAny:  "id",
		},
		{
			name:     "php passthru",
			code:     `passthru('ls -la')`,
			language: "php",
			wantMin:  1,
			wantAny:  "ls",
		},
		{
			name:     "php proc_open",
			code:     `proc_open("bash -c 'echo hi'")`,
			language: "php",
			wantMin:  1,
			wantAny:  "bash",
		},
		{
			name:     "rust Command::new",
			code:     `Command::new("curl")`,
			language: "rust",
			wantMin:  1,
			wantAny:  "curl",
		},
		{
			name:     "typescript execSync",
			code:     `execSync('npm install')`,
			language: "typescript",
			wantMin:  1,
			wantAny:  "npm",
		},
		{
			name:     "typescript spawn",
			code:     `spawn("node")`,
			language: "typescript",
			wantMin:  1,
			wantAny:  "node",
		},
		{
			name:     "typescript template literal exec",
			code:     "execSync(`rm -rf /tmp`)",
			language: "typescript",
			wantMin:  1,
			wantAny:  "rm",
		},
		{
			name:     "unknown language returns nil",
			code:     `print("hello")`,
			language: "haskell",
			wantMin:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmds := ExtractShellCommands(tt.code, tt.language)
			if len(cmds) < tt.wantMin {
				t.Errorf("ExtractShellCommands(%q, %q) = %v (%d cmds), want >= %d",
					tt.code, tt.language, cmds, len(cmds), tt.wantMin)
			}

			if tt.wantAny != "" && len(cmds) > 0 {
				found := false

				for _, c := range cmds {
					if contains(c, tt.wantAny) {
						found = true
						break
					}
				}

				if !found {
					t.Errorf("expected at least one command containing %q, got %v", tt.wantAny, cmds)
				}
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}

	return false
}

// ── TestSplitChainedCommands_EdgeCases ──

func TestSplitChainedCommands_EdgeCases(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "escaped double quote inside double quotes",
			input: `echo "he said \"hello\"" && ls`,
			want:  []string{`echo "he said \"hello\""`, "ls"},
		},
		{
			name:  "single quotes inside double quotes",
			input: `echo "it's fine" && whoami`,
			want:  []string{`echo "it's fine"`, "whoami"},
		},
		{
			name:  "double quotes inside single quotes",
			input: `echo '"hello"' && id`,
			want:  []string{`echo '"hello"'`, "id"},
		},
		{
			name:  "backtick in command",
			input: "echo `date` && ls",
			want:  []string{"echo `date`", "ls"},
		},
		{
			name:  "empty input",
			input: "",
			want:  nil,
		},
		{
			name:  "only whitespace",
			input: "   ",
			want:  nil,
		},
		{
			name:  "semicolons with extra whitespace",
			input: "  a  ;  b  ;  c  ",
			want:  []string{"a", "b", "c"},
		},
		{
			name:  "pipe operator",
			input: "cat file | grep pattern | wc -l",
			want:  []string{"cat file", "grep pattern", "wc -l"},
		},
		{
			name:  "unbalanced single quote treats rest as quoted",
			input: "echo 'hello && world",
			want:  []string{"echo 'hello && world"},
		},
		{
			name:  "unbalanced double quote treats rest as quoted",
			input: `echo "hello && world`,
			want:  []string{`echo "hello && world`},
		},
		{
			name:  "mixed operators",
			input: "a && b || c; d | e",
			want:  []string{"a", "b", "c", "d", "e"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SplitChainedCommands(tt.input)
			if len(got) != len(tt.want) {
				t.Errorf("SplitChainedCommands(%q) = %v (len %d), want %v (len %d)",
					tt.input, got, len(got), tt.want, len(tt.want))

				return
			}

			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("SplitChainedCommands(%q)[%d] = %q, want %q",
						tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
}

// ── TestReadSingleSettings ──

func TestReadSingleSettings(t *testing.T) {
	t.Run("missing file returns nil", func(t *testing.T) {
		got := readSingleSettings("/nonexistent/path/settings.json")
		if got != nil {
			t.Errorf("expected nil for missing file, got %+v", got)
		}
	})

	t.Run("invalid JSON returns nil", func(t *testing.T) {
		dir := t.TempDir()

		path := filepath.Join(dir, "bad.json")
		if err := os.WriteFile(path, []byte("{invalid json!}"), 0644); err != nil {
			t.Fatal(err)
		}

		got := readSingleSettings(path)
		if got != nil {
			t.Errorf("expected nil for invalid JSON, got %+v", got)
		}
	})

	t.Run("valid JSON with bash patterns", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "settings.json")
		s := settingsFile{}
		s.Permissions.Allow = []string{"Bash(ls:*)", "Read(.env)"}
		s.Permissions.Deny = []string{"Bash(sudo *)", "Write(*.key)"}
		s.Permissions.Ask = []string{"Bash(git push:*)"}

		data, _ := json.Marshal(s)
		if err := os.WriteFile(path, data, 0644); err != nil {
			t.Fatal(err)
		}

		got := readSingleSettings(path)
		if got == nil {
			t.Fatal("expected non-nil result")
		}
		// Only bash patterns should be kept.
		if len(got.Allow) != 1 || got.Allow[0] != "Bash(ls:*)" {
			t.Errorf("Allow = %v, want [Bash(ls:*)]", got.Allow)
		}

		if len(got.Deny) != 1 || got.Deny[0] != "Bash(sudo *)" {
			t.Errorf("Deny = %v, want [Bash(sudo *)]", got.Deny)
		}

		if len(got.Ask) != 1 || got.Ask[0] != "Bash(git push:*)" {
			t.Errorf("Ask = %v, want [Bash(git push:*)]", got.Ask)
		}
	})

	t.Run("empty permissions object", func(t *testing.T) {
		dir := t.TempDir()

		path := filepath.Join(dir, "empty.json")
		if err := os.WriteFile(path, []byte(`{"permissions":{}}`), 0644); err != nil {
			t.Fatal(err)
		}

		got := readSingleSettings(path)
		if got == nil {
			t.Fatal("expected non-nil for valid JSON")
		}

		if len(got.Allow) != 0 || len(got.Deny) != 0 || len(got.Ask) != 0 {
			t.Errorf("expected empty policy, got %+v", got)
		}
	})
}

// ── TestReadBashPolicies ──

func TestReadBashPolicies(t *testing.T) {
	t.Run("no files returns empty", func(t *testing.T) {
		policies := ReadBashPolicies("/nonexistent/dir", "/nonexistent/global.json")
		if len(policies) != 0 {
			t.Errorf("expected 0 policies, got %d", len(policies))
		}
	})

	t.Run("project local and global files", func(t *testing.T) {
		projectDir := t.TempDir()

		claudeDir := filepath.Join(projectDir, ".claude")
		if err := os.MkdirAll(claudeDir, 0755); err != nil {
			t.Fatal(err)
		}

		// Project-local settings.
		writeSettingsFile(t, filepath.Join(claudeDir, "settings.local.json"), settingsFile{
			Permissions: struct {
				Allow []string `json:"allow"`
				Deny  []string `json:"deny"`
				Ask   []string `json:"ask"`
			}{
				Allow: []string{"Bash(echo *)"},
			},
		})

		// Project-shared settings.
		writeSettingsFile(t, filepath.Join(claudeDir, "settings.json"), settingsFile{
			Permissions: struct {
				Allow []string `json:"allow"`
				Deny  []string `json:"deny"`
				Ask   []string `json:"ask"`
			}{
				Deny: []string{"Bash(rm -rf *)"},
			},
		})

		// Global settings.
		globalDir := t.TempDir()
		globalPath := filepath.Join(globalDir, "settings.json")
		writeSettingsFile(t, globalPath, settingsFile{
			Permissions: struct {
				Allow []string `json:"allow"`
				Deny  []string `json:"deny"`
				Ask   []string `json:"ask"`
			}{
				Ask: []string{"Bash(git push:*)"},
			},
		})

		policies := ReadBashPolicies(projectDir, globalPath)
		if len(policies) != 3 {
			t.Fatalf("expected 3 policies, got %d", len(policies))
		}
		// Precedence: local, shared, global.
		if len(policies[0].Allow) != 1 {
			t.Errorf("policy[0] (local) Allow = %v, want 1 entry", policies[0].Allow)
		}

		if len(policies[1].Deny) != 1 {
			t.Errorf("policy[1] (shared) Deny = %v, want 1 entry", policies[1].Deny)
		}

		if len(policies[2].Ask) != 1 {
			t.Errorf("policy[2] (global) Ask = %v, want 1 entry", policies[2].Ask)
		}
	})

	t.Run("empty project dir uses only global", func(t *testing.T) {
		globalDir := t.TempDir()
		globalPath := filepath.Join(globalDir, "settings.json")
		writeSettingsFile(t, globalPath, settingsFile{
			Permissions: struct {
				Allow []string `json:"allow"`
				Deny  []string `json:"deny"`
				Ask   []string `json:"ask"`
			}{
				Deny: []string{"Bash(sudo *)"},
			},
		})

		policies := ReadBashPolicies("", globalPath)
		if len(policies) != 1 {
			t.Fatalf("expected 1 policy, got %d", len(policies))
		}
	})
}

// ── TestReadToolDenyPatterns ──

func TestReadToolDenyPatterns(t *testing.T) {
	projectDir := t.TempDir()

	claudeDir := filepath.Join(projectDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatal(err)
	}

	writeSettingsFile(t, filepath.Join(claudeDir, "settings.local.json"), settingsFile{
		Permissions: struct {
			Allow []string `json:"allow"`
			Deny  []string `json:"deny"`
			Ask   []string `json:"ask"`
		}{
			Deny: []string{"Read(.env)", "Read(*.key)", "Write(*.log)", "Bash(sudo *)"},
		},
	})

	result := ReadToolDenyPatterns("Read", projectDir, "/nonexistent/global.json")
	if len(result) != 1 {
		t.Fatalf("expected 1 glob set, got %d", len(result))
	}

	if len(result[0]) != 2 {
		t.Errorf("expected 2 Read deny globs, got %v", result[0])
	}

	// Non-matching tool returns empty.
	result = ReadToolDenyPatterns("Execute", projectDir, "/nonexistent/global.json")
	if len(result) != 0 {
		t.Errorf("expected 0 glob sets for Execute, got %d", len(result))
	}
}

// ── TestEvaluateCommand_MultiPolicy ──

func TestEvaluateCommand_MultiPolicy(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		policies []SecurityPolicy
		want     PermissionDecision
	}{
		{
			name:    "deny in any policy trumps allow in another",
			command: "sudo rm -rf /",
			policies: []SecurityPolicy{
				{Allow: []string{"Bash(sudo *)"}}, // policy 1 allows sudo
				{Deny: []string{"Bash(sudo *)"}},  // policy 2 denies sudo
			},
			want: Deny,
		},
		{
			name:    "first policy ask beats second policy allow",
			command: "git push origin main",
			policies: []SecurityPolicy{
				{Ask: []string{"Bash(git push:*)"}},
				{Allow: []string{"Bash(git push:*)"}},
			},
			want: Ask,
		},
		{
			name:    "no matching policy defaults to ask",
			command: "some-unknown-cmd",
			policies: []SecurityPolicy{
				{Allow: []string{"Bash(ls:*)"}},
			},
			want: Ask,
		},
		{
			name:    "deny in chained segment detected across policies",
			command: "echo hello && sudo reboot",
			policies: []SecurityPolicy{
				{Allow: []string{"Bash(echo *)"}},
				{Deny: []string{"Bash(sudo *)"}},
			},
			want: Deny,
		},
		{
			name:     "empty policies defaults to ask",
			command:  "ls",
			policies: nil,
			want:     Ask,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := EvaluateCommand(tt.command, tt.policies)
			if d.Decision != tt.want {
				t.Errorf("EvaluateCommand(%q) = %q, want %q", tt.command, d.Decision, tt.want)
			}
		})
	}
}

// ── TestGlobToRegex_EdgeCases ──

func TestGlobToRegex_EdgeCases(t *testing.T) {
	tests := []struct {
		name            string
		glob            string
		input           string
		caseInsensitive bool
		matches         bool
	}{
		{
			name:    "colon with no args matches bare command",
			glob:    "tree:*",
			input:   "tree",
			matches: true,
		},
		{
			name:    "colon with args matches command with args",
			glob:    "tree:*",
			input:   "tree --help",
			matches: true,
		},
		{
			name:    "special regex chars in command are escaped",
			glob:    "git.status:*",
			input:   "gitXstatus",
			matches: false, // dot should be literal
		},
		{
			name:    "special regex chars in command literal match",
			glob:    "git.status:*",
			input:   "git.status",
			matches: true,
		},
		{
			name:    "plus sign escaped",
			glob:    "c++:*",
			input:   "c++",
			matches: true,
		},
		{
			name:    "parentheses in glob args",
			glob:    "echo (foo):*",
			input:   "echo (foo)",
			matches: true,
		},
		{
			name:            "case insensitive colon pattern",
			glob:            "GIT:*",
			input:           "git status",
			caseInsensitive: true,
			matches:         true,
		},
		{
			name:    "wildcard only",
			glob:    "*",
			input:   "anything goes here",
			matches: true,
		},
		{
			name:    "space format with special chars",
			glob:    "npm run *",
			input:   "npm run build:prod",
			matches: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			re := GlobToRegex(tt.glob, tt.caseInsensitive)

			got := re.MatchString(tt.input)
			if got != tt.matches {
				t.Errorf("GlobToRegex(%q, %v).Match(%q) = %v, want %v (regex: %s)",
					tt.glob, tt.caseInsensitive, tt.input, got, tt.matches, re.String())
			}
		})
	}
}

// ── TestFileGlobToRegex_Paths ──

func TestFileGlobToRegex_Paths(t *testing.T) {
	tests := []struct {
		name            string
		glob            string
		path            string
		caseInsensitive bool
		matches         bool
	}{
		{
			name:    "forward slash path",
			glob:    "src/**/*.go",
			path:    "src/internal/main.go",
			matches: true,
		},
		{
			name:    "double star at start",
			glob:    "**/.env",
			path:    ".env",
			matches: true,
		},
		{
			name:    "double star in middle",
			glob:    "src/**/test/*.go",
			path:    "src/pkg/test/foo.go",
			matches: true,
		},
		{
			name:    "question mark matches single char",
			glob:    "?.go",
			path:    "a.go",
			matches: true,
		},
		{
			name:    "question mark does not match slash",
			glob:    "?.go",
			path:    "/.go",
			matches: false,
		},
		{
			name:    "single star does not cross directories",
			glob:    "*.go",
			path:    "src/main.go",
			matches: false,
		},
		{
			name:            "case insensitive file glob",
			glob:            "**/*.GO",
			path:            "src/main.go",
			caseInsensitive: true,
			matches:         true,
		},
		{
			name:    "dot in filename is literal",
			glob:    "*.test.js",
			path:    "foo.test.js",
			matches: true,
		},
		{
			name:    "dot in filename does not match arbitrary char",
			glob:    "*.test.js",
			path:    "fooXtestXjs",
			matches: false,
		},
	}

	// On Windows, also test that backslash paths work via EvaluateFilePath's normalization.
	if runtime.GOOS == "windows" {
		tests = append(tests, struct {
			name            string
			glob            string
			path            string
			caseInsensitive bool
			matches         bool
		}{
			name:            "Windows backslash normalized in EvaluateFilePath",
			glob:            "**/*.key",
			path:            `C:\Users\test\secret.key`,
			caseInsensitive: true,
			matches:         true,
		})
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			re := FileGlobToRegex(tt.glob, tt.caseInsensitive)

			got := re.MatchString(tt.path)
			if got != tt.matches {
				t.Errorf("FileGlobToRegex(%q, %v).Match(%q) = %v, want %v (regex: %s)",
					tt.glob, tt.caseInsensitive, tt.path, got, tt.matches, re.String())
			}
		})
	}
}

// ── TestEscapeRegex ──

func TestEscapeRegex(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello", "hello"},
		{"a.b", `a\.b`},
		{"a+b", `a\+b`},
		{"a*b", `a\*b`},
		{"a?b", `a\?b`},
		{"a^b", `a\^b`},
		{"a$b", `a\$b`},
		{"a(b)", `a\(b\)`},
		{"a[b]", `a\[b\]`},
		{"a{b}", `a\{b\}`},
		{"a|b", `a\|b`},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := escapeRegex(tt.input)
			if got != tt.want {
				t.Errorf("escapeRegex(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ── TestConvertGlobPart ──

func TestConvertGlobPart(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello", "hello"},
		{"*", ".*"},
		{"a.b", `a\.b`},
		{"*.go", `.*\.go`},
		{"a+b*", `a\+b.*`},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := convertGlobPart(tt.input)
			if got != tt.want {
				t.Errorf("convertGlobPart(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ── helpers ──

func writeSettingsFile(t *testing.T, path string, s settingsFile) {
	t.Helper()

	data, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
}
