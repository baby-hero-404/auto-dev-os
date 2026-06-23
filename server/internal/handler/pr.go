package handler

import (
	"net/http"

	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator"
	"github.com/auto-code-os/auto-code-os/server/internal/service"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"github.com/go-chi/chi/v5"
)

// PRHandler handles PR approval and rejection endpoints.
type PRHandler struct {
	taskSvc  TaskService
	auditSvc AuditService
	orch     *orchestrator.Orchestrator
}

func NewPRHandler(taskSvc TaskService, auditSvc AuditService, orch *orchestrator.Orchestrator) *PRHandler {
	return &PRHandler{taskSvc: taskSvc, auditSvc: auditSvc, orch: orch}
}

// Approve approves a PR and transitions the task to merged status.
// POST /api/v1/tasks/:taskID/pr/approve
func (h *PRHandler) Approve(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskID")

	task, err := h.taskSvc.GetByID(r.Context(), taskID)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	// Validate task is in a reviewable state.
	if task.Status != models.TaskStatusHumanReview && task.Status != models.TaskStatusPrReady {
		writeError(w, http.StatusBadRequest, "task is not awaiting PR review (status: "+task.Status+")")
		return
	}

	// Transition to merged and execute merge in GitHub
	var updated *models.Task
	if h.orch != nil {
		updated, err = h.orch.ApproveMerge(r.Context(), taskID)
		if err != nil {
			writeServiceError(w, err)
			return
		}
	} else {
		merged := models.TaskStatusMerged
		updated, err = h.taskSvc.Update(r.Context(), taskID, models.UpdateTaskInput{Status: &merged})
		if err != nil {
			writeServiceError(w, err)
			return
		}
	}

	// Record audit log.
	h.auditSvc.RecordAction(r.Context(), models.AuditActionPRApproved, "task", taskID,
		service.WithTaskID(taskID),
	)

	writeJSON(w, http.StatusOK, updated)
}

// Reject rejects a PR with feedback and triggers a fix cycle.
// POST /api/v1/tasks/:taskID/pr/reject
func (h *PRHandler) Reject(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskID")

	task, err := h.taskSvc.GetByID(r.Context(), taskID)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	// Validate task is in a reviewable state.
	if task.Status != models.TaskStatusHumanReview && task.Status != models.TaskStatusPrReady {
		writeError(w, http.StatusBadRequest, "task is not awaiting PR review (status: "+task.Status+")")
		return
	}

	var input models.PRRejectInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if input.Feedback == "" {
		writeError(w, http.StatusBadRequest, "feedback is required when rejecting a PR")
		return
	}

	if h.orch != nil {
		if err := h.orch.CheckReviewLoopLimit(r.Context(), taskID); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	// Transition to fixing state to trigger the fix cycle.
	fixing := models.TaskStatusFixing
	updated, err := h.taskSvc.Update(r.Context(), taskID, models.UpdateTaskInput{Status: &fixing})
	if err != nil {
		writeServiceError(w, err)
		return
	}

	// Record audit log with rejection feedback.
	h.auditSvc.RecordAction(r.Context(), models.AuditActionPRRejected, "task", taskID,
		service.WithTaskID(taskID),
		service.WithDetails(map[string]string{"feedback": input.Feedback}),
	)

	if h.orch != nil {
		if err := h.orch.SavePRRejectionFeedback(r.Context(), taskID, input.Feedback); err != nil {
			writeServiceError(w, err)
			return
		}
		if err := h.orch.ClearCheckpointsForRepair(r.Context(), taskID); err != nil {
			writeServiceError(w, err)
			return
		}
		if _, err := h.orch.Execute(r.Context(), taskID); err != nil {
			writeServiceError(w, err)
			return
		}
	}

	writeJSON(w, http.StatusOK, updated)
}

// StartReview transitions a task from pr_ready to human_review.
// POST /api/v1/tasks/:taskID/pr/start-review
func (h *PRHandler) StartReview(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskID")

	task, err := h.taskSvc.GetByID(r.Context(), taskID)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	if task.Status != models.TaskStatusPrReady {
		writeError(w, http.StatusBadRequest, "task is not in pr_ready state (status: "+task.Status+")")
		return
	}

	var updated *models.Task
	if h.orch != nil {
		updated, err = h.orch.StartReview(r.Context(), taskID)
		if err != nil {
			writeServiceError(w, err)
			return
		}
	} else {
		hr := models.TaskStatusHumanReview
		updated, err = h.taskSvc.Update(r.Context(), taskID, models.UpdateTaskInput{Status: &hr})
		if err != nil {
			writeServiceError(w, err)
			return
		}
	}

	h.auditSvc.RecordAction(r.Context(), models.AuditActionPRReviewStarted, "task", taskID,
		service.WithTaskID(taskID),
	)

	writeJSON(w, http.StatusOK, updated)
}
