package llmrunner

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator/prompt"
	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator/workspace"
	"github.com/auto-code-os/auto-code-os/server/internal/context/provider"
	"github.com/auto-code-os/auto-code-os/server/internal/sandbox"
	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/llm"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

type PromptAssembler func(context.Context, models.Task, *models.Agent, []llm.Message) ([]llm.Message, error)

type ProjectResolver interface {
	GetByID(ctx context.Context, id string) (*models.Project, error)
}

type Runner struct {
	WorkspaceRoot           string
	Provider                llm.Provider
	AssemblePrompt          PromptAssembler
	Projects                ProjectResolver
	ReadAffectedFileContent func(context.Context, *models.Task, string) (string, bool)
	SaveArtifact            func(context.Context, string, string, string, string, any) error
	WriteTrace              func(context.Context, *models.Task, *models.Agent, string, []llm.Message, *llm.Response, map[string]any)
	Log                     func(context.Context, string, *string, string, string)
}

func (r Runner) Run(ctx context.Context, task *models.Task, agent *models.Agent, jobID, stepID, instruction string) (map[string]any, error) {
	if r.Provider == nil {
		return nil, fmt.Errorf("llm provider is not configured")
	}
	localPath := sandbox.WorkspacePath(r.WorkspaceRoot, task.ID)
	ctx = context.WithValue(ctx, provider.WorkspaceRootKey, localPath)
	ctx = context.WithValue(ctx, prompt.StepIDCtxKey, stepID)

	messages, err := r.initialMessages(ctx, task, agent)
	if err != nil {
		return nil, err
	}
	fullInstruction := instruction

	var analysis models.TaskAnalysis
	if len(task.Analysis) > 0 {
		_ = json.Unmarshal(task.Analysis, &analysis)
	}
	if len(analysis.AffectedFiles) > 0 && shouldIncludeAffectedFiles(stepID) && r.ReadAffectedFileContent != nil {
		var b strings.Builder
		b.WriteString("\n\n### Workspace Affected Files ###\n")
		for _, file := range analysis.AffectedFiles {
			if content, ok := r.ReadAffectedFileContent(ctx, task, file); ok {
				displayPath := workspace.WorkspaceToRepoRelative(file)
				b.WriteString(fmt.Sprintf("\n--- %s ---\n```\n%s\n```\n", displayPath, content))
			}
		}
		fullInstruction += b.String()
	}

	if requiresStrictJSON(stepID) {
		fullInstruction += "\n\nCRITICAL REQUIREMENT: Do NOT output any tool calls, function calls, or markdown block thoughts. You do NOT have tool execution capabilities in this single-shot step. You MUST output ONLY a valid JSON object matching the requested format directly (or inside a ```json ``` block)."
	}
	if requiresPatch(stepID) {
		fullInstruction += "\n\nCRITICAL REQUIREMENT: The patch/diff field MUST contain a valid Unified Git Diff (starting with 'diff --git') representing all source code changes. Do NOT output raw file contents. Do NOT include any text outside the JSON structure."
	}
	messages = append(messages, llm.Message{Role: "user", Content: "Workflow step: " + stepID + "\n\n" + fullInstruction})

	r.save(ctx, jobID, task.ID, stepID, "prompt", messages)

	ctx = llm.WithRouteOptions(ctx, llm.RouteOptions{
		Complexity: task.Complexity,
		OrgID:      agent.OrgID,
		ProjectID:  task.ProjectID,
		AgentID:    agent.ID,
		TaskID:     task.ID,
		RouteName:  r.routeName(ctx, task, agent),
	})
	var resp *llm.Response
	var parsed map[string]any

	for attempt := 1; attempt <= 3; attempt++ {
		var chatErr error
		for chatAttempt := 1; chatAttempt <= 3; chatAttempt++ {
			resp, chatErr = r.Provider.Chat(ctx, messages)
			if chatErr == nil {
				break
			}
			if isTransientError(chatErr) && chatAttempt < 3 {
				r.log(ctx, task.ID, nil, "warn", fmt.Sprintf("%s: llm chat call failed (attempt %d/3) with transient error: %v. Retrying in %d seconds...", stepID, chatAttempt, chatErr, chatAttempt*2))
				time.Sleep(time.Duration(chatAttempt) * 2 * time.Second)
				continue
			}
			break
		}
		if chatErr != nil {
			return nil, fmt.Errorf("llm call failed: %w", chatErr)
		}
		r.log(ctx, task.ID, nil, "info", fmt.Sprintf("%s (attempt %d): llm response from %s", stepID, attempt, resp.Model))

		if parsedJSON, err := ParseJSONMarkdown(resp.Content); err == nil {
			parsed = parsedJSON
			break
		} else {
			r.log(ctx, task.ID, nil, "warn", fmt.Sprintf("%s: invalid JSON in response: %v", stepID, err))
			if attempt == 3 {
				parsed = map[string]any{"raw_content": resp.Content}
				break
			}
			content := resp.Content
			if content == "" {
				content = "(empty response)"
			}
			messages = append(messages, llm.Message{Role: "assistant", Content: content})
			messages = append(messages, llm.Message{
				Role:    "user",
				Content: fmt.Sprintf("Your output was not valid JSON. Error: %v. Please correct the syntax and output ONLY strictly valid JSON matching the requested format directly (or inside a ```json ``` block).", err),
			})
		}
	}

	r.save(ctx, jobID, task.ID, stepID, "llm_response", parsed)
	if r.WriteTrace != nil {
		r.WriteTrace(ctx, task, agent, stepID, messages, resp, parsed)
	}

	return map[string]any{
		"status":        "llm_completed",
		"model":         resp.Model,
		"content":       resp.Content,
		"parsed":        parsed,
		"prompt_tokens": resp.PromptTokens,
		"output_tokens": resp.OutputTokens,
	}, nil
}

func (r Runner) initialMessages(ctx context.Context, task *models.Task, agent *models.Agent) ([]llm.Message, error) {
	if r.AssemblePrompt != nil {
		return r.AssemblePrompt(ctx, *task, agent, nil)
	}
	return []llm.Message{{Role: "user", Content: task.Title + "\n\n" + task.Description}}, nil
}

func (r Runner) routeName(ctx context.Context, task *models.Task, agent *models.Agent) string {
	routeName := agent.ModelLevelGroup
	if r.Projects != nil {
		if p, err := r.Projects.GetByID(ctx, task.ProjectID); err == nil {
			if agent.Role == models.AgentRolePlanner && p.DefaultModelLevel != "" {
				routeName = p.DefaultModelLevel
			} else if (routeName == "" || routeName == "default") && p.DefaultModelLevel != "" {
				routeName = p.DefaultModelLevel
			}
		}
	}
	return routeName
}

func (r Runner) save(ctx context.Context, jobID, taskID, stepID, artType string, payload any) {
	if r.SaveArtifact != nil {
		_ = r.SaveArtifact(ctx, jobID, taskID, stepID, artType, payload)
	}
}

func (r Runner) log(ctx context.Context, taskID string, jobID *string, level, message string) {
	if r.Log != nil {
		r.Log(ctx, taskID, jobID, level, message)
	}
}

func shouldIncludeAffectedFiles(stepID string) bool {
	return strings.HasPrefix(stepID, workflow.StepCodeBackend) ||
		strings.HasPrefix(stepID, workflow.StepCodeFrontend) ||
		stepID == workflow.StepFix ||
		stepID == workflow.StepReview
}

func requiresStrictJSON(stepID string) bool {
	return strings.HasPrefix(stepID, workflow.StepCodeBackend) ||
		strings.HasPrefix(stepID, workflow.StepCodeFrontend) ||
		stepID == workflow.StepFix ||
		stepID == workflow.StepPlan ||
		stepID == workflow.StepAnalyze
}

func requiresPatch(stepID string) bool {
	return strings.HasPrefix(stepID, workflow.StepCodeBackend) ||
		strings.HasPrefix(stepID, workflow.StepCodeFrontend) ||
		stepID == workflow.StepFix
}

func isTransientError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "429") ||
		strings.Contains(msg, "500") ||
		strings.Contains(msg, "502") ||
		strings.Contains(msg, "503") ||
		strings.Contains(msg, "504") ||
		strings.Contains(msg, "rate limit") ||
		strings.Contains(msg, "limit exceeded") ||
		strings.Contains(msg, "resource exhausted") ||
		strings.Contains(msg, "timeout") ||
		strings.Contains(msg, "deadline exceeded") ||
		strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "eof")
}
