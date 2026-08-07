package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/auto-code-os/auto-code-os/server/internal/repository"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

// actionRolePolicy maps an action ID to the minimum role required to perform
// it. Actions absent from this map are allowed for any authenticated role
// (admin or member) — the codebase has no granular project.write/admin
// permission system (docs/openspecs/status-driven-agent-workspace/design.md
// Authorization section), so ActionPolicy is built on the existing two-role
// model rather than inventing new permission infrastructure.
var actionRolePolicy = map[string]string{
	"delete": models.UserRoleAdmin,
}

// requiredRole returns the minimum role for an action, or "" if any
// authenticated role may perform it.
func requiredRole(action string) string {
	return actionRolePolicy[action]
}

func roleSatisfies(callerRole, required string) bool {
	if required == "" {
		return true
	}
	return callerRole == required
}

// ApplyActionPolicy sets DisabledReason on any action the caller's role may
// not perform, rather than omitting it — the same actionRolePolicy table
// used by Dispatch's 403 enforcement, so GET (display) and POST
// (enforcement) never drift out of sync (design.md Authorization: "one
// policy table, two call sites").
func ApplyActionPolicy(actions []models.AvailableAction, callerRole string) []models.AvailableAction {
	for i := range actions {
		if required := requiredRole(actions[i].ID); !roleSatisfies(callerRole, required) {
			actions[i].DisabledReason = fmt.Sprintf("requires role %q", required)
		}
	}
	return actions
}

// TaskActionOrchestrator is the subset of *orchestrator.Orchestrator that
// task action dispatch needs. Declared locally to avoid importing
// internal/orchestrator into internal/service (internal/orchestrator already
// imports internal/service).
type TaskActionOrchestrator interface {
	Execute(ctx context.Context, taskID string) (*models.WorkflowJob, error)
	PauseJob(ctx context.Context, taskID string) error
	CancelJob(ctx context.Context, taskID string) error
	RetryFromLastStep(ctx context.Context, taskID string) (*models.WorkflowJob, error)
	RetryBlockedParent(ctx context.Context, parentID string) (*models.Task, error)
	CheckSpecReviewLoopLimit(ctx context.Context, taskID string) error
	SaveSpecReviewCycle(ctx context.Context, taskID, comment string) error
}

type TaskActionService struct {
	taskSvc     *TaskService
	requestRepo *repository.TaskActionRequestRepo
	orch        TaskActionOrchestrator
}

func NewTaskActionService(taskSvc *TaskService, requestRepo *repository.TaskActionRequestRepo, orch TaskActionOrchestrator) *TaskActionService {
	return &TaskActionService{taskSvc: taskSvc, requestRepo: requestRepo, orch: orch}
}

type TaskActionInput struct {
	Action    string `json:"action"`
	RequestID string `json:"request_id"`
	Comment   string `json:"comment"`
}

// Dispatch executes a task action idempotently and with authorization,
// re-validating against the task's *current* available actions (not the
// caller's possibly-stale view) — docs/openspecs/status-driven-agent-workspace
// specs.md's single-approval-gate/action-dispatch invariants.
func (s *TaskActionService) Dispatch(ctx context.Context, taskID string, callerRole string, input TaskActionInput) (*models.Task, error) {
	if input.Action == "" {
		return nil, ErrValidation("action is required")
	}
	if input.RequestID == "" {
		return nil, ErrValidation("request_id is required")
	}

	if existing, err := s.requestRepo.FindByRequestID(ctx, taskID, input.RequestID); err == nil {
		var task models.Task
		if err := json.Unmarshal(existing.Response, &task); err != nil {
			return nil, fmt.Errorf("unmarshal stored action response: %w", err)
		}
		return &task, nil
	} else if err != repository.ErrNotFound {
		return nil, fmt.Errorf("lookup task action request: %w", err)
	}

	if !roleSatisfies(callerRole, requiredRole(input.Action)) {
		return nil, ErrAuthorizationf(fmt.Sprintf("role %q may not perform action %q", callerRole, input.Action))
	}

	task, err := s.taskSvc.GetByID(ctx, taskID)
	if err != nil {
		return nil, err
	}
	available := false
	for _, a := range task.AvailableActions {
		if a.ID == input.Action {
			available = true
			break
		}
	}
	if !available {
		return nil, ErrConflictf(fmt.Sprintf("action %q is not available for task in status %q", input.Action, task.Status))
	}

	result, err := s.execute(ctx, task, input)
	if err != nil {
		return nil, err
	}

	responseBytes, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("marshal action response: %w", err)
	}
	if err := s.requestRepo.Create(ctx, &models.TaskActionRequest{
		TaskID:    taskID,
		RequestID: input.RequestID,
		Action:    input.Action,
		Response:  responseBytes,
	}); err != nil {
		return nil, fmt.Errorf("record task action request: %w", err)
	}

	return result, nil
}

func (s *TaskActionService) execute(ctx context.Context, task *models.Task, input TaskActionInput) (*models.Task, error) {
	taskID := task.ID
	switch input.Action {
	case "approve_spec":
		t, err := s.taskSvc.ApproveAnalysis(ctx, taskID)
		if err != nil {
			return nil, err
		}
		if s.orch != nil {
			if _, err := s.orch.Execute(ctx, taskID); err != nil {
				return nil, err
			}
		}
		return s.refresh(ctx, taskID, t)
	case "request_changes":
		if s.orch != nil {
			if err := s.orch.CheckSpecReviewLoopLimit(ctx, taskID); err != nil {
				return nil, ErrConflictf(err.Error())
			}
		}
		t, err := s.taskSvc.RequestAnalysisChanges(ctx, taskID, models.ClarifyTaskInput{Context: input.Comment})
		if err != nil {
			return nil, err
		}
		if s.orch != nil {
			if err := s.orch.SaveSpecReviewCycle(ctx, taskID, input.Comment); err != nil {
				return nil, err
			}
			if _, err := s.orch.Execute(ctx, taskID); err != nil {
				return nil, err
			}
		}
		return s.refresh(ctx, taskID, t)
	case "execute":
		if s.orch != nil {
			if _, err := s.orch.Execute(ctx, taskID); err != nil {
				return nil, err
			}
		}
		return s.taskSvc.GetByID(ctx, taskID)
	case "pause":
		if s.orch == nil {
			return nil, ErrConflictf("orchestrator not available")
		}
		if err := s.orch.PauseJob(ctx, taskID); err != nil {
			return nil, err
		}
		return s.taskSvc.GetByID(ctx, taskID)
	case "cancel":
		if s.orch == nil {
			return nil, ErrConflictf("orchestrator not available")
		}
		if err := s.orch.CancelJob(ctx, taskID); err != nil {
			return nil, err
		}
		return s.taskSvc.GetByID(ctx, taskID)
	case "retry":
		if s.orch == nil {
			return nil, ErrConflictf("orchestrator not available")
		}
		if _, err := s.orch.RetryFromLastStep(ctx, taskID); err != nil {
			return nil, err
		}
		return s.taskSvc.GetByID(ctx, taskID)
	case "retry_blocked":
		if s.orch == nil {
			return nil, ErrConflictf("orchestrator not available")
		}
		// A decomposed-parent block records BlockedChildID and resumes via
		// RetryBlockedParent (redispatches the failed child); a guardrail
		// block (retry_count/execution_time/event_volume/security_review,
		// tasks.md 2.3) has no BlockedChildID and instead resumes the task
		// itself from its last checkpoint.
		if task.BlockedChildID != nil {
			return s.orch.RetryBlockedParent(ctx, taskID)
		}
		if _, err := s.orch.RetryFromLastStep(ctx, taskID); err != nil {
			return nil, err
		}
		return s.taskSvc.GetByID(ctx, taskID)
	case "delete":
		if err := s.taskSvc.Delete(ctx, taskID); err != nil {
			return nil, err
		}
		return task, nil
	default:
		return nil, ErrValidation(fmt.Sprintf("unknown action %q", input.Action))
	}
}

func (s *TaskActionService) refresh(ctx context.Context, taskID string, fallback *models.Task) (*models.Task, error) {
	if t, err := s.taskSvc.GetByID(ctx, taskID); err == nil {
		return t, nil
	}
	return fallback, nil
}
