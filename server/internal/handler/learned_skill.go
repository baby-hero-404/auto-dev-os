package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"github.com/go-chi/chi/v5"
)

// LearnedSkillService is the persistence dependency for LearnedSkillHandler.
type LearnedSkillService interface {
	ListByProjectID(ctx context.Context, projectID string) ([]models.LearnedSkill, error)
	GetByID(ctx context.Context, id string) (*models.LearnedSkill, error)
	Update(ctx context.Context, id string, input models.UpdateLearnedSkillInput) (*models.LearnedSkill, error)
	Delete(ctx context.Context, id string) error
}

// LearnedSkillHandler serves CRUD + approve/activate/disable for the
// reusable-skills-system's learned skills (REQ-005) — distinct from the
// agent tool/plugin catalog's SkillHandler.
type LearnedSkillHandler struct {
	repo LearnedSkillService
}

func NewLearnedSkillHandler(repo LearnedSkillService) *LearnedSkillHandler {
	return &LearnedSkillHandler{repo: repo}
}

// ListLearnedSkills godoc — GET /api/v1/projects/{projectID}/learned-skills
func (h *LearnedSkillHandler) ListLearnedSkills(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
	if projectID == "" {
		writeError(w, http.StatusBadRequest, "project_id is required")
		return
	}
	skills, err := h.repo.ListByProjectID(r.Context(), projectID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, envelope{"skills": skills})
}

// GetLearnedSkill godoc — GET /api/v1/learned-skills/{skillID}
func (h *LearnedSkillHandler) GetLearnedSkill(w http.ResponseWriter, r *http.Request) {
	skillID := chi.URLParam(r, "skillID")
	skill, err := h.repo.GetByID(r.Context(), skillID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, envelope{"skill": skill})
}

// UpdateLearnedSkill godoc — PATCH /api/v1/learned-skills/{skillID}
// Covers edit, activate ({"status":"active"}), disable ({"status":"disabled"}),
// and approve-draft ({"status":"active"}) in one endpoint since they're all
// partial field updates on the same resource.
func (h *LearnedSkillHandler) UpdateLearnedSkill(w http.ResponseWriter, r *http.Request) {
	skillID := chi.URLParam(r, "skillID")

	var input models.UpdateLearnedSkillInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	skill, err := h.repo.Update(r.Context(), skillID, input)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, envelope{"skill": skill})
}

// DeleteLearnedSkill godoc — DELETE /api/v1/learned-skills/{skillID}
func (h *LearnedSkillHandler) DeleteLearnedSkill(w http.ResponseWriter, r *http.Request) {
	skillID := chi.URLParam(r, "skillID")
	if err := h.repo.Delete(r.Context(), skillID); err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
