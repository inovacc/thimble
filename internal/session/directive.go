package session

import (
	"fmt"
	"strings"

	"github.com/inovacc/thimble/internal/model"
)

// BuildSessionDirective produces a compact XML knowledge block from session events
// for injection into resume/compact contexts. The source parameter identifies the
// trigger ("resume" or "compact").
func BuildSessionDirective(source string, events []model.StoredEvent) string {
	if len(events) == 0 {
		return ""
	}

	var b strings.Builder

	_, _ = fmt.Fprintf(&b, "<session_knowledge source=%q events=%d>\n", source, len(events))

	// Deduplicate by category, keeping last occurrence.
	seen := make(map[string]struct{})

	var unique []model.StoredEvent

	for i := len(events) - 1; i >= 0; i-- {
		key := events[i].Type + ":" + events[i].Category
		if _, ok := seen[key]; ok {
			continue
		}

		seen[key] = struct{}{}

		unique = append([]model.StoredEvent{events[i]}, unique...)
	}

	for _, e := range unique {
		_, _ = fmt.Fprintf(&b, "  <event type=%q category=%q>%s</event>\n", e.Type, e.Category, e.Data)
	}

	b.WriteString("</session_knowledge>")

	return b.String()
}
