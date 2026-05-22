package handler

import (
	"net/http"

	"github.com/auto-code-os/auto-code-os/server/internal/service"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"github.com/go-chi/chi/v5"
)

type RuleHandler struct{ svc *service.RuleService }

func NewRuleHandler(svc *service.RuleService) *RuleHandler {
	return &RuleHandler{svc: svc}
}

func (h *RuleHandler) Create(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
	var input models.CreateRuleInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	var pid *string
	if projectID != "" {
		pid = &projectID
	}
	rule, err := h.svc.Create(r.Context(), pid, input)
	if err != nil {
		if isValidationErr(err) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, rule)
}

func (h *RuleHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "ruleID")
	rule, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "rule not found")
		return
	}
	writeJSON(w, http.StatusOK, rule)
}

func (h *RuleHandler) List(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
	rules, err := h.svc.ListByProjectID(r.Context(), projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, rules)
}

func (h *RuleHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "ruleID")
	var input models.UpdateRuleInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	rule, err := h.svc.Update(r.Context(), id, input)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, rule)
}

func (h *RuleHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "ruleID")
	if err := h.svc.Delete(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
