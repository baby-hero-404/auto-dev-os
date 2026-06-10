package handler

import (
	"net/http"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"github.com/go-chi/chi/v5"
)

type GitAccountHandler struct {
	svc GitAccountService
}

func NewGitAccountHandler(svc GitAccountService) *GitAccountHandler {
	return &GitAccountHandler{svc: svc}
}

func (h *GitAccountHandler) Create(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgID")
	var input models.CreateGitAccountInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	acc, err := h.svc.Create(r.Context(), orgID, input)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, acc)
}

func (h *GitAccountHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "accID")
	acc, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, acc)
}

func (h *GitAccountHandler) List(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgID")
	accounts, err := h.svc.ListByOrgID(r.Context(), orgID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, accounts)
}

func (h *GitAccountHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "accID")
	var input models.UpdateGitAccountInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	acc, err := h.svc.Update(r.Context(), id, input)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, acc)
}

func (h *GitAccountHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "accID")
	if err := h.svc.Delete(r.Context(), id); err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *GitAccountHandler) Test(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "accID")
	if err := h.svc.TestConnection(r.Context(), id); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, envelope{"status": "connected"})
}
