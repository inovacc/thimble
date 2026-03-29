package session

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/inovacc/thimble/internal/model"
)

// maxTruncate is the default maximum data length for extracted events.
const maxTruncate = 300

func truncate(value string, maxLen int) string {
	if maxLen <= 0 {
		maxLen = maxTruncate
	}

	if len(value) <= maxLen {
		return value
	}

	return value[:maxLen]
}

func truncateAny(value any, maxLen int) string {
	if value == nil {
		return ""
	}

	switch v := value.(type) {
	case string:
		return truncate(v, maxLen)
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return ""
		}

		return truncate(string(b), maxLen)
	}
}

func getString(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}

	return ""
}

// ExtractEvents extracts session events from a PostToolUse hook input.
// Returns zero or more SessionEvents. Never panics.
func ExtractEvents(input model.HookInput) []model.SessionEvent {
	defer func() { recover() }() //nolint:errcheck

	if input.ToolCall == nil {
		return nil
	}

	var events []model.SessionEvent //nolint:prealloc // size unknown until extractors run

	events = append(events, extractFileAndRule(input)...)
	events = append(events, extractCwd(input)...)
	events = append(events, extractError(input)...)
	events = append(events, extractGit(input)...)
	events = append(events, extractEnv(input)...)
	events = append(events, extractTask(input)...)
	events = append(events, extractPlan(input)...)
	events = append(events, extractSkill(input)...)
	events = append(events, extractSubagent(input)...)
	events = append(events, extractMcp(input)...)
	events = append(events, extractDecision(input)...)
	events = append(events, extractWorktree(input)...)

	return events
}

// ExtractUserEvents extracts session events from a user message.
// Handles: decision, role, intent, data categories.
func ExtractUserEvents(message string) []model.SessionEvent {
	defer func() { recover() }() //nolint:errcheck

	var events []model.SessionEvent //nolint:prealloc // size unknown until extractors run

	events = append(events, extractUserDecision(message)...)
	events = append(events, extractRole(message)...)
	events = append(events, extractIntent(message)...)
	events = append(events, extractData(message)...)

	return events
}

// ── Category extractors ──

var ruleFileRe = regexp.MustCompile(`(?i)CLAUDE\.md$|\.claude[/\\]`)

func extractFileAndRule(input model.HookInput) []model.SessionEvent {
	tc := input.ToolCall
	toolInput := tc.ToolInput

	var events []model.SessionEvent

	switch tc.ToolName {
	case "Read":
		filePath := getString(toolInput, "file_path")
		if ruleFileRe.MatchString(filePath) {
			events = append(events, model.SessionEvent{
				Type: "rule", Category: "rule",
				Data: truncate(filePath, maxTruncate), Priority: 1,
			})
			if tc.ToolResponse != "" {
				events = append(events, model.SessionEvent{
					Type: "rule_content", Category: "rule",
					Data: truncate(tc.ToolResponse, 5000), Priority: 1,
				})
			}
		}

		events = append(events, model.SessionEvent{
			Type: "file_read", Category: "file",
			Data: truncate(filePath, maxTruncate), Priority: 1,
		})

	case "Edit":
		events = append(events, model.SessionEvent{
			Type: "file_edit", Category: "file",
			Data: truncate(getString(toolInput, "file_path"), maxTruncate), Priority: 1,
		})

	case "NotebookEdit":
		events = append(events, model.SessionEvent{
			Type: "file_edit", Category: "file",
			Data: truncate(getString(toolInput, "notebook_path"), maxTruncate), Priority: 1,
		})

	case "Write":
		events = append(events, model.SessionEvent{
			Type: "file_write", Category: "file",
			Data: truncate(getString(toolInput, "file_path"), maxTruncate), Priority: 1,
		})

	case "Glob":
		events = append(events, model.SessionEvent{
			Type: "file_glob", Category: "file",
			Data: truncate(getString(toolInput, "pattern"), maxTruncate), Priority: 3,
		})

	case "Grep":
		pattern := getString(toolInput, "pattern")
		path := getString(toolInput, "path")
		events = append(events, model.SessionEvent{
			Type: "file_search", Category: "file",
			Data: truncate(fmt.Sprintf("%s in %s", pattern, path), maxTruncate), Priority: 3,
		})
	}

	return events
}

var cdRe = regexp.MustCompile(`\bcd\s+("([^"]+)"|'([^']+)'|(\S+))`)

func extractCwd(input model.HookInput) []model.SessionEvent {
	if input.ToolCall.ToolName != "Bash" {
		return nil
	}

	cmd := getString(input.ToolCall.ToolInput, "command")

	m := cdRe.FindStringSubmatch(cmd)
	if m == nil {
		return nil
	}

	dir := m[2]
	if dir == "" {
		dir = m[3]
	}

	if dir == "" {
		dir = m[4]
	}

	return []model.SessionEvent{{
		Type: "cwd", Category: "cwd",
		Data: truncate(dir, maxTruncate), Priority: 2,
	}}
}

var errorRe = regexp.MustCompile(`(?i)exit code [1-9]|error:|FAIL|failed`)

func extractError(input model.HookInput) []model.SessionEvent {
	tc := input.ToolCall
	response := tc.ToolResponse

	isBashError := tc.ToolName == "Bash" && errorRe.MatchString(response)
	if !isBashError && !tc.IsError {
		return nil
	}

	return []model.SessionEvent{{
		Type: "error_tool", Category: "error",
		Data: truncate(response, maxTruncate), Priority: 2,
	}}
}

var gitPatterns = []struct {
	re *regexp.Regexp
	op string
}{
	{regexp.MustCompile(`\bgit\s+checkout\b`), "branch"},
	{regexp.MustCompile(`\bgit\s+commit\b`), "commit"},
	{regexp.MustCompile(`\bgit\s+merge\s+\S+`), "merge"},
	{regexp.MustCompile(`\bgit\s+rebase\b`), "rebase"},
	{regexp.MustCompile(`\bgit\s+stash\b`), "stash"},
	{regexp.MustCompile(`\bgit\s+push\b`), "push"},
	{regexp.MustCompile(`\bgit\s+pull\b`), "pull"},
	{regexp.MustCompile(`\bgit\s+log\b`), "log"},
	{regexp.MustCompile(`\bgit\s+diff\b`), "diff"},
	{regexp.MustCompile(`\bgit\s+status\b`), "status"},
	{regexp.MustCompile(`\bgit\s+branch\b`), "branch"},
	{regexp.MustCompile(`\bgit\s+reset\b`), "reset"},
	{regexp.MustCompile(`\bgit\s+add\b`), "add"},
	{regexp.MustCompile(`\bgit\s+cherry-pick\b`), "cherry-pick"},
	{regexp.MustCompile(`\bgit\s+tag\b`), "tag"},
	{regexp.MustCompile(`\bgit\s+fetch\b`), "fetch"},
	{regexp.MustCompile(`\bgit\s+clone\b`), "clone"},
	{regexp.MustCompile(`\bgit\s+worktree\b`), "worktree"},
}

func extractGit(input model.HookInput) []model.SessionEvent {
	if input.ToolCall.ToolName != "Bash" {
		return nil
	}

	cmd := getString(input.ToolCall.ToolInput, "command")
	for _, gp := range gitPatterns {
		if gp.re.MatchString(cmd) {
			return []model.SessionEvent{{
				Type: "git", Category: "git",
				Data: truncate(gp.op, maxTruncate), Priority: 2,
			}}
		}
	}

	return nil
}

var taskTools = map[string]string{
	"TodoWrite":  "task",
	"TaskCreate": "task_create",
	"TaskUpdate": "task_update",
}

func extractTask(input model.HookInput) []model.SessionEvent {
	evType, ok := taskTools[input.ToolCall.ToolName]
	if !ok {
		return nil
	}

	return []model.SessionEvent{{
		Type: evType, Category: "task",
		Data: truncateAny(input.ToolCall.ToolInput, maxTruncate), Priority: 1,
	}}
}

var planPathRe = regexp.MustCompile(`[/\\]\.claude[/\\]plans[/\\]`)

func extractPlan(input model.HookInput) []model.SessionEvent {
	tc := input.ToolCall

	switch tc.ToolName {
	case "EnterPlanMode":
		return []model.SessionEvent{{
			Type: "plan_enter", Category: "plan",
			Data: "entered plan mode", Priority: 2,
		}}

	case "ExitPlanMode":
		var events []model.SessionEvent

		detail := "exited plan mode"
		if prompts, ok := tc.ToolInput["allowedPrompts"]; ok {
			detail = fmt.Sprintf("exited plan mode (allowed: %s)", truncateAny(prompts, 200))
		}

		events = append(events, model.SessionEvent{
			Type: "plan_exit", Category: "plan",
			Data: truncate(detail, maxTruncate), Priority: 2,
		})

		resp := strings.ToLower(tc.ToolResponse)
		if strings.Contains(resp, "approved") || strings.Contains(resp, "approve") {
			events = append(events, model.SessionEvent{
				Type: "plan_approved", Category: "plan",
				Data: "plan approved by user", Priority: 1,
			})
		} else if strings.Contains(resp, "rejected") || strings.Contains(resp, "decline") || strings.Contains(resp, "denied") {
			events = append(events, model.SessionEvent{
				Type: "plan_rejected", Category: "plan",
				Data: truncate("plan rejected: "+tc.ToolResponse, maxTruncate), Priority: 2,
			})
		}

		return events

	case "Write", "Edit":
		filePath := getString(tc.ToolInput, "file_path")
		if planPathRe.MatchString(filePath) {
			parts := strings.Split(strings.ReplaceAll(filePath, "\\", "/"), "/")
			name := parts[len(parts)-1]

			return []model.SessionEvent{{
				Type: "plan_file_write", Category: "plan",
				Data: truncate("plan file: "+name, maxTruncate), Priority: 2,
			}}
		}
	}

	return nil
}

var envPatterns = []*regexp.Regexp{
	regexp.MustCompile(`\bsource\s+\S*activate\b`),
	regexp.MustCompile(`\bexport\s+\w+=`),
	regexp.MustCompile(`\bnvm\s+use\b`),
	regexp.MustCompile(`\bpyenv\s+(shell|local|global)\b`),
	regexp.MustCompile(`\bconda\s+activate\b`),
	regexp.MustCompile(`\brbenv\s+(shell|local|global)\b`),
	regexp.MustCompile(`\bnpm\s+install\b`),
	regexp.MustCompile(`\bnpm\s+ci\b`),
	regexp.MustCompile(`\bpip\s+install\b`),
	regexp.MustCompile(`\bbun\s+install\b`),
	regexp.MustCompile(`\byarn\s+(add|install)\b`),
	regexp.MustCompile(`\bpnpm\s+(add|install)\b`),
	regexp.MustCompile(`\bcargo\s+(install|add)\b`),
	regexp.MustCompile(`\bgo\s+(install|get)\b`),
	regexp.MustCompile(`\brustup\b`),
	regexp.MustCompile(`\basdf\b`),
	regexp.MustCompile(`\bvolta\b`),
	regexp.MustCompile(`\bdeno\s+install\b`),
}

var exportSanitizeRe = regexp.MustCompile(`\bexport\s+(\w+)=\S*`)

func extractEnv(input model.HookInput) []model.SessionEvent {
	if input.ToolCall.ToolName != "Bash" {
		return nil
	}

	cmd := getString(input.ToolCall.ToolInput, "command")
	isEnv := false

	for _, p := range envPatterns {
		if p.MatchString(cmd) {
			isEnv = true
			break
		}
	}

	if !isEnv {
		return nil
	}

	sanitized := exportSanitizeRe.ReplaceAllString(cmd, "export $1=***")

	return []model.SessionEvent{{
		Type: "env", Category: "env",
		Data: truncate(sanitized, maxTruncate), Priority: 2,
	}}
}

func extractSkill(input model.HookInput) []model.SessionEvent {
	if input.ToolCall.ToolName != "Skill" {
		return nil
	}

	return []model.SessionEvent{{
		Type: "skill", Category: "skill",
		Data: truncate(getString(input.ToolCall.ToolInput, "skill"), maxTruncate), Priority: 3,
	}}
}

func extractSubagent(input model.HookInput) []model.SessionEvent {
	if input.ToolCall.ToolName != "Agent" {
		return nil
	}

	prompt := getString(input.ToolCall.ToolInput, "prompt")
	if prompt == "" {
		prompt = getString(input.ToolCall.ToolInput, "description")
	}

	prompt = truncate(prompt, 200)

	response := truncate(input.ToolCall.ToolResponse, maxTruncate)
	isCompleted := response != ""

	evType := "subagent_launched"
	data := "[launched] " + prompt
	priority := 3

	if isCompleted {
		evType = "subagent_completed"
		data = truncate("[completed] "+prompt+" → "+response, maxTruncate)
		priority = 2
	}

	return []model.SessionEvent{{
		Type: evType, Category: "subagent",
		Data: data, Priority: priority,
	}}
}

func extractMcp(input model.HookInput) []model.SessionEvent {
	name := input.ToolCall.ToolName
	if !strings.HasPrefix(name, "mcp__") {
		return nil
	}

	parts := strings.Split(name, "__")
	toolShort := parts[len(parts)-1]

	argStr := ""

	for _, v := range input.ToolCall.ToolInput {
		if s, ok := v.(string); ok {
			argStr = ": " + truncate(s, 100)
			break
		}
	}

	return []model.SessionEvent{{
		Type: "mcp", Category: "mcp",
		Data: truncate(toolShort+argStr, maxTruncate), Priority: 3,
	}}
}

func extractDecision(input model.HookInput) []model.SessionEvent {
	if input.ToolCall.ToolName != "AskUserQuestion" {
		return nil
	}

	questionText := ""

	if questions, ok := input.ToolCall.ToolInput["questions"]; ok {
		if arr, ok := questions.([]any); ok && len(arr) > 0 {
			if q, ok := arr[0].(map[string]any); ok {
				questionText = getString(q, "question")
			}
		}
	}

	answer := truncate(input.ToolCall.ToolResponse, 150)

	var summary string
	if questionText != "" {
		summary = fmt.Sprintf("Q: %s → A: %s", truncate(questionText, 120), answer)
	} else {
		summary = "answer: " + answer
	}

	return []model.SessionEvent{{
		Type: "decision_question", Category: "decision",
		Data: truncate(summary, maxTruncate), Priority: 2,
	}}
}

func extractWorktree(input model.HookInput) []model.SessionEvent {
	if input.ToolCall.ToolName != "EnterWorktree" {
		return nil
	}

	name := getString(input.ToolCall.ToolInput, "name")
	if name == "" {
		name = "unnamed"
	}

	return []model.SessionEvent{{
		Type: "worktree", Category: "env",
		Data: truncate("entered worktree: "+name, maxTruncate), Priority: 2,
	}}
}

// ── User-message extractors ──

var decisionPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b(don'?t|do not|never|always|instead|rather|prefer)\b`),
	regexp.MustCompile(`(?i)\b(use|switch to|go with|pick|choose)\s+\w+\s+(instead|over|not)\b`),
	regexp.MustCompile(`(?i)\b(no,?\s+(use|do|try|make))\b`),
}

func extractUserDecision(message string) []model.SessionEvent {
	for _, p := range decisionPatterns {
		if p.MatchString(message) {
			return []model.SessionEvent{{
				Type: "decision", Category: "decision",
				Data: truncate(message, maxTruncate), Priority: 2,
			}}
		}
	}

	return nil
}

var rolePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b(act as|you are|behave like|pretend|role of|persona)\b`),
	regexp.MustCompile(`(?i)\b(senior|staff|principal|lead)\s+(engineer|developer|architect)\b`),
}

func extractRole(message string) []model.SessionEvent {
	for _, p := range rolePatterns {
		if p.MatchString(message) {
			return []model.SessionEvent{{
				Type: "role", Category: "role",
				Data: truncate(message, maxTruncate), Priority: 3,
			}}
		}
	}

	return nil
}

var intentPatterns = []struct {
	mode string
	re   *regexp.Regexp
}{
	{"investigate", regexp.MustCompile(`(?i)\b(why|how does|explain|understand|what is|analyze|debug|look into)\b`)},
	{"implement", regexp.MustCompile(`(?i)\b(create|add|build|implement|write|make|develop|fix)\b`)},
	{"discuss", regexp.MustCompile(`(?i)\b(think about|consider|should we|what if|pros and cons|opinion)\b`)},
	{"review", regexp.MustCompile(`(?i)\b(review|check|audit|verify|test|validate)\b`)},
}

func extractIntent(message string) []model.SessionEvent {
	for _, ip := range intentPatterns {
		if ip.re.MatchString(message) {
			return []model.SessionEvent{{
				Type: "intent", Category: "intent",
				Data: truncate(ip.mode, maxTruncate), Priority: 4,
			}}
		}
	}

	return nil
}

func extractData(message string) []model.SessionEvent {
	if len(message) <= 1024 {
		return nil
	}

	return []model.SessionEvent{{
		Type: "data", Category: "data",
		Data: truncate(message, 200), Priority: 4,
	}}
}
