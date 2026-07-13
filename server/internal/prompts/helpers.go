package prompts

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/llm"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"github.com/auto-code-os/auto-code-os/server/pkg/paths"
)

func formatTasksMD(tasks []models.TaskDAG) string {
	var b strings.Builder
	for i, t := range tasks {
		b.WriteString(fmt.Sprintf("## %d. %s\n", i+1, t.ID))
		if t.Complexity != nil {
			b.WriteString(fmt.Sprintf("- Complexity: Arch=%s, Mig=%v, Break=%v\n", t.Complexity.Architecture, t.Complexity.DataMigration, t.Complexity.BreakingChange))
		}
	}
	return b.String()
}

func shouldAttachCodeContext(agent *models.Agent) bool {
	return true
}

func formatMemories(memories []models.EpisodicMemory) string {
	var b strings.Builder
	for _, mem := range memories {
		b.WriteString(fmt.Sprintf("[%s/%s] %s\n", mem.Tier, mem.Category, mem.Summary))
		if mem.Content != "" && mem.Content != mem.Summary {
			b.WriteString(fmt.Sprintf("Detail: %s\n", mem.Content))
		}
		b.WriteString("\n")
	}
	return b.String()
}

func formatContextSnippets(snippets []models.ContextSnippet) string {
	var b strings.Builder
	for i, snippet := range snippets {
		displayPath := paths.WorkspaceToRepoRelative(snippet.Path)
		b.WriteString(fmt.Sprintf("### Snippet %d: %s:%d-%d (score %.2f, %s)\n", i+1, displayPath, snippet.StartLine, snippet.EndLine, snippet.Relevance, snippet.Retriever))
		b.WriteString("```")
		b.WriteString(displayPath)
		b.WriteString("\n")
		b.WriteString(snippet.Content)
		if !strings.HasSuffix(snippet.Content, "\n") {
			b.WriteString("\n")
		}
		b.WriteString("```\n")
	}
	return b.String()
}

// snippetDedupOverlapThreshold is the line-range overlap fraction above which a snippet is
// considered a duplicate of an already-kept snippet from the same file (REQ-M07).
const snippetDedupOverlapThreshold = 0.5

// deduplicateSnippets removes snippets that overlap more than snippetDedupOverlapThreshold with
// an already-kept snippet from the same file. Keeps the first occurrence.
func deduplicateSnippets(snippets []models.ContextSnippet) []models.ContextSnippet {
	var result []models.ContextSnippet
	for _, s := range snippets {
		isDup := false
		for _, kept := range result {
			if kept.Path == s.Path && lineOverlap(kept, s) > snippetDedupOverlapThreshold {
				isDup = true
				break
			}
		}
		if !isDup {
			result = append(result, s)
		}
	}
	return result
}

// filterAffectedFileSnippets drops snippets for files already delivered in full
// elsewhere (llmrunner.Runner injects full content for AffectedFiles on every
// call for coding/fix/review steps), so the same file isn't sent twice.
func filterAffectedFileSnippets(snippets []models.ContextSnippet, affectedFiles []models.AffectedFile) []models.ContextSnippet {
	if len(affectedFiles) == 0 {
		return snippets
	}
	affected := make(map[string]bool, len(affectedFiles))
	for _, af := range affectedFiles {
		affected[af.File] = true
	}
	var result []models.ContextSnippet
	for _, s := range snippets {
		if affected[s.Path] {
			continue
		}
		result = append(result, s)
	}
	return result
}

// lineOverlap returns the fraction of the shorter snippet's line range
// that overlaps with the other snippet.
func lineOverlap(a, b models.ContextSnippet) float64 {
	overlapStart := a.StartLine
	if b.StartLine > overlapStart {
		overlapStart = b.StartLine
	}
	overlapEnd := a.EndLine
	if b.EndLine < overlapEnd {
		overlapEnd = b.EndLine
	}
	if overlapStart >= overlapEnd {
		return 0
	}
	overlap := float64(overlapEnd - overlapStart)
	aLen := float64(a.EndLine - a.StartLine)
	bLen := float64(b.EndLine - b.StartLine)
	shorter := aLen
	if bLen < shorter {
		shorter = bLen
	}
	if shorter <= 0 {
		return 0
	}
	return overlap / shorter
}

func TruncateHistory(history []llm.Message, maxChars int) []llm.Message {
	if maxChars <= 0 || len(history) == 0 {
		return nil
	}
	selected := []llm.Message{}
	total := 0
	for i := len(history) - 1; i >= 0; i-- {
		msg := history[i]
		size := len(msg.Role) + len(msg.Content)
		if total+size > maxChars {
			selected = append(selected, llm.Message{
				Role:    "system",
				Content: fmt.Sprintf("Earlier conversation summarized: %d messages omitted to stay within token budget.", i+1),
			})
			break
		}
		total += size
		selected = append(selected, msg)
	}
	for i, j := 0, len(selected)-1; i < j; i, j = i+1, j-1 {
		selected[i], selected[j] = selected[j], selected[i]
	}
	return selected
}

func appendSystemPrompt(core string, metadata map[string]any) string {
	if len(metadata) == 0 {
		return core
	}
	metadataJSON, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return core
	}
	return fmt.Sprintf("%s\n\n=== Task Configuration ===\n```json\n%s\n```", core, string(metadataJSON))
}

// extractSubtaskIndex extracts the numeric index from a step ID (e.g., "code_backend_2" -> 2)
func extractSubtaskIndex(stepID string) (int, bool) {
	parts := strings.Split(stepID, "_")
	if len(parts) > 0 {
		idx, err := strconv.Atoi(parts[len(parts)-1])
		if err == nil {
			return idx, true
		}
	}
	return -1, false
}

// extractSpecsSectionForSubtask attempts to find the relevant section in SpecsMD
// by matching the heading number corresponding to the current subtask.
func extractSpecsSectionForSubtask(specsMD, tasksMD string, subtaskIndex int, stepID string) string {
	if specsMD == "" || tasksMD == "" || subtaskIndex < 0 {
		return ""
	}

	// Determine role to find the correct heading in TasksMD
	role := "backend"
	if strings.Contains(stepID, "frontend") {
		role = "frontend"
	}

	// 1. Find the Nth heading for this role in TasksMD to extract its number
	lines := strings.Split(tasksMD, "\n")
	roleIdx := 0
	headingNumber := ""

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## ") {
			heading := strings.TrimPrefix(trimmed, "## ")
			// Delegate to the same classifier ParseTasksMD uses (REQ-M01), so the index
			// found here can never drift from the index used to bucket TasksMD by role.
			if workflow.ClassifyHeading(heading) == role {
				if roleIdx == subtaskIndex {
					// Extract number from heading, e.g. "## 6. Thiết lập" -> "6"
					re := regexp.MustCompile(`##\s*(\d+)[\.\s]`)
					matches := re.FindStringSubmatch(trimmed)
					if len(matches) > 1 {
						headingNumber = matches[1]
					}
					break
				}
				roleIdx++
			}
		}
	}

	if headingNumber == "" {
		// Fallback: just use subtaskIndex + 1 if no explicit number found
		headingNumber = strconv.Itoa(subtaskIndex + 1)
	}

	// 2. Find the corresponding section in SpecsMD
	// Look for a heading that contains this number
	specsLines := strings.Split(specsMD, "\n")
	var sectionBuilder strings.Builder
	inTargetSection := false

	targetRe := regexp.MustCompile(`(?i)^#{2,4}\s*.*(?:requirement|yêu cầu)?:?\s*0*` + headingNumber + `[\.\s]`)
	nextHeadingRe := regexp.MustCompile(`^#{2,4}\s`)

	for _, line := range specsLines {
		trimmed := strings.TrimSpace(line)
		isHeading := nextHeadingRe.MatchString(trimmed)

		if inTargetSection {
			if isHeading {
				// Stop if we hit a heading of the same or higher level
				// (Simplified: stop at any new major heading)
				break
			}
			sectionBuilder.WriteString(line + "\n")
		} else if isHeading && targetRe.MatchString(trimmed) {
			inTargetSection = true
			sectionBuilder.WriteString(line + "\n")
		}
	}

	return strings.TrimSpace(sectionBuilder.String())
}

// summarizeTasksProgress creates a concise summary of workflow progress
func summarizeTasksProgress(tasksMD string, subtaskIndex int, stepID string) string {
	if tasksMD == "" {
		return ""
	}
	role := "backend"
	if strings.Contains(stepID, "frontend") {
		role = "frontend"
	}
	return fmt.Sprintf("Progress: Completed %d %s subtask groups. Working on group %d.", subtaskIndex, role, subtaskIndex+1)
}
