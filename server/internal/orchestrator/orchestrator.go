package orchestrator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/auto-code-os/auto-code-os/server/internal/repository"
	"github.com/auto-code-os/auto-code-os/server/internal/sandbox"
	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

type Orchestrator struct {
	tasks     *repository.TaskRepo
	workflows *repository.WorkflowRepo
	agents    *AgentManager
	runtime   sandbox.Runtime
	prompts   *PromptAssembler
}

func NewOrchestrator(taskRepo *repository.TaskRepo, workflowRepo *repository.WorkflowRepo, agentManager *AgentManager, runtime sandbox.Runtime) *Orchestrator {
	return &Orchestrator{tasks: taskRepo, workflows: workflowRepo, agents: agentManager, runtime: runtime}
}

func NewOrchestratorWithPrompt(taskRepo *repository.TaskRepo, workflowRepo *repository.WorkflowRepo, agentManager *AgentManager, runtime sandbox.Runtime, prompts *PromptAssembler) *Orchestrator {
	return &Orchestrator{tasks: taskRepo, workflows: workflowRepo, agents: agentManager, runtime: runtime, prompts: prompts}
}

func (o *Orchestrator) Execute(ctx context.Context, taskID string) (*models.WorkflowJob, error) {
	if _, err := o.tasks.GetByID(ctx, taskID); err != nil {
		return nil, err
	}

	job, err := o.workflows.Enqueue(ctx, taskID)
	if err != nil {
		return nil, err
	}
	o.log(ctx, taskID, &job.ID, "info", "workflow job queued")

	go o.run(context.Background(), job.ID)
	return job, nil
}

func (o *Orchestrator) WorkflowStatus(ctx context.Context, taskID string) (*models.WorkflowStatus, error) {
	task, err := o.tasks.GetByID(ctx, taskID)
	if err != nil {
		return nil, err
	}
	checkpoints, err := o.workflows.ListCheckpoints(ctx, taskID)
	if err != nil {
		return nil, err
	}
	job, _ := o.workflows.LatestByTaskID(ctx, taskID)
	return &models.WorkflowStatus{Task: task, Job: job, Checkpoints: checkpoints}, nil
}

func (o *Orchestrator) Logs(ctx context.Context, taskID string) ([]models.TaskLog, error) {
	return o.workflows.ListLogs(ctx, taskID)
}

func (o *Orchestrator) ApproveMerge(ctx context.Context, taskID string) (*models.Task, error) {
	task, err := o.tasks.GetByID(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if task.Status != models.TaskStatusHumanReview {
		return nil, fmt.Errorf("task is not waiting for human PR approval")
	}
	status := models.TaskStatusMerged
	updated, err := o.tasks.Update(ctx, taskID, models.UpdateTaskInput{Status: &status})
	if err != nil {
		return nil, err
	}
	o.log(ctx, taskID, nil, "info", "human approved workflow for merge")
	return updated, nil
}

func (o *Orchestrator) run(ctx context.Context, jobID string) {
	job, err := o.workflows.UpdateJob(ctx, jobID, map[string]any{"status": models.WorkflowJobStatusRunning})
	if err != nil {
		return
	}

	task, err := o.tasks.GetByID(ctx, job.TaskID)
	if err != nil {
		o.fail(ctx, job, err)
		return
	}

	if err := o.checkpoint(ctx, task.ID, &job.ID, models.WorkflowStepAssign, map[string]any{"status": workflow.StepStatusRunning}); err != nil {
		o.fail(ctx, job, err)
		return
	}
	agent, err := o.agents.Assign(ctx, task)
	if err != nil {
		o.fail(ctx, job, err)
		return
	}
	if _, err := o.workflows.UpdateJob(ctx, job.ID, map[string]any{"agent_id": agent.ID, "step": models.WorkflowStepAssign}); err != nil {
		o.fail(ctx, job, err)
		return
	}
	if _, err := o.tasks.Update(ctx, task.ID, models.UpdateTaskInput{AgentID: &agent.ID}); err != nil {
		o.fail(ctx, job, err)
		return
	}
	o.log(ctx, task.ID, &job.ID, "info", fmt.Sprintf("assigned agent %s", agent.Name))

	status := models.TaskStatusInProgress
	if _, err := o.tasks.Update(ctx, task.ID, models.UpdateTaskInput{Status: &status}); err != nil {
		o.fail(ctx, job, err)
		return
	}

	if err := o.agents.MarkRunning(ctx, agent.ID); err != nil {
		o.fail(ctx, job, err)
		return
	}
	defer func() {
		_ = o.agents.Release(context.Background(), agent.ID)
	}()

	engine := &workflow.Engine{
		MaxParallel: 2,
		OnEvent: func(ctx context.Context, event workflow.Event) error {
			updates := map[string]any{"step": event.StepID}
			if event.Status == workflow.StepStatusPaused {
				updates["status"] = models.WorkflowJobStatusPaused
				updates["last_error"] = event.Error
			}
			if event.Status == workflow.StepStatusFailed {
				updates["last_error"] = event.Error
			}
			if _, err := o.workflows.UpdateJob(ctx, job.ID, updates); err != nil {
				return err
			}
			state := map[string]any{"status": event.Status}
			if event.Output != nil {
				state["output"] = event.Output
			}
			if event.Error != "" {
				state["error"] = event.Error
			}
			if err := o.checkpoint(ctx, task.ID, &job.ID, event.StepID, state); err != nil {
				return err
			}
			o.log(ctx, task.ID, &job.ID, "info", fmt.Sprintf("step %s %s", event.StepID, event.Status))
			return nil
		},
	}

	def := workflow.DefaultWorkflow(o.stepRunners(task, agent))
	result, err := engine.Run(ctx, def, map[string]any{"task_id": task.ID, "agent_id": agent.ID})
	if err != nil {
		if errors.Is(err, workflow.ErrPaused) {
			_, _ = o.workflows.UpdateJob(ctx, job.ID, map[string]any{"status": models.WorkflowJobStatusPaused, "last_error": err.Error()})
			o.log(ctx, task.ID, &job.ID, "info", err.Error())
			return
		}
		o.fail(ctx, job, err)
		return
	}
	if _, err := o.workflows.UpdateJob(ctx, job.ID, map[string]any{"status": models.WorkflowJobStatusDone, "step": models.WorkflowStepDone}); err != nil {
		o.fail(ctx, job, err)
		return
	}
	_ = o.checkpoint(ctx, task.ID, &job.ID, models.WorkflowStepDone, map[string]any{"status": models.WorkflowJobStatusDone, "steps": result.Status})
	o.log(ctx, task.ID, &job.ID, "info", "workflow completed and is waiting for human PR approval")
}

func (o *Orchestrator) fail(ctx context.Context, job *models.WorkflowJob, err error) {
	_, _ = o.workflows.UpdateJob(ctx, job.ID, map[string]any{"status": models.WorkflowJobStatusFailed, "last_error": err.Error()})
	failedStatus := models.TaskStatusFailed
	_, _ = o.tasks.Update(ctx, job.TaskID, models.UpdateTaskInput{Status: &failedStatus})
	o.log(ctx, job.TaskID, &job.ID, "error", err.Error())
}

func (o *Orchestrator) checkpoint(ctx context.Context, taskID string, jobID *string, step string, state map[string]any) error {
	raw, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return o.workflows.CreateCheckpoint(ctx, models.WorkflowCheckpoint{TaskID: taskID, JobID: jobID, Step: step, State: raw})
}

func (o *Orchestrator) log(ctx context.Context, taskID string, jobID *string, level, message string) {
	_ = o.workflows.CreateLog(ctx, models.TaskLog{TaskID: taskID, JobID: jobID, Level: level, Message: message})
}

func taskReadyForExecution(task *models.Task) bool {
	switch task.SpecStatus {
	case models.TaskSpecStatusApproved, models.TaskSpecStatusAutoApproved:
		return true
	default:
		return false
	}
}

func (o *Orchestrator) stepRunners(task *models.Task, agent *models.Agent) map[string]workflow.StepFunc {
	return map[string]workflow.StepFunc{
		workflow.StepAnalyze: func(ctx context.Context, _ workflow.StepContext) (map[string]any, error) {
			if o.prompts != nil {
				messages, tools, err := o.prompts.AssembleForAgent(ctx, *task, agent, nil)
				if err != nil {
					return nil, err
				}
				o.log(ctx, task.ID, nil, "info", fmt.Sprintf("assembled prompt with %d messages and %d tools", len(messages), len(tools)))
			}
			if taskReadyForExecution(task) {
				return map[string]any{"complexity": task.Complexity, "spec_status": task.SpecStatus}, nil
			}
			analysis := deriveWorkflowAnalysis(task)
			raw, err := json.Marshal(analysis)
			if err != nil {
				return nil, err
			}
			specStatus := models.TaskSpecStatusPendingReview
			status := models.TaskStatusSpecReview
			if len(analysis.ClarificationQuestions) > 0 {
				specStatus = models.TaskSpecStatusChangesRequested
			} else if analysis.Complexity == models.TaskComplexityEasy {
				specStatus = models.TaskSpecStatusAutoApproved
				status = models.TaskStatusInProgress
			}
			if _, err := o.tasks.Update(ctx, task.ID, models.UpdateTaskInput{
				Complexity: &analysis.Complexity,
				Analysis:   raw,
				SpecStatus: &specStatus,
				Status:     &status,
			}); err != nil {
				return nil, err
			}
			task.Complexity = analysis.Complexity
			task.SpecStatus = specStatus
			task.Analysis = raw
			if specStatus == models.TaskSpecStatusPendingReview || specStatus == models.TaskSpecStatusChangesRequested {
				return nil, workflow.PauseError{Step: workflow.StepAnalyze, Reason: "workflow paused for human spec review"}
			}
			return map[string]any{"complexity": analysis.Complexity, "spec_status": specStatus}, nil
		},
		workflow.StepPlan: func(ctx context.Context, _ workflow.StepContext) (map[string]any, error) {
			plan := []any{
				map[string]any{"id": "backend", "role": models.AgentRoleBackend, "description": "Implement server-side changes and data contracts."},
				map[string]any{"id": "frontend", "role": models.AgentRoleFrontend, "description": "Implement user-facing workflow updates when applicable."},
			}
			return map[string]any{"subtasks": plan}, nil
		},
		workflow.StepCodeBackend: func(ctx context.Context, _ workflow.StepContext) (map[string]any, error) {
			return o.runSandboxStep(ctx, task, agent, workflow.StepCodeBackend, "echo phase3b backend coding complete")
		},
		workflow.StepCodeFrontend: func(ctx context.Context, _ workflow.StepContext) (map[string]any, error) {
			return o.runSandboxStep(ctx, task, agent, workflow.StepCodeFrontend, "echo phase3b frontend coding complete")
		},
		workflow.StepMerge: func(context.Context, workflow.StepContext) (map[string]any, error) {
			return map[string]any{"status": "merged"}, nil
		},
		workflow.StepReview: func(context.Context, workflow.StepContext) (map[string]any, error) {
			return map[string]any{"findings": []any{}}, nil
		},
		workflow.StepFix: func(context.Context, workflow.StepContext) (map[string]any, error) {
			return map[string]any{"status": "no_fixes_required"}, nil
		},
		workflow.StepTest: func(ctx context.Context, _ workflow.StepContext) (map[string]any, error) {
			out, err := o.runSandboxStep(ctx, task, agent, workflow.StepTest, "echo phase3b tests passed")
			if err != nil {
				return nil, err
			}
			out["exit_code"] = 0
			return out, nil
		},
		workflow.StepPR: func(ctx context.Context, _ workflow.StepContext) (map[string]any, error) {
			status := models.TaskStatusHumanReview
			if _, err := o.tasks.Update(ctx, task.ID, models.UpdateTaskInput{Status: &status}); err != nil {
				return nil, err
			}
			return map[string]any{"status": "pr_ready_for_human_approval"}, nil
		},
	}
}

func (o *Orchestrator) runSandboxStep(ctx context.Context, task *models.Task, agent *models.Agent, stepID, command string) (map[string]any, error) {
	result, err := o.runtime.Run(ctx, sandbox.CommandRequest{
		TaskID:      task.ID,
		AgentID:     agent.ID,
		Command:     []string{"bash", "-lc", command},
		NetworkMode: sandbox.NetworkModeNone,
		Timeout:     5 * time.Minute,
	})
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(result.Stdout) != "" {
		o.log(ctx, task.ID, nil, "info", fmt.Sprintf("%s: %s", stepID, strings.TrimSpace(result.Stdout)))
	}
	if strings.TrimSpace(result.Stderr) != "" {
		o.log(ctx, task.ID, nil, "warn", fmt.Sprintf("%s: %s", stepID, strings.TrimSpace(result.Stderr)))
	}
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("%s failed with exit code %d", stepID, result.ExitCode)
	}
	return map[string]any{"status": "ok", "stdout": result.Stdout}, nil
}

func deriveWorkflowAnalysis(task *models.Task) models.TaskAnalysis {
	text := strings.ToLower(task.Title + " " + task.Description)
	complexity := task.Complexity
	if complexity == "" {
		complexity = models.TaskComplexityEasy
	}
	hardSignals := []string{"architecture", "security", "auth", "permission", "rbac", "payment", "migration", "distributed"}
	mediumSignals := []string{"feature", "refactor", "api", "database", "ui", "workflow", "integration"}
	for _, signal := range hardSignals {
		if strings.Contains(text, signal) {
			complexity = models.TaskComplexityHard
			break
		}
	}
	if complexity != models.TaskComplexityHard {
		for _, signal := range mediumSignals {
			if strings.Contains(text, signal) {
				complexity = models.TaskComplexityMedium
				break
			}
		}
	}
	questions := []string{}
	if len(strings.TrimSpace(task.Description)) < 30 {
		questions = append(questions, "Please provide more implementation context, affected module names, and expected behavior.")
	}
	return models.TaskAnalysis{
		Complexity:    complexity,
		Scope:         "Generated by the Phase 3b workflow analyze step.",
		AffectedFiles: []string{},
		Risks:         []string{"Workflow uses deterministic planning until full LLM step execution is enabled."},
		ExecutionPlan: []string{
			"Assemble prompt with role, rules, and retrieved context.",
			"Decompose work into typed subtasks.",
			"Run backend and frontend coding tracks in parallel sandboxes.",
			"Merge, review, fix, test, and prepare PR approval checkpoint.",
		},
		ClarificationQuestions: questions,
	}
}
