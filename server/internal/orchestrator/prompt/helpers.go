package prompt

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator/workspace"
	"github.com/auto-code-os/auto-code-os/server/pkg/llm"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

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
		displayPath := workspace.WorkspaceToRepoRelative(snippet.Path)
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

func personaFile(role string) string {
	switch strings.ToLower(role) {
	case models.AgentRolePlanner:
		return "project-planner.md"
	case models.AgentRoleFrontend:
		return "frontend-specialist.md"
	case models.AgentRoleReviewer:
		return "security-auditor.md"
	case models.AgentRoleQA:
		return "test-engineer.md"
	case models.AgentRoleDocumentationWriter:
		return "documentation-writer.md"
	default:
		return "backend-specialist.md"
	}
}

func readOptional(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return string(data), nil
}

func defaultPromptRoot() string {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return filepath.Clean(filepath.Join("..", "resources", "prompt_base"))
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", "..", "..", "resources", "prompt_base"))
}
