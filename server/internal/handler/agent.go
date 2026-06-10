package handler

import (
	"net/http"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"github.com/go-chi/chi/v5"
)

type AgentHandler struct{ svc AgentService }

func NewAgentHandler(svc AgentService) *AgentHandler {
	return &AgentHandler{svc: svc}
}

func (h *AgentHandler) Create(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
	var input models.CreateAgentInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	a, err := h.svc.AssignToProject(r.Context(), projectID, input)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, a)
}

func (h *AgentHandler) Hire(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgID")
	var input models.CreateAgentInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	a, err := h.svc.Hire(r.Context(), orgID, input)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, a)
}

func (h *AgentHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "agentID")
	a, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, a)
}

func (h *AgentHandler) List(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
	agents, err := h.svc.ListByProjectID(r.Context(), projectID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, agents)
}

func (h *AgentHandler) ListOrg(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgID")
	agents, err := h.svc.ListByOrgID(r.Context(), orgID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, agents)
}

func (h *AgentHandler) ListRoleTemplates(w http.ResponseWriter, r *http.Request) {
	templates, err := h.svc.ListRoleTemplates(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, templates)
}

func (h *AgentHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "agentID")
	var input models.UpdateAgentInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	a, err := h.svc.Update(r.Context(), id, input)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, a)
}

func (h *AgentHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "agentID")
	if err := h.svc.Delete(r.Context(), id); err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
