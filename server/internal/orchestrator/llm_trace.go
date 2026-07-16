package orchestrator

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"context"

	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator/llmrunner"
	"github.com/auto-code-os/auto-code-os/server/internal/prompts"
	"github.com/auto-code-os/auto-code-os/server/pkg/llm"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

var secretRegexes = []*regexp.Regexp{
	regexp.MustCompile(`(?i)ghp_[a-zA-Z0-9]{36}`),
	regexp.MustCompile(`(?i)github_pat_[a-zA-Z0-9_]{82}`),
	regexp.MustCompile(`(?i)sk-[a-zA-Z0-9]{48}`),
	regexp.MustCompile(`(?i)sk-proj-[a-zA-Z0-9-_]{150,}`),
	regexp.MustCompile(`(?i)sk-ant-[a-zA-Z0-9-_]{90,}`),
	regexp.MustCompile(`(?i)AIzaSy[a-zA-Z0-9-_]{33}`),
}

func redactSecrets(s string) string {
	for _, re := range secretRegexes {
		s = re.ReplaceAllString(s, "[REDACTED]")
	}
	return s
}

func (o *Orchestrator) writeLLMCallTrace(ctx context.Context, task *models.Task, agent *models.Agent, stepID string, messages []llm.Message, resp *llm.Response, parsed map[string]any, counters llmrunner.TraceCounters, latency time.Duration) {
	if !o.llmTraceEnabled {
		return
	}

	o.initWkspace()
	ws := o.wkspace.GetTaskWorkspace(task)
	traceRoot := filepath.Join(ws.Root, "logs", "llm")
	_ = os.MkdirAll(traceRoot, 0o755)

	callNumber := 1
	if files, err := os.ReadDir(traceRoot); err == nil {
		for _, f := range files {
			if f.IsDir() && strings.HasPrefix(f.Name(), "call-") {
				parts := strings.SplitN(f.Name(), "-", 3)
				if len(parts) >= 2 {
					var n int
					if _, errScan := fmt.Sscanf(parts[1], "%d", &n); errScan == nil {
						if n >= callNumber {
							callNumber = n + 1
						}
					}
				}
			}
		}
	}

	callDirName := fmt.Sprintf("call-%03d-%s", callNumber, stepID)
	callPath := filepath.Join(traceRoot, callDirName)
	_ = os.MkdirAll(callPath, 0o755)

	type TraceMetadata struct {
		Step            string    `json:"step"`
		CallNumber      int       `json:"call_number"`
		Model           string    `json:"model"`
		PromptTokens    int       `json:"prompt_tokens"`
		OutputTokens    int       `json:"output_tokens"`
		AgentID         string    `json:"agent_id"`
		AgentName       string    `json:"agent_name"`
		Role            string    `json:"role"`
		Timestamp       time.Time `json:"timestamp"`
		PromptHash      string    `json:"prompt_hash"`
		TemplateVersion string    `json:"template_version,omitempty"`
		ContextVersion  string    `json:"context_version,omitempty"`
		Iteration       int       `json:"iteration"`
		RetryAttempt    int       `json:"retry_attempt"`
		CallKind        string    `json:"call_kind,omitempty"`
		LatencyMS       int64     `json:"latency_ms"`
		CostUSD         float64   `json:"cost_usd"`
	}

	var promptForHash strings.Builder
	for _, msg := range messages {
		promptForHash.WriteString(msg.Role)
		promptForHash.WriteString(":")
		promptForHash.WriteString(msg.Content)
		promptForHash.WriteString("\n")
	}
	promptHash := fmt.Sprintf("%x", sha256.Sum256([]byte(promptForHash.String())))

	var analysis models.TaskAnalysis
	if len(task.Analysis) > 0 {
		_ = json.Unmarshal(task.Analysis, &analysis)
	}

	costUSD := llm.EstimateCost(resp.PromptTokens, resp.OutputTokens, llm.MetadataForModel("", resp.Model))

	meta := TraceMetadata{
		Step:           stepID,
		CallNumber:     callNumber,
		Model:          resp.Model,
		PromptTokens:   resp.PromptTokens,
		OutputTokens:   resp.OutputTokens,
		AgentID:        agent.ID,
		AgentName:      agent.Name,
		Role:           agent.Role,
		Timestamp:      time.Now(),
		PromptHash:     promptHash,
		ContextVersion: analysis.SpecHash,
		Iteration:      counters.Iteration,
		RetryAttempt:   counters.RetryAttempt,
		CallKind:       counters.Kind,
		LatencyMS:      latency.Milliseconds(),
		CostUSD:        costUSD,
	}
	metaJSON, _ := json.MarshalIndent(meta, "", "  ")
	_ = os.WriteFile(filepath.Join(callPath, "metadata.json"), metaJSON, 0o644)

	if strings.ToLower(o.llmLogLevel) == "info" {
		return
	}

	reqJSON, _ := json.MarshalIndent(messages, "", "  ")
	_ = os.WriteFile(filepath.Join(callPath, "request.json"), []byte(redactSecrets(string(reqJSON))), 0o644)

	resJSON, _ := json.MarshalIndent(resp, "", "  ")
	_ = os.WriteFile(filepath.Join(callPath, "response.json"), []byte(redactSecrets(string(resJSON))), 0o644)

	var promptBuilder strings.Builder
	promptBuilder.WriteString("# LLM Request Prompt Reconstructed\n\n")
	for _, msg := range messages {
		promptBuilder.WriteString(fmt.Sprintf("## Role: %s\n\n%s\n\n---\n\n", msg.Role, msg.Content))
	}
	_ = os.WriteFile(filepath.Join(callPath, "prompt.md"), []byte(redactSecrets(promptBuilder.String())), 0o644)

	_ = os.WriteFile(filepath.Join(callPath, "output.md"), []byte(redactSecrets(resp.Content)), 0o644)

	if len(parsed) > 0 {
		parsedJSON, _ := json.MarshalIndent(parsed, "", "  ")
		_ = os.WriteFile(filepath.Join(callPath, "parsed.json"), []byte(redactSecrets(string(parsedJSON))), 0o644)
	}

	if budgetTrace := prompts.BudgetTraceFromCtx(ctx); budgetTrace != nil && len(budgetTrace.Logs) > 0 {
		budgetLogJSON, _ := json.MarshalIndent(budgetTrace.Logs, "", "  ")
		_ = os.WriteFile(filepath.Join(callPath, "budget_log.json"), []byte(redactSecrets(string(budgetLogJSON))), 0o644)
	}
}
