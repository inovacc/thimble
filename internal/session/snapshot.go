package session

import (
	"encoding/json"
	"fmt"
	"html"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/inovacc/thimble/internal/model"
)

// Budget constants for snapshot assembly.
const (
	DefaultMaxBytes = 2048
	MaxActiveFiles  = 10
)

// SnapshotBudget returns the snapshot budget from THIMBLE_SNAPSHOT_BUDGET or DefaultMaxBytes.
func SnapshotBudget() int {
	if v := os.Getenv("THIMBLE_SNAPSHOT_BUDGET"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}

	return DefaultMaxBytes
}

// BuildSnapshotOpts configures snapshot generation.
type BuildSnapshotOpts struct {
	MaxBytes     int
	CompactCount int
}

// escapeXML escapes XML special characters.
func escapeXML(s string) string {
	return html.EscapeString(s)
}

// truncateString truncates a string to max bytes.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}

	return s[:maxLen]
}

// BuildResumeSnapshot builds a resume snapshot XML string from stored session events.
//
// Algorithm:
// 1. Group events by category
// 2. Render each section
// 3. Assemble by priority tier with budget trimming
// 4. If over maxBytes, drop lowest priority sections first
func BuildResumeSnapshot(events []model.StoredEvent, opts *BuildSnapshotOpts) string { //nolint:maintidx // complex by nature; refactor tracked separately
	maxBytes := SnapshotBudget()
	compactCount := 1

	if opts != nil {
		if opts.MaxBytes > 0 {
			maxBytes = opts.MaxBytes
		}

		if opts.CompactCount > 0 {
			compactCount = opts.CompactCount
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)

	// Group events by category.
	var (
		fileEvents     []model.StoredEvent
		taskEvents     []model.StoredEvent
		ruleEvents     []model.StoredEvent
		decisionEvents []model.StoredEvent
		cwdEvents      []model.StoredEvent
		errorEvents    []model.StoredEvent
		envEvents      []model.StoredEvent
		gitEvents      []model.StoredEvent
		subagentEvents []model.StoredEvent
		intentEvents   []model.StoredEvent
		mcpEvents      []model.StoredEvent
		planEvents     []model.StoredEvent
		goalEvents     []model.StoredEvent
	)

	for _, ev := range events {
		switch ev.Category {
		case "file":
			fileEvents = append(fileEvents, ev)
		case "task":
			taskEvents = append(taskEvents, ev)
		case "rule":
			ruleEvents = append(ruleEvents, ev)
		case "decision":
			decisionEvents = append(decisionEvents, ev)
		case "cwd":
			cwdEvents = append(cwdEvents, ev)
		case "error":
			errorEvents = append(errorEvents, ev)
		case "env":
			envEvents = append(envEvents, ev)
		case "git":
			gitEvents = append(gitEvents, ev)
		case "subagent":
			subagentEvents = append(subagentEvents, ev)
		case "intent":
			intentEvents = append(intentEvents, ev)
		case "mcp":
			mcpEvents = append(mcpEvents, ev)
		case "plan":
			planEvents = append(planEvents, ev)
		}

		// Collect events with a goal tag for cross-category grouping.
		goalEvents = collectGoalEvent(goalEvents, ev)
	}

	// P1 sections (50% budget): active_files, task_state, rules
	var p1Sections []string
	if s := renderActiveFiles(fileEvents); s != "" {
		p1Sections = append(p1Sections, s)
	}

	if s := renderTaskState(taskEvents); s != "" {
		p1Sections = append(p1Sections, s)
	}

	if s := renderRules(ruleEvents); s != "" {
		p1Sections = append(p1Sections, s)
	}

	// P2 sections (35% budget): decisions, environment, errors, completed subagents, plan
	var p2Sections []string
	if s := renderDecisions(decisionEvents); s != "" {
		p2Sections = append(p2Sections, s)
	}

	var lastCwd, lastGit *model.StoredEvent
	if len(cwdEvents) > 0 {
		lastCwd = &cwdEvents[len(cwdEvents)-1]
	}

	if len(gitEvents) > 0 {
		lastGit = &gitEvents[len(gitEvents)-1]
	}

	if s := renderEnvironment(lastCwd, envEvents, lastGit); s != "" {
		p2Sections = append(p2Sections, s)
	}

	if s := renderErrors(errorEvents); s != "" {
		p2Sections = append(p2Sections, s)
	}

	var completedSubagents []model.StoredEvent

	for _, e := range subagentEvents {
		if e.Type == "subagent_completed" {
			completedSubagents = append(completedSubagents, e)
		}
	}

	if s := renderSubagents(completedSubagents); s != "" {
		p2Sections = append(p2Sections, s)
	}

	if s := renderGoals(goalEvents); s != "" {
		p2Sections = append(p2Sections, s)
	}

	if len(planEvents) > 0 {
		last := planEvents[len(planEvents)-1]
		switch last.Type {
		case "plan_enter":
			p2Sections = append(p2Sections, `  <plan_mode status="active">Awaiting approval — do NOT proceed with implementation until the plan is approved.</plan_mode>`)
		case "plan_approved":
			p2Sections = append(p2Sections, `  <plan_mode status="approved">Plan was approved — proceed with implementation. Do NOT re-propose the plan.</plan_mode>`)
		case "plan_rejected":
			p2Sections = append(p2Sections, `  <plan_mode status="rejected">Plan was rejected — ask the user what changes they want before re-proposing.</plan_mode>`)
		}
	}

	// P3-P4 sections (15% budget): intent, mcp_tools, launched subagents
	var p3Sections []string

	if len(intentEvents) > 0 {
		last := intentEvents[len(intentEvents)-1]
		p3Sections = append(p3Sections, renderIntent(last))
	}

	if s := renderMcpTools(mcpEvents); s != "" {
		p3Sections = append(p3Sections, s)
	}

	var launchedSubagents []model.StoredEvent

	for _, e := range subagentEvents {
		if e.Type == "subagent_launched" {
			launchedSubagents = append(launchedSubagents, e)
		}
	}

	if s := renderSubagents(launchedSubagents); s != "" {
		p3Sections = append(p3Sections, s)
	}

	// Assemble with budget trimming.
	header := fmt.Sprintf(`<session_resume compact_count="%d" events_captured="%d" generated_at="%s">`,
		compactCount, len(events), now)
	footer := `</session_resume>`

	tiers := [][]string{p1Sections, p2Sections, p3Sections}

	for dropFrom := len(tiers); dropFrom >= 0; dropFrom-- {
		active := tiers[:dropFrom]

		var allSections []string
		for _, tier := range active {
			allSections = append(allSections, tier...)
		}

		body := strings.Join(allSections, "\n")

		var xml string
		if body != "" {
			xml = header + "\n" + body + "\n" + footer
		} else {
			xml = header + "\n" + footer
		}

		if len(xml) <= maxBytes {
			return xml
		}
	}

	return header + "\n" + footer
}

// ── Section renderers ──

func renderActiveFiles(fileEvents []model.StoredEvent) string {
	if len(fileEvents) == 0 {
		return ""
	}

	type fileEntry struct {
		ops  map[string]int
		last string
	}

	fileMap := make(map[string]*fileEntry)

	var order []string

	for _, ev := range fileEvents {
		path := ev.Data

		entry, exists := fileMap[path]
		if !exists {
			entry = &fileEntry{ops: make(map[string]int)}
			fileMap[path] = entry
			order = append(order, path)
		}

		var op string

		switch ev.Type {
		case "file_write":
			op = "write"
		case "file_read":
			op = "read"
		case "file_edit":
			op = "edit"
		default:
			op = ev.Type
		}

		entry.ops[op]++
		entry.last = op
	}

	// Limit to last MaxActiveFiles.
	if len(order) > MaxActiveFiles {
		order = order[len(order)-MaxActiveFiles:]
	}

	lines := []string{"  <active_files>"}

	for _, path := range order {
		entry := fileMap[path]

		var opParts []string
		for k, v := range entry.ops {
			opParts = append(opParts, k+":"+strconv.Itoa(v))
		}

		opsStr := strings.Join(opParts, ",")
		lines = append(lines, fmt.Sprintf(`    <file path="%s" ops="%s" last="%s" />`,
			escapeXML(path), escapeXML(opsStr), escapeXML(entry.last)))
	}

	lines = append(lines, "  </active_files>")

	return strings.Join(lines, "\n")
}

func renderTaskState(taskEvents []model.StoredEvent) string {
	if len(taskEvents) == 0 {
		return ""
	}

	var creates []string

	updates := make(map[string]string)

	for _, ev := range taskEvents {
		var parsed map[string]any
		if err := json.Unmarshal([]byte(ev.Data), &parsed); err != nil {
			continue
		}

		if subject, ok := parsed["subject"].(string); ok {
			creates = append(creates, subject)
		} else if taskID, ok := parsed["taskId"].(string); ok {
			if status, ok := parsed["status"].(string); ok {
				updates[taskID] = status
			}
		}
	}

	if len(creates) == 0 {
		return ""
	}

	done := map[string]bool{"completed": true, "deleted": true, "failed": true}

	// Sort IDs numerically.
	var sortedIDs []string
	for id := range updates {
		sortedIDs = append(sortedIDs, id)
	}
	// Simple sort — IDs are typically small integers.
	for i := 0; i < len(sortedIDs); i++ {
		for j := i + 1; j < len(sortedIDs); j++ {
			a, _ := strconv.Atoi(sortedIDs[i])

			b, _ := strconv.Atoi(sortedIDs[j])
			if a > b {
				sortedIDs[i], sortedIDs[j] = sortedIDs[j], sortedIDs[i]
			}
		}
	}

	var pending []string

	for i, subject := range creates {
		status := "pending"

		if i < len(sortedIDs) {
			if s, ok := updates[sortedIDs[i]]; ok {
				status = s
			}
		}

		if !done[status] {
			pending = append(pending, subject)
		}
	}

	if len(pending) == 0 {
		return ""
	}

	lines := []string{"  <task_state>"}
	for _, task := range pending {
		lines = append(lines, "    - "+escapeXML(truncateString(task, 100)))
	}

	lines = append(lines, "  </task_state>")

	return strings.Join(lines, "\n")
}

func renderRules(ruleEvents []model.StoredEvent) string {
	if len(ruleEvents) == 0 {
		return ""
	}

	seen := make(map[string]bool)
	lines := []string{"  <rules>"}

	for _, ev := range ruleEvents {
		key := ev.Data
		if seen[key] {
			continue
		}

		seen[key] = true

		if ev.Type == "rule_content" {
			lines = append(lines, fmt.Sprintf("    <rule_content>%s</rule_content>",
				escapeXML(truncateString(ev.Data, 400))))
		} else {
			lines = append(lines, "    - "+escapeXML(truncateString(ev.Data, 200)))
		}
	}

	lines = append(lines, "  </rules>")

	return strings.Join(lines, "\n")
}

func renderDecisions(decisionEvents []model.StoredEvent) string {
	if len(decisionEvents) == 0 {
		return ""
	}

	seen := make(map[string]bool)
	lines := []string{"  <decisions>"}

	for _, ev := range decisionEvents {
		if seen[ev.Data] {
			continue
		}

		seen[ev.Data] = true
		lines = append(lines, "    - "+escapeXML(truncateString(ev.Data, 200)))
	}

	lines = append(lines, "  </decisions>")

	return strings.Join(lines, "\n")
}

func renderEnvironment(cwdEvent *model.StoredEvent, envEvents []model.StoredEvent, gitEvent *model.StoredEvent) string {
	if cwdEvent == nil && len(envEvents) == 0 && gitEvent == nil {
		return ""
	}

	parts := []string{"  <environment>"}

	if cwdEvent != nil {
		parts = append(parts, fmt.Sprintf("    <cwd>%s</cwd>", escapeXML(cwdEvent.Data)))
	}

	if gitEvent != nil {
		parts = append(parts, fmt.Sprintf(`    <git op="%s" />`, escapeXML(gitEvent.Data)))
	}

	for _, env := range envEvents {
		parts = append(parts, fmt.Sprintf("    <env>%s</env>", escapeXML(truncateString(env.Data, 150))))
	}

	parts = append(parts, "  </environment>")

	return strings.Join(parts, "\n")
}

func renderErrors(errorEvents []model.StoredEvent) string {
	if len(errorEvents) == 0 {
		return ""
	}

	lines := []string{"  <errors_encountered>"}
	for _, ev := range errorEvents {
		lines = append(lines, "    - "+escapeXML(truncateString(ev.Data, 150)))
	}

	lines = append(lines, "  </errors_encountered>")

	return strings.Join(lines, "\n")
}

func renderIntent(intentEvent model.StoredEvent) string {
	return fmt.Sprintf(`  <intent mode="%s">%s</intent>`,
		escapeXML(intentEvent.Data), escapeXML(truncateString(intentEvent.Data, 100)))
}

func renderSubagents(subagentEvents []model.StoredEvent) string {
	if len(subagentEvents) == 0 {
		return ""
	}

	lines := []string{"  <subagents>"}

	for _, ev := range subagentEvents {
		status := "unknown"

		switch ev.Type {
		case "subagent_completed":
			status = "completed"
		case "subagent_launched":
			status = "launched"
		}

		lines = append(lines, fmt.Sprintf(`    <agent status="%s">%s</agent>`,
			status, escapeXML(truncateString(ev.Data, 200))))
	}

	lines = append(lines, "  </subagents>")

	return strings.Join(lines, "\n")
}

// collectGoalEvent appends ev to goalEvents if its Data contains a "goal" JSON key.
func collectGoalEvent(goalEvents []model.StoredEvent, ev model.StoredEvent) []model.StoredEvent {
	// Fast check: Data must look like JSON with a "goal" field.
	if !strings.Contains(ev.Data, `"goal"`) {
		return goalEvents
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(ev.Data), &parsed); err != nil {
		return goalEvents
	}

	if _, ok := parsed["goal"].(string); ok {
		return append(goalEvents, ev)
	}

	return goalEvents
}

// renderGoals groups events by their shared "goal" tag and renders a summary.
func renderGoals(goalEvents []model.StoredEvent) string {
	if len(goalEvents) == 0 {
		return ""
	}

	type goalGroup struct {
		events []model.StoredEvent
	}

	groups := make(map[string]*goalGroup)

	var order []string

	for _, ev := range goalEvents {
		var parsed map[string]any
		if err := json.Unmarshal([]byte(ev.Data), &parsed); err != nil {
			continue
		}

		goal, ok := parsed["goal"].(string)
		if !ok || goal == "" {
			continue
		}

		g, exists := groups[goal]
		if !exists {
			g = &goalGroup{}
			groups[goal] = g
			order = append(order, goal)
		}

		g.events = append(g.events, ev)
	}

	if len(order) == 0 {
		return ""
	}

	lines := []string{"  <goals>"}

	for _, goal := range order {
		g := groups[goal]

		// Count categories involved in this goal.
		cats := make(map[string]int)
		for _, ev := range g.events {
			cats[ev.Category]++
		}

		var catParts []string
		for k, v := range cats {
			catParts = append(catParts, k+":"+strconv.Itoa(v))
		}

		lines = append(lines, fmt.Sprintf(`    <goal name="%s" events="%d" categories="%s" />`,
			escapeXML(truncateString(goal, 100)), len(g.events), escapeXML(strings.Join(catParts, ","))))
	}

	lines = append(lines, "  </goals>")

	return strings.Join(lines, "\n")
}

func renderMcpTools(mcpEvents []model.StoredEvent) string {
	if len(mcpEvents) == 0 {
		return ""
	}

	toolCounts := make(map[string]int)

	var order []string

	for _, ev := range mcpEvents {
		tool := strings.SplitN(ev.Data, ":", 2)[0]

		tool = strings.TrimSpace(tool)
		if _, exists := toolCounts[tool]; !exists {
			order = append(order, tool)
		}

		toolCounts[tool]++
	}

	lines := []string{"  <mcp_tools>"}
	for _, tool := range order {
		lines = append(lines, fmt.Sprintf(`    <tool name="%s" calls="%d" />`,
			escapeXML(tool), toolCounts[tool]))
	}

	lines = append(lines, "  </mcp_tools>")

	return strings.Join(lines, "\n")
}
