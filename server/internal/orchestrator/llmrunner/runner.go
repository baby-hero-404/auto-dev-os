package llmrunner

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/auto-code-os/auto-code-os/server/internal/context/provider"
	"github.com/auto-code-os/auto-code-os/server/internal/prompts"
	"github.com/auto-code-os/auto-code-os/server/internal/sandbox"
	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/llm"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"github.com/auto-code-os/auto-code-os/server/pkg/paths"
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
	WriteTrace              func(context.Context, *models.Task, *models.Agent, string, []llm.Message, *llm.Response, map[string]any, int, time.Duration)
	Log                     func(context.Context, string, *string, string, string)

	// Tools and ToolExecutor enable the agentic tool-call loop (Issue 1+2). When both are set,
	// Run() drives a native ChatWithOptions tool loop instead of the single-shot Chat() path.
	Tools        []llm.ToolDefinition
	ToolExecutor ToolExecutor
}

func (r Runner) isAgentic() bool {
	return len(r.Tools) > 0 && r.ToolExecutor != nil
}

func (r Runner) Run(ctx context.Context, task *models.Task, agent *models.Agent, jobID, stepID, instruction string) (map[string]any, error) {
	if r.Provider == nil {
		return nil, fmt.Errorf("llm provider is not configured")
	}
	localPath := sandbox.WorkspacePath(r.WorkspaceRoot, task.ID)
	ctx = context.WithValue(ctx, provider.WorkspaceRootKey, localPath)
	ctx = context.WithValue(ctx, prompts.StepIDCtxKey, stepID)

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
			displayPath := paths.WorkspaceToRepoRelative(file.File)
			if content, ok := r.ReadAffectedFileContent(ctx, task, file.File); ok {
				b.WriteString(fmt.Sprintf("\n--- %s ---\n```\n%s\n```\n", displayPath, content))
			} else {
				b.WriteString(fmt.Sprintf("\n--- %s [NEW FILE — does not exist yet] ---\nThis file needs to be created. Use the create_file tool.\n", displayPath))
			}
		}
		fullInstruction += b.String()
	}

	agentic := r.isAgentic()
	if agentic {
		if requiresPatch(stepID) {
			fullInstruction += "\n\nCRITICAL REQUIREMENT: Use the provided tools (e.g. search_replace, create_file) to make the required code changes directly in the workspace. Use run_tests/run_build/run_lint to verify your work before finishing. When done, respond with a JSON object containing a 'summary' field describing what you changed. Do NOT output a patch or diff — your tool calls already applied the changes."
		}
	} else {
		if requiresStrictJSON(stepID) {
			fullInstruction += "\n\nCRITICAL REQUIREMENT: Do NOT output any tool calls, function calls, or markdown block thoughts. You do NOT have tool execution capabilities in this single-shot step. You MUST output ONLY a valid JSON object matching the requested format directly (or inside a ```json ``` block)."
		}
		if requiresPatch(stepID) {
			fullInstruction += "\n\nCRITICAL REQUIREMENT: The patch/diff field MUST contain a valid Unified Git Diff (starting with 'diff --git') representing all source code changes. Do NOT output raw file contents. Do NOT include any text outside the JSON structure."
		}
	}
	messages = append(messages, llm.Message{Role: "user", Content: "Workflow step: " + stepID + "\n\n" + fullInstruction})

	r.save(ctx, jobID, task.ID, stepID, "prompt", messages)

	ctx = llm.WithRouteOptions(ctx, llm.RouteOptions{
		Complexity:     task.Complexity,
		OrgID:          agent.OrgID,
		ProjectID:      task.ProjectID,
		AgentID:        agent.ID,
		TaskID:         task.ID,
		RouteName:      r.routeName(ctx, task, agent),
		ExcludeModelID: llm.ExcludeModelIDFromContext(ctx),
	})

	if agentic {
		return r.runAgentic(ctx, task, agent, jobID, stepID, messages)
	}

	var resp *llm.Response
	var parsed map[string]any
	var finalAttempt int
	var callLatency time.Duration

	for attempt := 1; attempt <= 3; attempt++ {
		finalAttempt = attempt
		var chatErr error
		for chatAttempt := 1; chatAttempt <= 3; chatAttempt++ {
			callStart := time.Now()
			resp, chatErr = r.Provider.Chat(ctx, messages)
			callLatency = time.Since(callStart)
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

		parsedJSON, parseErr := ParseJSONMarkdown(resp.Content)
		if parseErr == nil {
			if schemaErr := r.validateSchema(stepID, parsedJSON, false); schemaErr != nil {
				parseErr = schemaErr
			} else if bizErr := r.validateBusiness(stepID, parsedJSON); bizErr != nil {
				parseErr = bizErr
			}
		}

		if parseErr == nil {
			parsed = parsedJSON
			break
		} else {
			var parseKind ParseErrorKind = ParseBusinessError
			var errMsg string = parseErr.Error()

			if cErr, ok := parseErr.(*ClassifiedParseError); ok {
				parseKind = cErr.Kind
				errMsg = cErr.Message
			}

			r.log(ctx, task.ID, nil, "warn", fmt.Sprintf("%s: parse failure [%s] (attempt %d/3): %s", stepID, parseKind, attempt, errMsg))
			if attempt == 3 {
				parsed = map[string]any{"raw_content": resp.Content, "error": errMsg}
				break
			}

			// FormatError: only local JSON repair is attempted (no LLM re-call)
			if parseKind == ParseFormatError {
				r.log(ctx, task.ID, nil, "warn", fmt.Sprintf("%s: format error, aborting LLM retry loop", stepID))
				parsed = map[string]any{"raw_content": resp.Content, "error": errMsg}
				break
			}

			content := resp.Content
			if content == "" {
				content = "(empty response)"
			}
			messages = append(messages, llm.Message{Role: "assistant", Content: content})

			var feedbackMsg string
			switch parseKind {
			case ParseTruncationError:
				feedbackMsg = fmt.Sprintf("Your response appeared to be truncated or incomplete. Error: %s. Please output the complete response again, making sure all JSON structures are properly closed.", errMsg)
			case ParseSchemaError:
				feedbackMsg = fmt.Sprintf("Your response was valid JSON but did not match the expected schema. Error: %s. Please correct the schema and output ONLY strictly valid JSON matching the requested format directly (or inside a ```json ``` block).", errMsg)
			case ParseBusinessError:
				feedbackMsg = fmt.Sprintf("Your response failed domain/business validation. Error: %s. Please correct the contents and output ONLY strictly valid JSON matching the requested format directly (or inside a ```json ``` block).", errMsg)
			default:
				feedbackMsg = fmt.Sprintf("Your output was not valid JSON. Error: %s. Please correct the syntax and output ONLY strictly valid JSON matching the requested format directly (or inside a ```json ``` block).", errMsg)
			}

			messages = append(messages, llm.Message{
				Role:    "user",
				Content: feedbackMsg,
			})
		}
	}

	r.save(ctx, jobID, task.ID, stepID, "llm_response", parsed)
	if r.WriteTrace != nil {
		r.WriteTrace(ctx, task, agent, stepID, messages, resp, parsed, finalAttempt, callLatency)
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

// runAgentic drives the native tool-calling loop (RunToolLoop) instead of the single-shot
// Chat() path, for steps where the caller resolved tools and a ToolExecutor (Issue 1+2).
func (r Runner) runAgentic(ctx context.Context, task *models.Task, agent *models.Agent, jobID, stepID string, messages []llm.Message) (map[string]any, error) {
	var lastResp *llm.Response

	parsed, _, partial, err := RunToolLoop(ctx, ToolLoopConfig{
		Messages:      messages,
		Tools:         r.Tools,
		MaxIterations: 8,
		Chat: func(ctx context.Context, msgs []llm.Message, opts llm.ChatOptions) (*llm.Response, error) {
			var resp *llm.Response
			var chatErr error
			for chatAttempt := 1; chatAttempt <= 3; chatAttempt++ {
				resp, chatErr = r.Provider.ChatWithOptions(ctx, msgs, opts)
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
			if chatErr == nil {
				lastResp = resp
			}
			return resp, chatErr
		},
		ExecuteTool: r.ToolExecutor,
		Validate: func(parsed map[string]any) error {
			if schemaErr := r.validateSchema(stepID, parsed, true); schemaErr != nil {
				return schemaErr
			}
			if bizErr := r.validateBusiness(stepID, parsed); bizErr != nil {
				return bizErr
			}
			return nil
		},
		OnCall: func(iteration int, msgs []llm.Message, resp *llm.Response, parsed map[string]any, latency time.Duration) {
			if r.WriteTrace != nil {
				r.WriteTrace(ctx, task, agent, stepID, msgs, resp, parsed, iteration, latency)
			}
		},
	})
	if err != nil {
		return nil, fmt.Errorf("agentic tool loop failed: %w", err)
	}

	if partial != nil && partial.Partial {
		// Iteration budget exhausted, but real edits already landed in the workspace — surface
		// this as a partial result (Issue 6) instead of a hard failure, so the caller
		// (patchRetryConfig) can decide whether to checkpoint + targeted-test the salvaged
		// edits rather than discarding completed work.
		r.log(ctx, task.ID, nil, "warn", fmt.Sprintf("%s: agentic tool loop exhausted its iteration budget but %d edit(s) were already applied; surfacing as a partial result", stepID, len(partial.EditsApplied)))
		content := ""
		var promptTokens, outputTokens int
		model := ""
		if lastResp != nil {
			content = lastResp.Content
			promptTokens = lastResp.PromptTokens
			outputTokens = lastResp.OutputTokens
			model = lastResp.Model
		}
		return map[string]any{
			"status":            "llm_partial",
			"model":             model,
			"content":           content,
			"prompt_tokens":     promptTokens,
			"output_tokens":     outputTokens,
			"tool_loop_partial": true,
			"edits_applied":     partial.EditsApplied,
			"files_read":        partial.FilesRead,
		}, nil
	}

	r.save(ctx, jobID, task.ID, stepID, "llm_response", parsed)

	content := ""
	var promptTokens, outputTokens int
	model := ""
	if lastResp != nil {
		content = lastResp.Content
		promptTokens = lastResp.PromptTokens
		outputTokens = lastResp.OutputTokens
		model = lastResp.Model
	}

	var filesRead []string
	if partial != nil {
		filesRead = partial.FilesRead
	}

	return map[string]any{
		"status":        "llm_completed",
		"model":         model,
		"content":       content,
		"parsed":        parsed,
		"prompt_tokens": promptTokens,
		"output_tokens": outputTokens,
		"files_read":    filesRead,
	}, nil
}

func (r Runner) validateSchema(stepID string, parsed map[string]any, agentic bool) error {
	if requiresPatch(stepID) {
		if agentic {
			// Edits are already applied to the workspace via tool calls; the LLM only needs
			// to confirm completion with a summary (Issue 1+2: drop the patch/diff contract).
			if summary, ok := parsed["summary"].(string); !ok || strings.TrimSpace(summary) == "" {
				return &ClassifiedParseError{
					Kind:    ParseSchemaError,
					Message: "missing required 'summary' field in JSON response",
				}
			}
			return nil
		}
		p := ""
		if pat, ok := parsed["patch"].(string); ok {
			p = pat
		} else if pat, ok := parsed["patch_text"].(string); ok {
			p = pat
		} else if pat, ok := parsed["diff"].(string); ok {
			p = pat
		}
		if p == "" {
			return &ClassifiedParseError{
				Kind:    ParseSchemaError,
				Message: "missing required 'patch', 'patch_text', or 'diff' field in JSON response",
			}
		}
	}
	if requiresStrictJSON(stepID) {
		if len(parsed) == 0 {
			return &ClassifiedParseError{
				Kind:    ParseSchemaError,
				Message: "parsed JSON response is empty",
			}
		}
	}
	return nil
}

func (r Runner) validateBusiness(stepID string, parsed map[string]any) error {
	if requiresPatch(stepID) {
		if fc, ok := parsed["files_changed"]; ok {
			switch val := fc.(type) {
			case []any:
				if len(val) == 0 {
					return &ClassifiedParseError{
						Kind:    ParseBusinessError,
						Message: "files_changed is empty, but patch changes are expected",
					}
				}
			case []string:
				if len(val) == 0 {
					return &ClassifiedParseError{
						Kind:    ParseBusinessError,
						Message: "files_changed is empty, but patch changes are expected",
					}
				}
			default:
				return &ClassifiedParseError{
					Kind:    ParseSchemaError,
					Message: "files_changed must be an array of strings",
				}
			}
		}
	}
	return nil
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

// isTransientError delegates to the single canonical classifier (REQ-M05) — do not
// reintroduce a separate copy of this logic here. Network errors (timeout, connection
// refused, EOF) are now classified identically at both this layer and the gateway layer,
// so the credential/model pair actually gets cooled down at the point of failure instead of
// only being retried by this outer, credential-unaware layer.
func isTransientError(err error) bool {
	return llm.IsTransientError(err)
}
