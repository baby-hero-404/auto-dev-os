package handler

import (
	"net/http"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"github.com/go-chi/chi/v5"
)

type ModelRouteHandler struct {
	svc ModelRouteService
}

func NewModelRouteHandler(svc ModelRouteService) *ModelRouteHandler {
	return &ModelRouteHandler{svc: svc}
}

func (h *ModelRouteHandler) Create(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgID")
	var input models.CreateModelRouteInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	route, err := h.svc.Create(r.Context(), orgID, input)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, route)
}

func (h *ModelRouteHandler) List(w http.ResponseWriter, r *http.Request) {
	routes, err := h.svc.ListByOrg(r.Context(), chi.URLParam(r, "orgID"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, routes)
}

func (h *ModelRouteHandler) Update(w http.ResponseWriter, r *http.Request) {
	var input models.UpdateModelRouteInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	route, err := h.svc.Update(r.Context(), chi.URLParam(r, "routeID"), input)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, route)
}

func (h *ModelRouteHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Delete(r.Context(), chi.URLParam(r, "routeID")); err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
