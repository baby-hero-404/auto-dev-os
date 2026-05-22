package handler

import (
	"net/http"

	"github.com/auto-code-os/auto-code-os/server/internal/service"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"github.com/go-chi/chi/v5"
)

type SkillHandler struct{ svc *service.SkillService }

func NewSkillHandler(svc *service.SkillService) *SkillHandler {
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
		if isValidationErr(err) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, s)
}

func (h *SkillHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "skillID")
	s, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "skill not found")
		return
	}
	writeJSON(w, http.StatusOK, s)
}

func (h *SkillHandler) List(w http.ResponseWriter, r *http.Request) {
	skills, err := h.svc.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, skills)
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
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, s)
}

func (h *SkillHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "skillID")
	if err := h.svc.Delete(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
