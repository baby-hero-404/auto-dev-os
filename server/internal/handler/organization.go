package handler

import (
	"net/http"

	"github.com/auto-code-os/auto-code-os/server/internal/service"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"github.com/go-chi/chi/v5"
)

type OrganizationHandler struct{ svc *service.OrganizationService }

func NewOrganizationHandler(svc *service.OrganizationService) *OrganizationHandler {
	return &OrganizationHandler{svc: svc}
}

func (h *OrganizationHandler) Create(w http.ResponseWriter, r *http.Request) {
	var input models.CreateOrganizationInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	org, err := h.svc.Create(r.Context(), input)
	if err != nil {
		if isValidationErr(err) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, org)
}

func (h *OrganizationHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "orgID")
	org, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "organization not found")
		return
	}
	writeJSON(w, http.StatusOK, org)
}

func (h *OrganizationHandler) List(w http.ResponseWriter, r *http.Request) {
	orgs, err := h.svc.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, orgs)
}

func (h *OrganizationHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "orgID")
	var input models.UpdateOrganizationInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	org, err := h.svc.Update(r.Context(), id, input)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, org)
}

func (h *OrganizationHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "orgID")
	if err := h.svc.Delete(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
