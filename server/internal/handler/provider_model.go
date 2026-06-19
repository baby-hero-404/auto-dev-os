package handler

import (
	"net/http"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"github.com/go-chi/chi/v5"
)

type ProviderModelHandler struct {
	svc ProviderModelService
}

func NewProviderModelHandler(svc ProviderModelService) *ProviderModelHandler {
	return &ProviderModelHandler{svc: svc}
}

func (h *ProviderModelHandler) Create(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgID")
	var input models.CreateProviderModelInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	model, err := h.svc.Create(r.Context(), orgID, input)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, model)
}

func (h *ProviderModelHandler) List(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgID")
	provider := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("provider")))
	levelGroup := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("level_group")))

	if levelGroup != "" {
		if levelGroup != models.ModelLevelFast && levelGroup != models.ModelLevelBalanced && levelGroup != models.ModelLevelPowerful {
			writeError(w, http.StatusBadRequest, "invalid level_group: must be fast, balanced, or powerful")
			return
		}
	}

	var filter models.ProviderModelFilter
	if provider != "" {
		filter.Provider = &provider
	}
	if levelGroup != "" {
		filter.LevelGroup = &levelGroup
	}

	modelsList, err := h.svc.ListByOrg(r.Context(), orgID, filter)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, modelsList)
}

func (h *ProviderModelHandler) Update(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgID")
	var input models.UpdateProviderModelInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	model, err := h.svc.Update(r.Context(), orgID, chi.URLParam(r, "modelID"), input)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, model)
}

func (h *ProviderModelHandler) Delete(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgID")
	if err := h.svc.Delete(r.Context(), orgID, chi.URLParam(r, "modelID")); err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
