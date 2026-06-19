package handler

import (
	"net/http"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"github.com/go-chi/chi/v5"
)

type SkillHandler struct{ svc SkillService }

func NewSkillHandler(svc SkillService) *SkillHandler {
	return &SkillHandler{svc: svc}
}

func (h *SkillHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "skillID")
	s, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, s)
}

func (h *SkillHandler) List(w http.ResponseWriter, r *http.Request) {
	skills, err := h.svc.List(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, skills)
}

func (h *SkillHandler) Test(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "skillID")
	var input map[string]any
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	result, err := h.svc.Test(r.Context(), id, input)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *SkillHandler) ListFiles(w http.ResponseWriter, r *http.Request) {
	sourceID := chi.URLParam(r, "sourceID")
	path := r.URL.Query().Get("path")
	files, err := h.svc.ListFiles(r.Context(), sourceID, path)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, files)
}

func (h *SkillHandler) GetFileContent(w http.ResponseWriter, r *http.Request) {
	sourceID := chi.URLParam(r, "sourceID")
	path := r.URL.Query().Get("path")
	content, err := h.svc.GetFileContent(r.Context(), sourceID, path)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, content)
}

func (h *SkillHandler) Seed(w http.ResponseWriter, r *http.Request) {
	skills, err := h.svc.SeedDefaultSkills(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, skills)
}

func (h *SkillHandler) ListSources(w http.ResponseWriter, r *http.Request) {
	sources, err := h.svc.ListSources(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, sources)
}

func (h *SkillHandler) AddSource(w http.ResponseWriter, r *http.Request) {
	var input models.CreateSkillSourceInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	s, err := h.svc.AddSource(r.Context(), input)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, s)
}

func (h *SkillHandler) DeleteSource(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "sourceID")
	if err := h.svc.DeleteSource(r.Context(), id); err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *SkillHandler) SyncSource(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "sourceID")
	s, err := h.svc.SyncSource(r.Context(), id)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, s)
}
