package handler

import (
	"net/http"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"github.com/go-chi/chi/v5"
)

type RepositoryHandler struct{ svc RepositoryService }

func NewRepositoryHandler(svc RepositoryService) *RepositoryHandler {
	return &RepositoryHandler{svc: svc}
}

func (h *RepositoryHandler) ValidateToken(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "repoID")
	if err := h.svc.ValidateToken(r.Context(), id); err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, envelope{"valid": true})
}

func (h *RepositoryHandler) ListRemoteRepos(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	repos, err := h.svc.ListRemoteRepos(r.Context(), token)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, repos)
}

func (h *RepositoryHandler) Clone(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "repoID")
	repo, err := h.svc.Clone(r.Context(), id)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, repo)
}

func (h *RepositoryHandler) Create(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
	var input models.CreateRepositoryInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	repo, err := h.svc.Create(r.Context(), projectID, input)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, repo)
}

func (h *RepositoryHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "repoID")
	repo, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, repo)
}

func (h *RepositoryHandler) List(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
	repos, err := h.svc.ListByProjectID(r.Context(), projectID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, repos)
}

func (h *RepositoryHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "repoID")
	var input models.UpdateRepositoryInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	repo, err := h.svc.Update(r.Context(), id, input)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, repo)
}

func (h *RepositoryHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "repoID")
	if err := h.svc.Delete(r.Context(), id); err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
