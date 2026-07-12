package workflow

import (
	"strings"
)

var frontendSignals = []string{"frontend", "ui", "component", "page", "view", "style", "css", "layout", "giao diện", "giao dien"}
var backendSignals = []string{"backend", "server", "api", "database", "db", "migration", "model", "service", "handler", "cơ sở dữ liệu", "co so du lieu"}

// ClassifyHeading is the single source of truth for bucketing a TasksMD heading into
// "frontend" or "backend". Any other code that needs to know which role a heading
// belongs to (e.g. prompts.extractSpecsSectionForSubtask) must call this rather than
// keep a separate keyword list, or the two classifications can drift apart (REQ-M01).
func ClassifyHeading(heading string) string {
	lower := strings.ToLower(heading)
	for _, signal := range frontendSignals {
		if strings.Contains(lower, signal) {
			return "frontend"
		}
	}
	for _, signal := range backendSignals {
		if strings.Contains(lower, signal) {
			return "backend"
		}
	}
	return "backend"
}

func isCheckboxLine(line string) bool {
	return strings.HasPrefix(line, "- [ ]") || strings.HasPrefix(line, "- [x]") || strings.HasPrefix(line, "- [X]")
}

func extractCheckboxText(line string) string {
	for _, prefix := range []string{"- [ ] ", "- [x] ", "- [X] "} {
		if strings.HasPrefix(line, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(line, prefix))
		}
	}
	if len(line) > 6 {
		return strings.TrimSpace(line[6:])
	}
	return ""
}

// ParseTasksMD parses the tasks markdown and groups checkboxes under their respective ## headings.
// This prevents task fragmentation by treating each major heading group as a single subtask.
func ParseTasksMD(tasksMD string) map[string][]string {
	result := map[string][]string{}
	if strings.TrimSpace(tasksMD) == "" {
		return result
	}

	lines := strings.Split(tasksMD, "\n")
	var sections []struct {
		heading string
		role    string
		items   []string
	}

	var currentSection *struct {
		heading string
		role    string
		items   []string
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "## ") {
			if currentSection != nil && len(currentSection.items) > 0 {
				sections = append(sections, *currentSection)
			}
			heading := strings.TrimPrefix(trimmed, "## ")
			role := ClassifyHeading(heading)
			currentSection = &struct {
				heading string
				role    string
				items   []string
			}{
				heading: trimmed,
				role:    role,
				items:   []string{},
			}
			continue
		}

		if isCheckboxLine(trimmed) && currentSection != nil {
			currentSection.items = append(currentSection.items, trimmed)
		}
	}

	if currentSection != nil && len(currentSection.items) > 0 {
		sections = append(sections, *currentSection)
	}

	for _, sec := range sections {
		var sb strings.Builder
		sb.WriteString(sec.heading)
		for _, item := range sec.items {
			sb.WriteString("\n")
			sb.WriteString(item)
		}
		result[sec.role] = append(result[sec.role], sb.String())
	}

	// Fallback to original parsing if no ## headings with checkboxes were found
	if len(result["backend"]) == 0 && len(result["frontend"]) == 0 {
		currentRole := ""
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "## ") {
				heading := strings.ToLower(strings.TrimPrefix(trimmed, "## "))
				currentRole = ClassifyHeading(heading)
				continue
			}
			if isCheckboxLine(trimmed) && currentRole != "" {
				item := extractCheckboxText(trimmed)
				if item != "" {
					result[currentRole] = append(result[currentRole], item)
				}
			}
		}
	}

	return result
}
