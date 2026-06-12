package handler

import (
	"net/http"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"github.com/go-chi/chi/v5"
)

type ProviderCredentialHandler struct {
	svc ProviderCredentialService
}

func NewProviderCredentialHandler(svc ProviderCredentialService) *ProviderCredentialHandler {
	return &ProviderCredentialHandler{svc: svc}
}

func (h *ProviderCredentialHandler) Create(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgID")
	var input models.CreateProviderCredentialInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	cred, err := h.svc.Create(r.Context(), orgID, input)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, cred)
}

func (h *ProviderCredentialHandler) List(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgID")
	creds, err := h.svc.ListByOrg(r.Context(), orgID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, creds)
}

func (h *ProviderCredentialHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "credentialID")
	var input models.UpdateProviderCredentialInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	cred, err := h.svc.Update(r.Context(), id, input)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, cred)
}

func (h *ProviderCredentialHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "credentialID")
	if err := h.svc.Delete(r.Context(), id); err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *ProviderCredentialHandler) Test(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "credentialID")
	if err := h.svc.TestConnection(r.Context(), id); err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, envelope{"status": "configured"})
}

func (h *ProviderCredentialHandler) TestInput(w http.ResponseWriter, r *http.Request) {
	var input models.TestProviderCredentialInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.svc.TestConnectionInput(r.Context(), input); err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, envelope{"status": "ok"})
}
