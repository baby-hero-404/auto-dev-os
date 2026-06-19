package handler

import (
	"net/http"

	"github.com/auto-code-os/auto-code-os/server/internal/service"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"github.com/go-chi/chi/v5"
)

type RuleHandler struct{ svc RuleService }

func NewRuleHandler(svc RuleService) *RuleHandler {
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
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, rule)
}

func (h *RuleHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "ruleID")
	claims, _ := r.Context().Value(authClaimsKey).(*service.TokenClaims)
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	rule, err := h.svc.GetByID(r.Context(), id, claims.OrgID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, rule)
}

func (h *RuleHandler) List(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
	rules, err := h.svc.ListByProjectID(r.Context(), projectID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, rules)
}

func (h *RuleHandler) CreateGlobal(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgID")
	var input models.CreateRuleInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	rule, err := h.svc.CreateGlobal(r.Context(), orgID, input)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, rule)
}

func (h *RuleHandler) ListGlobal(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgID")
	rules, err := h.svc.ListGlobalByOrgID(r.Context(), orgID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, rules)
}

func (h *RuleHandler) SeedGlobal(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgID")
	rules, err := h.svc.SeedGlobalDefaultRules(r.Context(), orgID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, rules)
}

func (h *RuleHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "ruleID")
	claims, _ := r.Context().Value(authClaimsKey).(*service.TokenClaims)
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	var input models.UpdateRuleInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	rule, err := h.svc.Update(r.Context(), id, claims.OrgID, claims.Role, input)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, rule)
}

func (h *RuleHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "ruleID")
	claims, _ := r.Context().Value(authClaimsKey).(*service.TokenClaims)
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	if err := h.svc.Delete(r.Context(), id, claims.OrgID); err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *RuleHandler) Seed(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
	rules, err := h.svc.SeedDefaultRules(r.Context(), projectID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, rules)
}
