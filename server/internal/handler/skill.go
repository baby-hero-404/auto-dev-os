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

func (h *SkillHandler) Create(w http.ResponseWriter, r *http.Request) {
	var input models.CreateSkillInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	s, err := h.svc.Create(r.Context(), input)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, s)
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

func (h *SkillHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "skillID")
	var input models.UpdateSkillInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	s, err := h.svc.Update(r.Context(), id, input)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, s)
}

func (h *SkillHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "skillID")
	if err := h.svc.Delete(r.Context(), id); err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *SkillHandler) Seed(w http.ResponseWriter, r *http.Request) {
	skills, err := h.svc.SeedDefaultSkills(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, skills)
}
