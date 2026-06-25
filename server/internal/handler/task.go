package handler

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"github.com/go-chi/chi/v5"
)

type TaskHandler struct {
	svc  TaskService
	orch *orchestrator.Orchestrator
}

func NewTaskHandler(svc TaskService, orch *orchestrator.Orchestrator) *TaskHandler {
	return &TaskHandler{svc: svc, orch: orch}
}

func (h *TaskHandler) Analyze(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "taskID")
	t, err := h.svc.Analyze(r.Context(), id)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	if t.SpecStatus == models.TaskSpecStatusAutoApproved && h.orch != nil {
		if _, err := h.orch.Execute(r.Context(), id); err != nil {
			writeServiceError(w, err)
			return
		}
	}
	writeJSON(w, http.StatusOK, t)
}

func (h *TaskHandler) Clarify(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "taskID")
	var input models.ClarifyTaskInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	t, err := h.svc.Clarify(r.Context(), id, input)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (h *TaskHandler) GetAnalysis(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "taskID")
	analysis, err := h.svc.GetAnalysis(r.Context(), id)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, envelope{"analysis": json.RawMessage(analysis)})
}

func (h *TaskHandler) ApproveAnalysis(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "taskID")
	t, err := h.svc.ApproveAnalysis(r.Context(), id)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	if h.orch != nil {
		if _, err := h.orch.Execute(r.Context(), id); err != nil {
			writeServiceError(w, err)
			return
		}
	}
	writeJSON(w, http.StatusOK, t)
}

func (h *TaskHandler) RequestAnalysisChanges(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "taskID")
	var input models.ClarifyTaskInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	t, err := h.svc.RequestAnalysisChanges(r.Context(), id, input)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (h *TaskHandler) UpdateAnalysis(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "taskID")
	var raw json.RawMessage
	if err := decodeJSON(r, &raw); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	t, err := h.svc.UpdateAnalysis(r.Context(), id, raw)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (h *TaskHandler) ListSubTasks(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "taskID")
	tasks, err := h.svc.ListSubTasks(r.Context(), id)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, tasks)
}

func (h *TaskHandler) CreateSubTask(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "taskID")
	var input models.CreateTaskInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	t, err := h.svc.CreateSubTask(r.Context(), id, input)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	if h.orch != nil {
		if _, err := h.orch.Execute(r.Context(), t.ID); err != nil {
			writeServiceError(w, err)
			return
		}
	}
	writeJSON(w, http.StatusCreated, t)
}

func (h *TaskHandler) Create(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
	var input models.CreateTaskInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	t, err := h.svc.Create(r.Context(), projectID, input)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	if h.orch != nil {
		if _, err := h.orch.Execute(r.Context(), t.ID); err != nil {
			_ = h.svc.Delete(context.WithoutCancel(r.Context()), t.ID)
			writeServiceError(w, err)
			return
		}
		if updated, err := h.svc.GetByID(r.Context(), t.ID); err == nil {
			t = updated
		}
	}
	writeJSON(w, http.StatusCreated, t)
}

func (h *TaskHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "taskID")
	t, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (h *TaskHandler) List(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
	tasks, err := h.svc.ListByProjectID(r.Context(), projectID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, tasks)
}

func (h *TaskHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "taskID")
	var input models.UpdateTaskInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	t, err := h.svc.Update(r.Context(), id, input)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (h *TaskHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "taskID")
	if err := h.svc.Delete(r.Context(), id); err != nil {
		writeServiceError(w, err)
		return
	}
	if h.orch != nil {
		if err := h.orch.RemoveWorkspace(id); err != nil {
			log.Printf("warn: failed to delete workspace for task %s: %v", id, err)
		}
	}
	w.WriteHeader(http.StatusNoContent)
}
