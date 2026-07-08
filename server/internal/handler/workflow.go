package handler

import (
	"fmt"
	"net/http"

	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator"
	"github.com/go-chi/chi/v5"
)

type WorkflowHandler struct {
	orch *orchestrator.Orchestrator
}

func NewWorkflowHandler(orch *orchestrator.Orchestrator) *WorkflowHandler {
	return &WorkflowHandler{orch: orch}
}

func (h *WorkflowHandler) Execute(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskID")
	job, err := h.orch.Execute(r.Context(), taskID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, job)
}

func (h *WorkflowHandler) Status(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskID")
	status, err := h.orch.WorkflowStatus(r.Context(), taskID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (h *WorkflowHandler) Logs(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskID")
	logs, err := h.orch.Logs(r.Context(), taskID)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	if r.URL.Query().Get("stream") != "true" {
		writeJSON(w, http.StatusOK, logs)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	for _, log := range logs {
		_, _ = fmt.Fprintf(w, "event: log\ndata: %s\n\n", log.Message)
	}
}

func (h *WorkflowHandler) Approve(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskID")
	task, err := h.orch.ApproveMerge(r.Context(), taskID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, task)
}

// Retry re-enqueues a failed task, resuming from the last successful checkpoint.
// POST /api/v1/tasks/:taskID/retry
func (h *WorkflowHandler) Retry(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskID")
	job, err := h.orch.RetryFromLastStep(r.Context(), taskID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, job)
}

func (h *WorkflowHandler) Pause(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskID")
	if err := h.orch.PauseJob(r.Context(), taskID); err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, envelope{"status": "paused"})
}

func (h *WorkflowHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskID")
	if err := h.orch.CancelJob(r.Context(), taskID); err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, envelope{"status": "cancelled"})
}

func (h *WorkflowHandler) Artifacts(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobID")
	artifacts, err := h.orch.ListArtifacts(r.Context(), jobID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, artifacts)
}
