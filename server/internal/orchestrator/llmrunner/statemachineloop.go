package llmrunner

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/auto-code-os/auto-code-os/server/pkg/llm"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

// resolveExecutionIRForStep locates the ExecutionIR for the current step based on the agent's role.
func resolveExecutionIRForStep(analysis *models.TaskAnalysis, stepID string) models.ExecutionIR {
	role := agenticWorkspaceRole(stepID)
	for _, ir := range analysis.ExecutionIRs {
		for _, unit := range analysis.ExecutionUnits {
			if unit.ID == ir.NodeID {
				if strings.ToLower(unit.ExecutionProfile.Agent) == role {
					return ir
				}
			}
		}
	}
	if len(analysis.ExecutionIRs) == 1 {
		return analysis.ExecutionIRs[0]
	}
	// Fallback budget defaults. Schema-valid (not just a budget stand-in) so it can still be
	// passed to PromptCompiler.Compile — ValidateExecutionIR requires schema_version,
	// intent.capability, intent.operation, and non-nil constraints/acceptance.
	return models.ExecutionIR{
		SchemaVersion: models.CurrentExecutionIRSchemaVersion,
		NodeID:        "default",
		Intent:        models.Intent{Capability: stepID, Operation: "modify"},
		Constraints:   []string{},
		Acceptance:    []string{},
		Budget: models.PhaseBudgets{
			Discovery:      6,
			Implementation: 12,
			Validation:     3,
		},
	}
}

func agenticWorkspaceRole(stepID string) string {
	if strings.Contains(stepID, "code_backend") {
		return "backend"
	}
	if strings.Contains(stepID, "code_frontend") {
		return "frontend"
	}
	return ""
}

func pathMatchesTarget(targetPath string, resolvedTargets []string) bool {
	for _, rt := range resolvedTargets {
		if strings.HasSuffix(targetPath, rt) || strings.HasSuffix(rt, targetPath) {
			return true
		}
	}
	return false
}

// runStateMachine drives the step execution using the deterministic Node State Machine.
func (r Runner) runStateMachine(ctx context.Context, task *models.Task, agent *models.Agent, jobID, stepID string, messages []llm.Message) (map[string]any, error) {
	var analysis models.TaskAnalysis
	if len(task.Analysis) > 0 {
		_ = json.Unmarshal(task.Analysis, &analysis)
	}

	ir := resolveExecutionIRForStep(&analysis, stepID)
	sm := NewStateMachine(ir.Budget)
	r.log(ctx, task.ID, nil, "info", fmt.Sprintf("StateMachine initialized for node %s with budget: Discovery=%d, Implementation=%d, Validation=%d", ir.NodeID, ir.Budget.Discovery, ir.Budget.Implementation, ir.Budget.Validation))

	resolvedTargets := analysis.ExecutionIRTargets[ir.NodeID]

	// Compile the node's Execution Contract (constraints, acceptance criteria, physical write
	// scope, budgets) once and prepend it, so the model actually sees the IR's constraints and
	// acceptance criteria — previously nothing in this loop surfaced them (design.md: State
	// Machine -> Prompt Compiler -> LLM Node Executor).
	if r.Compiler != nil {
		if contractMsgs, err := r.Compiler.Compile(ir, resolvedTargets); err != nil {
			r.log(ctx, task.ID, nil, "warn", fmt.Sprintf("PromptCompiler failed for node %s, continuing without execution contract: %v", ir.NodeID, err))
		} else {
			messages = append(messages, contractMsgs...)
		}
	}

	// initialMessages is captured before the loop mutates messages, so the snapshot hash
	// reflects the same prompt construction that a resume can independently reconstruct
	// (see saveExecutionSnapshot and Runner.BuildInitialMessages).
	initialMessages := append([]llm.Message{}, messages...)

	var lastResp *llm.Response
	var lastParsed map[string]any
	var editsApplied []string
	var filesRead []string
	var toolHistory []models.ToolCallRecord

	failureCounts := make(map[string]int)
	readMemo := make(map[string]int)

	// lastActiveState tracks the most recent non-terminal state so a terminal snapshot can
	// record how many iterations of that phase actually ran (sm.used has no entry for
	// terminal states themselves).
	var lastActiveState NodeState

	// Start in DISCOVERY
	for {
		currentState := sm.Current()
		if currentState.Terminal() {
			break
		}
		lastActiveState = currentState

		if currentState == StatePlanReady {
			// Deterministic ResolvePlan gate (REQ-004). If the step reached here, targets were verified or warn-resolved.
			newState, err := sm.ResolvePlan(true)
			if err != nil {
				return nil, err
			}
			r.log(ctx, task.ID, nil, "info", fmt.Sprintf("StateMachine deterministic transition: StatePlanReady -> %s", newState))
			continue
		}

		// Restrict tools based on the current state's allowlist (REQ-M01)
		var allowedTools []llm.ToolDefinition
		for _, tool := range r.Tools {
			if sm.ToolAllowed(tool.Name) {
				allowedTools = append(allowedTools, tool)
			}
		}

		// Prepare prompt instructions for the current state/phase
		phasePrompt := fmt.Sprintf("\n\n[STATE MACHINE] Current Phase: %s. Remaining budget: %d iterations. Allowed tools: %s.", currentState, sm.Remaining(), getToolNames(allowedTools))
		if currentState == StateDiscovery {
			phasePrompt += "\nExplore the codebase to discover files needing change. Do NOT edit any files yet."
		} else if currentState == StateImplementation {
			phasePrompt += fmt.Sprintf("\nApply your edits directly using write tools. You are strictly allowed to modify only these files: %s.", strings.Join(resolvedTargets, ", "))
		} else if currentState == StateValidation {
			phasePrompt += "\nVerify your edits using the test/lint tools. Fix any issues found."
		}

		// Append the phase instructions to the user prompt history
		msgs := append([]llm.Message{}, messages...)
		msgs = append(msgs, llm.Message{Role: "user", Content: phasePrompt})

		// LLM call for the turn
		callStart := time.Now()
		resp, err := r.Provider.ChatWithOptions(ctx, msgs, llm.ChatOptions{Tools: allowedTools, ToolChoice: "auto"})
		latency := time.Since(callStart)
		if err != nil {
			return nil, fmt.Errorf("llm state machine loop call failed in state %s: %w", currentState, err)
		}
		lastResp = resp

		if r.WriteTrace != nil {
			var tracedParsed map[string]any
			if len(resp.ToolCalls) > 0 {
				tracedParsed = map[string]any{"tool_calls": resp.ToolCalls}
			} else {
				tracedParsed = map[string]any{"raw_content": resp.Content}
			}
			r.WriteTrace(ctx, task, agent, stepID, msgs, resp, tracedParsed, sm.used[currentState]+1, latency)
		}

		if len(resp.ToolCalls) > 0 {
			// Process tool calls
			messages = append(messages, llm.Message{Role: "assistant", Content: resp.Content, ToolCalls: resp.ToolCalls})
			var editAppliedThisTurn bool

			for _, call := range resp.ToolCalls {
				discriminator := extractCallKey(call.Arguments)
				key := call.Name + ":" + discriminator

				// 1. Tool Allowlist Check (REQ-M01)
				if errCheck := sm.CheckTool(call.Name); errCheck != nil {
					r.log(ctx, task.ID, nil, "warn", fmt.Sprintf("StateMachine blocked forbidden tool %s in state %s", call.Name, currentState))
					messages = append(messages, llm.Message{Role: "tool", ToolCallID: call.ID, ToolName: call.Name, Content: "Error: " + errCheck.Error()})
					continue
				}

				// 2. Write-Scope Validation (REQ-002 / REQ-M01)
				if editToolNames[call.Name] && len(resolvedTargets) > 0 {
					if !pathMatchesTarget(discriminator, resolvedTargets) {
						errMsg := fmt.Sprintf("Error: You are trying to modify %s, but your execution boundary and resolved intents only allow modifications to the following files: %s", discriminator, strings.Join(resolvedTargets, ", "))
						r.log(ctx, task.ID, nil, "warn", fmt.Sprintf("StateMachine blocked out-of-scope write to %q", discriminator))
						messages = append(messages, llm.Message{Role: "tool", ToolCallID: call.ID, ToolName: call.Name, Content: errMsg})
						continue
					}
				}

				// Read Memoization check
				var readMemoKey string
				if call.Name == "read_file" {
					readMemoKey = readFileMemoKey(call.Arguments)
					if readMemoKey != "" {
						if turn, seen := readMemo[readMemoKey]; seen {
							result := fmt.Sprintf("Already read at turn %d for this path/range — reusing that content. Refer back to your earlier read_file result.", turn)
							messages = append(messages, llm.Message{Role: "tool", ToolCallID: call.ID, ToolName: call.Name, Content: result})
							continue
						}
					}
				}

				if failureCounts[key] >= 2 {
					result := fmt.Sprintf("Error: You have called %s on %q multiple times without success. Stop repeating this exact call.", call.Name, discriminator)
					messages = append(messages, llm.Message{Role: "tool", ToolCallID: call.ID, ToolName: call.Name, Content: result})
					continue
				}

				// Execute tool
				result, toolErr := r.ToolExecutor(ctx, call.Name, call.Arguments)
				if toolErr != nil {
					rawArgs := json.RawMessage(call.Arguments)
					toolHistory = append(toolHistory, models.ToolCallRecord{
						State:     string(currentState),
						Iteration: sm.used[currentState] + 1,
						Tool:      call.Name,
						Args:      rawArgs,
						Error:     toolErr.Error(),
					})
					return nil, toolErr
				}

				if strings.HasPrefix(result, "Error:") {
					failureCounts[key]++
				} else {
					failureCounts[key] = 0
					if editToolNames[call.Name] && discriminator != "" {
						editsApplied = append(editsApplied, discriminator)
						editAppliedThisTurn = true
					}
					if call.Name == "read_file" && discriminator != "" {
						filesRead = append(filesRead, discriminator)
					}
					if readMemoKey != "" {
						readMemo[readMemoKey] = sm.used[currentState] + 1
					}
				}
				result = r.truncateToolResult(result)
				rawArgs := json.RawMessage(call.Arguments)
				var errStr string
				if strings.HasPrefix(result, "Error:") {
					errStr = result
				}
				toolHistory = append(toolHistory, models.ToolCallRecord{
					State:     string(currentState),
					Iteration: sm.used[currentState] + 1,
					Tool:      call.Name,
					Args:      rawArgs,
					Result:    result,
					Error:     errStr,
				})
				messages = append(messages, llm.Message{Role: "tool", ToolCallID: call.ID, ToolName: call.Name, Content: result})
			}

			// Advise transitions for tool-using iteration
			if currentState == StateDiscovery {
				_, _ = sm.AdviseDiscovery(false)
			} else if currentState == StateImplementation {
				_, _ = sm.AdviseImplementation(false, editAppliedThisTurn)
			} else if currentState == StateValidation {
				// Advise validation failure if validation tools returned failures.
				// If they passed, we wait until the LLM returns its final non-tool JSON response to call AdviseValidation(true).
				checksPassed := true
				for _, msg := range messages[len(messages)-len(resp.ToolCalls):] {
					if strings.HasPrefix(msg.Content, "Error:") {
						checksPassed = false
						break
					}
				}
				if !checksPassed {
					_, _ = sm.AdviseValidation(false)
				}
			}
			continue
		}

		// No tool calls: LLM returned final text/JSON response
		parsedJSON, parseErr := ParseJSONMarkdown(resp.Content)
		if parseErr != nil {
			messages = append(messages, llm.Message{Role: "assistant", Content: resp.Content})
			messages = append(messages, llm.Message{
				Role:    "user",
				Content: fmt.Sprintf("Your output was not valid JSON. Error: %v. Please output strictly valid JSON matching the schema.", parseErr),
			})
			if currentState == StateDiscovery {
				_, _ = sm.AdviseDiscovery(false)
			} else if currentState == StateImplementation {
				_, _ = sm.AdviseImplementation(false, false)
			} else if currentState == StateValidation {
				_, _ = sm.AdviseValidation(false)
			}
			continue
		}

		// Perform business/schema validation on non-tool JSON responses
		if schemaErr := r.validateSchema(stepID, parsedJSON, true); schemaErr != nil {
			parseErr = schemaErr
		} else if bizErr := r.validateBusiness(stepID, parsedJSON); bizErr != nil {
			parseErr = bizErr
		}

		if parseErr != nil {
			messages = append(messages, llm.Message{Role: "assistant", Content: resp.Content})
			messages = append(messages, llm.Message{Role: "user", Content: "Validation failed: " + parseErr.Error()})
			if currentState == StateDiscovery {
				_, _ = sm.AdviseDiscovery(false)
			} else if currentState == StateImplementation {
				_, _ = sm.AdviseImplementation(false, false)
			} else if currentState == StateValidation {
				_, _ = sm.AdviseValidation(false)
			}
			continue
		}

		lastParsed = parsedJSON

		// Non-tool response triggers phase transition
		if currentState == StateDiscovery {
			// Stopped calling tools and returned JSON; discovery complete
			_, _ = sm.AdviseDiscovery(true)
		} else if currentState == StateImplementation {
			// Implementation completed edits
			_, _ = sm.AdviseImplementation(true, false)
		} else if currentState == StateValidation {
			// Validation complete and passed
			_, _ = sm.AdviseValidation(true)
		}
	}

	finalState := sm.Current()
	r.log(ctx, task.ID, nil, "info", fmt.Sprintf("StateMachine execution ended in terminal state: %s", finalState))

	if finalState == StateDone {
		r.saveExecutionSnapshot(ctx, task, agent, jobID, stepID, initialMessages, sm, toolHistory, finalState, lastActiveState)
		r.save(ctx, jobID, task.ID, stepID, "llm_response", lastParsed)
		return map[string]any{
			"status":        "llm_completed",
			"model":         lastResp.Model,
			"content":       lastResp.Content,
			"parsed":        lastParsed,
			"prompt_tokens": lastResp.PromptTokens,
			"output_tokens": lastResp.OutputTokens,
			"files_read":    filesRead,
		}, nil
	}

	if finalState == StateSalvaged {
		r.saveExecutionSnapshot(ctx, task, agent, jobID, stepID, initialMessages, sm, toolHistory, finalState, lastActiveState)
		r.log(ctx, task.ID, nil, "warn", fmt.Sprintf("%s: StateMachine implementation loop exhausted budget but %d edit(s) were applied; surfacing partial result", stepID, len(editsApplied)))
		return map[string]any{
			"status":            "llm_partial",
			"model":             lastResp.Model,
			"content":           lastResp.Content,
			"prompt_tokens":     lastResp.PromptTokens,
			"output_tokens":     lastResp.OutputTokens,
			"tool_loop_partial": true,
			"edits_applied":     editsApplied,
			"files_read":        filesRead,
		}, nil
	}

	// StateFailed
	return nil, fmt.Errorf("state machine execution failed (terminal state FAILED)")
}

func getToolNames(tools []llm.ToolDefinition) string {
	names := make([]string, 0, len(tools))
	for _, t := range tools {
		names = append(names, t.Name)
	}
	return "[" + strings.Join(names, ", ") + "]"
}

// updateShadowSM advances the shadow StateMachine for parallel telemetry analysis.
func (r Runner) updateShadowSM(ctx context.Context, shadowSM *StateMachine, resp *llm.Response, resolvedTargets []string, taskID string) {
	if shadowSM == nil {
		return
	}
	currentState := shadowSM.Current()
	if currentState.Terminal() {
		return
	}

	hasWriteTool := false
	hasValidationTool := false
	for _, call := range resp.ToolCalls {
		if editToolNames[call.Name] {
			hasWriteTool = true
		} else if call.Name == "run_tests" || call.Name == "run_lint" || call.Name == "run_build" {
			hasValidationTool = true
		}
	}

	if currentState == StateDiscovery {
		if hasWriteTool {
			r.log(ctx, taskID, nil, "warn", "[TELEMETRY-VIOLATION] Shadow state machine: write tool call detected during StateDiscovery, advancing to StateImplementation")
			_, _ = shadowSM.AdviseDiscovery(true)
			_, _ = shadowSM.ResolvePlan(true)
		} else {
			_, _ = shadowSM.AdviseDiscovery(len(resp.ToolCalls) == 0)
		}
	} else if currentState == StateImplementation {
		if hasValidationTool || (len(resp.ToolCalls) == 0 && resp.Content != "") {
			_, _ = shadowSM.AdviseImplementation(true, false)
		} else {
			_, _ = shadowSM.AdviseImplementation(false, hasWriteTool)
		}
	} else if currentState == StateValidation {
		if len(resp.ToolCalls) == 0 {
			_, _ = shadowSM.AdviseValidation(true)
		} else {
			_, _ = shadowSM.AdviseValidation(false)
		}
	}
}

func (r Runner) truncateToolResult(s string) string {
	maxChars := r.MaxToolResultChars
	if maxChars <= 0 {
		maxChars = 8000
	}
	if len(s) <= maxChars {
		return s
	}
	return s[:maxChars] + fmt.Sprintf("\n... [truncated %d chars]", len(s)-maxChars)
}

func (r Runner) saveExecutionSnapshot(ctx context.Context, task *models.Task, agent *models.Agent, jobID, stepID string, initialMessages []llm.Message, sm *StateMachine, toolHistory []models.ToolCallRecord, finalState, lastActiveState NodeState) {
	if r.CaptureDiff == nil || r.SaveArtifact == nil {
		return
	}

	role := agenticWorkspaceRole(stepID)
	diffText, err := r.CaptureDiff(ctx, task, agent, role)
	if err != nil {
		r.log(ctx, task.ID, nil, "error", fmt.Sprintf("failed to capture workspace diff for snapshot: %v", err))
		return
	}

	// Hash the initial prompt (not the accumulated tool-call transcript) so a resume can
	// independently reconstruct a comparable hash via Runner.BuildInitialMessages without
	// replaying the LLM conversation.
	rawMsgs, _ := json.Marshal(initialMessages)
	h := sha256.Sum256(rawMsgs)
	promptHash := hex.EncodeToString(h[:])

	snapshot := models.ExecutionSnapshot{
		ExecutionID:   stepID,
		CurrentState:  string(finalState),
		Iteration:     sm.used[lastActiveState],
		WorkspaceDiff: diffText,
		ToolHistory:   toolHistory,
		PromptHash:    promptHash,
		Timestamp:     time.Now(),
	}

	saveErr := r.SaveArtifact(ctx, jobID, task.ID, stepID, "execution_snapshot", snapshot)
	if saveErr != nil {
		r.log(ctx, task.ID, nil, "error", fmt.Sprintf("failed to save execution snapshot: %v", saveErr))
	} else {
		r.log(ctx, task.ID, nil, "info", fmt.Sprintf("ExecutionSnapshot successfully saved with prompt hash: %s", promptHash))
	}
}
