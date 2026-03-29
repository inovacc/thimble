package session

import (
	"strings"
	"testing"

	"github.com/inovacc/thimble/internal/model"
)

func TestExtractFileRead(t *testing.T) {
	events := ExtractEvents(model.HookInput{
		ToolCall: &model.ToolCall{
			ToolName: "Read",
			ToolInput: map[string]any{
				"file_path": "/src/main.go",
			},
		},
	})
	if len(events) != 1 {
		t.Fatalf("events = %d, want 1", len(events))
	}

	if events[0].Type != "file_read" {
		t.Errorf("type = %q, want file_read", events[0].Type)
	}
}

func TestExtractRuleFile(t *testing.T) {
	events := ExtractEvents(model.HookInput{
		ToolCall: &model.ToolCall{
			ToolName: "Read",
			ToolInput: map[string]any{
				"file_path": "/project/CLAUDE.md",
			},
			ToolResponse: "# Rules\nDo this...",
		},
	})
	// Should emit rule + rule_content + file_read = 3 events.
	if len(events) != 3 {
		t.Fatalf("events = %d, want 3", len(events))
	}

	types := map[string]bool{}
	for _, e := range events {
		types[e.Type] = true
	}

	if !types["rule"] || !types["rule_content"] || !types["file_read"] {
		t.Errorf("missing expected types: %v", types)
	}
}

func TestExtractGitCommit(t *testing.T) {
	events := ExtractEvents(model.HookInput{
		ToolCall: &model.ToolCall{
			ToolName: "Bash",
			ToolInput: map[string]any{
				"command": "git commit -m 'fix bug'",
			},
		},
	})
	if len(events) != 1 {
		t.Fatalf("events = %d, want 1", len(events))
	}

	if events[0].Data != "commit" {
		t.Errorf("data = %q, want commit", events[0].Data)
	}
}

func TestExtractCwd(t *testing.T) {
	events := ExtractEvents(model.HookInput{
		ToolCall: &model.ToolCall{
			ToolName: "Bash",
			ToolInput: map[string]any{
				"command": `cd "/home/user/project"`,
			},
		},
	})
	found := false

	for _, e := range events {
		if e.Type == "cwd" {
			found = true

			if e.Data != "/home/user/project" {
				t.Errorf("cwd data = %q", e.Data)
			}
		}
	}

	if !found {
		t.Error("expected cwd event")
	}
}

func TestExtractError(t *testing.T) {
	events := ExtractEvents(model.HookInput{
		ToolCall: &model.ToolCall{
			ToolName:     "Bash",
			ToolInput:    map[string]any{"command": "go build ."},
			ToolResponse: "exit code 1: compilation failed",
		},
	})
	found := false

	for _, e := range events {
		if e.Type == "error_tool" {
			found = true
		}
	}

	if !found {
		t.Error("expected error_tool event")
	}
}

func TestExtractEnvSanitized(t *testing.T) {
	events := ExtractEvents(model.HookInput{
		ToolCall: &model.ToolCall{
			ToolName: "Bash",
			ToolInput: map[string]any{
				"command": "export API_KEY=secret123",
			},
		},
	})
	found := false

	for _, e := range events {
		if e.Type == "env" {
			found = true

			if strings.Contains(e.Data, "secret123") {
				t.Error("env data should not contain secret value")
			}

			if !strings.Contains(e.Data, "***") {
				t.Error("env data should contain sanitized value")
			}
		}
	}

	if !found {
		t.Error("expected env event")
	}
}

func TestExtractMcp(t *testing.T) {
	events := ExtractEvents(model.HookInput{
		ToolCall: &model.ToolCall{
			ToolName: "mcp__plugin__ctx_search",
			ToolInput: map[string]any{
				"query": "kubernetes deploy",
			},
		},
	})
	if len(events) != 1 {
		t.Fatalf("events = %d, want 1", len(events))
	}

	if events[0].Category != "mcp" {
		t.Errorf("category = %q, want mcp", events[0].Category)
	}
}

func TestExtractSubagent(t *testing.T) {
	// Launched.
	events := ExtractEvents(model.HookInput{
		ToolCall: &model.ToolCall{
			ToolName: "Agent",
			ToolInput: map[string]any{
				"prompt": "search for the config file",
			},
		},
	})
	if len(events) != 1 || events[0].Type != "subagent_launched" {
		t.Errorf("launched: %v", events)
	}

	// Completed.
	events = ExtractEvents(model.HookInput{
		ToolCall: &model.ToolCall{
			ToolName:     "Agent",
			ToolInput:    map[string]any{"prompt": "search"},
			ToolResponse: "Found config at /etc/app.conf",
		},
	})
	if len(events) != 1 || events[0].Type != "subagent_completed" {
		t.Errorf("completed: %v", events)
	}
}

func TestExtractUserDecision(t *testing.T) {
	events := ExtractUserEvents("don't use mocks in the tests")
	if len(events) == 0 {
		t.Fatal("expected decision event")
	}

	if events[0].Category != "decision" {
		t.Errorf("category = %q, want decision", events[0].Category)
	}
}

func TestExtractUserIntent(t *testing.T) {
	events := ExtractUserEvents("explain how the auth middleware works")
	found := false

	for _, e := range events {
		if e.Category == "intent" && e.Data == "investigate" {
			found = true
		}
	}

	if !found {
		t.Error("expected intent=investigate event")
	}
}

func TestExtractUserData(t *testing.T) {
	// Short message — no data event.
	events := ExtractUserEvents("hello")
	for _, e := range events {
		if e.Category == "data" {
			t.Error("unexpected data event for short message")
		}
	}

	// Long message — should emit data event.
	long := strings.Repeat("x", 2000)
	events = ExtractUserEvents(long)
	found := false

	for _, e := range events {
		if e.Category == "data" {
			found = true
		}
	}

	if !found {
		t.Error("expected data event for long message")
	}
}

func TestExtractPlanEnter(t *testing.T) {
	events := ExtractEvents(model.HookInput{
		ToolCall: &model.ToolCall{
			ToolName:  "EnterPlanMode",
			ToolInput: map[string]any{},
		},
	})
	if len(events) != 1 || events[0].Type != "plan_enter" {
		t.Errorf("plan enter: %v", events)
	}
}

func TestExtractNilToolCall(t *testing.T) {
	events := ExtractEvents(model.HookInput{})
	if len(events) != 0 {
		t.Errorf("nil tool call should return empty, got %d", len(events))
	}
}

func TestExtractFileEdit(t *testing.T) {
	events := ExtractEvents(model.HookInput{
		ToolCall: &model.ToolCall{
			ToolName:  "Edit",
			ToolInput: map[string]any{"file_path": "/src/handler.go"},
		},
	})
	if len(events) != 1 || events[0].Type != "file_edit" {
		t.Errorf("Edit: got %v", events)
	}
}

func TestExtractFileWrite(t *testing.T) {
	events := ExtractEvents(model.HookInput{
		ToolCall: &model.ToolCall{
			ToolName:  "Write",
			ToolInput: map[string]any{"file_path": "/src/new_file.go"},
		},
	})
	if len(events) != 1 || events[0].Type != "file_write" {
		t.Errorf("Write: got %v", events)
	}
}

func TestExtractFileGlob(t *testing.T) {
	events := ExtractEvents(model.HookInput{
		ToolCall: &model.ToolCall{
			ToolName:  "Glob",
			ToolInput: map[string]any{"pattern": "**/*.go"},
		},
	})
	if len(events) != 1 || events[0].Type != "file_glob" {
		t.Errorf("Glob: got %v", events)
	}
}

func TestExtractFileGrep(t *testing.T) {
	events := ExtractEvents(model.HookInput{
		ToolCall: &model.ToolCall{
			ToolName:  "Grep",
			ToolInput: map[string]any{"pattern": "TODO", "path": "/src"},
		},
	})
	if len(events) != 1 || events[0].Type != "file_search" {
		t.Errorf("Grep: got %v", events)
	}

	if !strings.Contains(events[0].Data, "TODO") {
		t.Errorf("expected pattern in data, got %q", events[0].Data)
	}
}

func TestExtractNotebookEdit(t *testing.T) {
	events := ExtractEvents(model.HookInput{
		ToolCall: &model.ToolCall{
			ToolName:  "NotebookEdit",
			ToolInput: map[string]any{"notebook_path": "/notebooks/analysis.ipynb"},
		},
	})
	if len(events) != 1 || events[0].Type != "file_edit" {
		t.Errorf("NotebookEdit: got %v", events)
	}
}

func TestExtractSkill(t *testing.T) {
	events := ExtractEvents(model.HookInput{
		ToolCall: &model.ToolCall{
			ToolName:  "Skill",
			ToolInput: map[string]any{"skill": "commit"},
		},
	})
	if len(events) != 1 || events[0].Type != "skill" {
		t.Errorf("Skill: got %v", events)
	}

	if events[0].Data != "commit" {
		t.Errorf("Skill data = %q, want commit", events[0].Data)
	}
}

func TestExtractTask(t *testing.T) {
	tests := []struct {
		tool   string
		evType string
	}{
		{"TodoWrite", "task"},
		{"TaskCreate", "task_create"},
		{"TaskUpdate", "task_update"},
	}

	for _, tt := range tests {
		t.Run(tt.tool, func(t *testing.T) {
			events := ExtractEvents(model.HookInput{
				ToolCall: &model.ToolCall{
					ToolName:  tt.tool,
					ToolInput: map[string]any{"content": "fix the bug"},
				},
			})
			if len(events) != 1 || events[0].Type != tt.evType {
				t.Errorf("%s: got %v, want type %q", tt.tool, events, tt.evType)
			}
		})
	}
}

func TestExtractPlanExitApproved(t *testing.T) {
	events := ExtractEvents(model.HookInput{
		ToolCall: &model.ToolCall{
			ToolName:     "ExitPlanMode",
			ToolInput:    map[string]any{"allowedPrompts": []any{"build", "test"}},
			ToolResponse: "Plan approved by user",
		},
	})

	types := map[string]bool{}
	for _, e := range events {
		types[e.Type] = true
	}

	if !types["plan_exit"] {
		t.Error("missing plan_exit event")
	}

	if !types["plan_approved"] {
		t.Error("missing plan_approved event")
	}
}

func TestExtractPlanExitRejected(t *testing.T) {
	events := ExtractEvents(model.HookInput{
		ToolCall: &model.ToolCall{
			ToolName:     "ExitPlanMode",
			ToolInput:    map[string]any{},
			ToolResponse: "Plan rejected by user",
		},
	})

	types := map[string]bool{}
	for _, e := range events {
		types[e.Type] = true
	}

	if !types["plan_exit"] {
		t.Error("missing plan_exit event")
	}

	if !types["plan_rejected"] {
		t.Error("missing plan_rejected event")
	}
}

func TestExtractPlanFileWrite(t *testing.T) {
	events := ExtractEvents(model.HookInput{
		ToolCall: &model.ToolCall{
			ToolName:  "Write",
			ToolInput: map[string]any{"file_path": "/project/.claude/plans/migration.md"},
		},
	})

	types := map[string]bool{}
	for _, e := range events {
		types[e.Type] = true
	}

	if !types["plan_file_write"] {
		t.Error("missing plan_file_write event")
	}
}

func TestExtractWorktree(t *testing.T) {
	events := ExtractEvents(model.HookInput{
		ToolCall: &model.ToolCall{
			ToolName:  "EnterWorktree",
			ToolInput: map[string]any{"name": "feature-branch"},
		},
	})
	if len(events) != 1 || events[0].Type != "worktree" {
		t.Errorf("Worktree: got %v", events)
	}

	if !strings.Contains(events[0].Data, "feature-branch") {
		t.Errorf("expected worktree name in data, got %q", events[0].Data)
	}
}

func TestExtractWorktreeUnnamed(t *testing.T) {
	events := ExtractEvents(model.HookInput{
		ToolCall: &model.ToolCall{
			ToolName:  "EnterWorktree",
			ToolInput: map[string]any{},
		},
	})
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	if !strings.Contains(events[0].Data, "unnamed") {
		t.Errorf("expected 'unnamed' in data, got %q", events[0].Data)
	}
}

func TestExtractDecisionQuestion(t *testing.T) {
	events := ExtractEvents(model.HookInput{
		ToolCall: &model.ToolCall{
			ToolName: "AskUserQuestion",
			ToolInput: map[string]any{
				"questions": []any{
					map[string]any{"question": "Should I use Redis or Memcached?"},
				},
			},
			ToolResponse: "Use Redis",
		},
	})
	if len(events) != 1 || events[0].Type != "decision_question" {
		t.Errorf("Decision: got %v", events)
	}

	if !strings.Contains(events[0].Data, "Redis") {
		t.Errorf("expected question content in data, got %q", events[0].Data)
	}
}

func TestExtractGitOperations(t *testing.T) {
	tests := []struct {
		cmd  string
		want string
	}{
		{"git checkout main", "branch"},
		{"git merge feature", "merge"},
		{"git rebase main", "rebase"},
		{"git stash", "stash"},
		{"git push origin main", "push"},
		{"git pull origin main", "pull"},
		{"git log --oneline", "log"},
		{"git diff HEAD~1", "diff"},
		{"git status", "status"},
		{"git branch -a", "branch"},
		{"git reset --hard HEAD", "reset"},
		{"git add .", "add"},
		{"git cherry-pick abc123", "cherry-pick"},
		{"git tag v1.0.0", "tag"},
		{"git fetch origin", "fetch"},
		{"git clone https://github.com/repo", "clone"},
		{"git worktree add ../branch", "worktree"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			events := ExtractEvents(model.HookInput{
				ToolCall: &model.ToolCall{
					ToolName:  "Bash",
					ToolInput: map[string]any{"command": tt.cmd},
				},
			})
			if len(events) != 1 || events[0].Data != tt.want {
				t.Errorf("cmd=%q: got %v, want data=%q", tt.cmd, events, tt.want)
			}
		})
	}
}

func TestExtractEnvPatterns(t *testing.T) {
	commands := []string{
		"source venv/bin/activate",
		"nvm use 18",
		"pyenv shell 3.11",
		"conda activate myenv",
		"npm install express",
		"pip install requests",
		"cargo install ripgrep",
		"go install golang.org/x/tools@latest",
		"rustup default stable",
	}

	for _, cmd := range commands {
		t.Run(cmd, func(t *testing.T) {
			events := ExtractEvents(model.HookInput{
				ToolCall: &model.ToolCall{
					ToolName:  "Bash",
					ToolInput: map[string]any{"command": cmd},
				},
			})
			found := false

			for _, e := range events {
				if e.Type == "env" {
					found = true
				}
			}

			if !found {
				t.Errorf("expected env event for %q", cmd)
			}
		})
	}
}

func TestExtractErrorFromIsError(t *testing.T) {
	events := ExtractEvents(model.HookInput{
		ToolCall: &model.ToolCall{
			ToolName:     "Read",
			ToolInput:    map[string]any{"file_path": "/nonexistent"},
			ToolResponse: "file not found",
			IsError:      true,
		},
	})

	found := false

	for _, e := range events {
		if e.Type == "error_tool" {
			found = true
		}
	}

	if !found {
		t.Error("expected error_tool event from IsError=true")
	}
}

func TestExtractUserRole(t *testing.T) {
	tests := []string{
		"act as a senior engineer",
		"you are a code reviewer",
		"behave like a principal architect",
	}

	for _, msg := range tests {
		t.Run(msg, func(t *testing.T) {
			events := ExtractUserEvents(msg)
			found := false

			for _, e := range events {
				if e.Category == "role" {
					found = true
				}
			}

			if !found {
				t.Errorf("expected role event for %q", msg)
			}
		})
	}
}

func TestExtractUserIntentModes(t *testing.T) {
	tests := []struct {
		msg  string
		mode string
	}{
		{"why does the test fail?", "investigate"},
		{"create a new endpoint for users", "implement"},
		{"what if we used a different approach?", "discuss"},
		{"review the authentication code", "review"},
	}

	for _, tt := range tests {
		t.Run(tt.mode, func(t *testing.T) {
			events := ExtractUserEvents(tt.msg)
			found := false

			for _, e := range events {
				if e.Category == "intent" && e.Data == tt.mode {
					found = true
				}
			}

			if !found {
				t.Errorf("expected intent=%q for %q", tt.mode, tt.msg)
			}
		})
	}
}

func TestExtractCwdSingleQuotes(t *testing.T) {
	events := ExtractEvents(model.HookInput{
		ToolCall: &model.ToolCall{
			ToolName:  "Bash",
			ToolInput: map[string]any{"command": "cd '/path/with spaces'"},
		},
	})

	found := false

	for _, e := range events {
		if e.Type == "cwd" && e.Data == "/path/with spaces" {
			found = true
		}
	}

	if !found {
		t.Error("expected cwd event with single-quoted path")
	}
}

func TestExtractCwdUnquoted(t *testing.T) {
	events := ExtractEvents(model.HookInput{
		ToolCall: &model.ToolCall{
			ToolName:  "Bash",
			ToolInput: map[string]any{"command": "cd /home/user"},
		},
	})

	found := false

	for _, e := range events {
		if e.Type == "cwd" && e.Data == "/home/user" {
			found = true
		}
	}

	if !found {
		t.Error("expected cwd event with unquoted path")
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("hello", 10); got != "hello" {
		t.Errorf("truncate short: got %q", got)
	}

	if got := truncate("hello world", 5); got != "hello" {
		t.Errorf("truncate long: got %q", got)
	}

	if got := truncate("test", 0); got != "test" {
		t.Errorf("truncate zero max should use default: got %q", got)
	}
}

func TestTruncateAny(t *testing.T) {
	if got := truncateAny(nil, 10); got != "" {
		t.Errorf("truncateAny nil: got %q", got)
	}

	if got := truncateAny("hello", 3); got != "hel" {
		t.Errorf("truncateAny string: got %q", got)
	}

	if got := truncateAny(42, 10); got != "42" {
		t.Errorf("truncateAny int: got %q", got)
	}
}

func TestGetString(t *testing.T) {
	m := map[string]any{"key": "value", "num": 42}

	if got := getString(m, "key"); got != "value" {
		t.Errorf("getString existing: got %q", got)
	}

	if got := getString(m, "missing"); got != "" {
		t.Errorf("getString missing: got %q", got)
	}

	if got := getString(m, "num"); got != "" {
		t.Errorf("getString non-string: got %q", got)
	}
}

func TestExtractMcpNoArgs(t *testing.T) {
	events := ExtractEvents(model.HookInput{
		ToolCall: &model.ToolCall{
			ToolName:  "mcp__server__tool_name",
			ToolInput: map[string]any{"count": 5},
		},
	})
	if len(events) != 1 {
		t.Fatalf("events = %d, want 1", len(events))
	}

	if !strings.Contains(events[0].Data, "tool_name") {
		t.Errorf("expected tool name in data, got %q", events[0].Data)
	}
}

func TestExtractSubagentDescription(t *testing.T) {
	events := ExtractEvents(model.HookInput{
		ToolCall: &model.ToolCall{
			ToolName:  "Agent",
			ToolInput: map[string]any{"description": "find config"},
		},
	})
	if len(events) != 1 || events[0].Type != "subagent_launched" {
		t.Errorf("subagent via description: got %v", events)
	}

	if !strings.Contains(events[0].Data, "find config") {
		t.Errorf("expected description in data, got %q", events[0].Data)
	}
}

func TestExtractNoGitForNonBash(t *testing.T) {
	events := ExtractEvents(model.HookInput{
		ToolCall: &model.ToolCall{
			ToolName:  "Read",
			ToolInput: map[string]any{"file_path": "git commit -m 'test'"},
		},
	})

	for _, e := range events {
		if e.Category == "git" {
			t.Error("should not extract git events from non-Bash tool")
		}
	}
}

func TestExtractNoCwdForNonBash(t *testing.T) {
	events := ExtractEvents(model.HookInput{
		ToolCall: &model.ToolCall{
			ToolName:  "Write",
			ToolInput: map[string]any{"command": "cd /tmp"},
		},
	})

	for _, e := range events {
		if e.Type == "cwd" {
			t.Error("should not extract cwd from non-Bash tool")
		}
	}
}
