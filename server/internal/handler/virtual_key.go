package handler

import (
	"net/http"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"github.com/go-chi/chi/v5"
)

type VirtualKeyHandler struct {
	svc VirtualKeyService
}

func NewVirtualKeyHandler(svc VirtualKeyService) *VirtualKeyHandler {
	return &VirtualKeyHandler{svc: svc}
}

func (h *VirtualKeyHandler) Create(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgID")
	var input models.CreateVirtualKeyInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	key, err := h.svc.Create(r.Context(), orgID, input)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, key)
}

func (h *VirtualKeyHandler) List(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgID")
	keys, err := h.svc.ListByOrg(r.Context(), orgID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, keys)
}

func (h *VirtualKeyHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	key, err := h.svc.GetByID(r.Context(), chi.URLParam(r, "virtualKeyID"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, key)
}

func (h *VirtualKeyHandler) Update(w http.ResponseWriter, r *http.Request) {
	var input models.UpdateVirtualKeyInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	key, err := h.svc.Update(r.Context(), chi.URLParam(r, "virtualKeyID"), input)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, key)
}

func (h *VirtualKeyHandler) Revoke(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Revoke(r.Context(), chi.URLParam(r, "virtualKeyID")); err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
