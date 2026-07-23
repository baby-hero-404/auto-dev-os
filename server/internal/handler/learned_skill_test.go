package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"github.com/go-chi/chi/v5"
)

type fakeLearnedSkillService struct {
	skills      map[string]*models.LearnedSkill
	byProjectID map[string][]models.LearnedSkill
	updateErr   error
}

func (f *fakeLearnedSkillService) ListByProjectID(ctx context.Context, projectID string) ([]models.LearnedSkill, error) {
	return f.byProjectID[projectID], nil
}

func (f *fakeLearnedSkillService) GetByID(ctx context.Context, id string) (*models.LearnedSkill, error) {
	if s, ok := f.skills[id]; ok {
		return s, nil
	}
	return nil, http.ErrNoLocation
}

func (f *fakeLearnedSkillService) Update(ctx context.Context, id string, input models.UpdateLearnedSkillInput) (*models.LearnedSkill, error) {
	if f.updateErr != nil {
		return nil, f.updateErr
	}
	s, ok := f.skills[id]
	if !ok {
		return nil, http.ErrNoLocation
	}
	if input.Status != nil {
		s.Status = *input.Status
	}
	return s, nil
}

func (f *fakeLearnedSkillService) Delete(ctx context.Context, id string) error {
	delete(f.skills, id)
	return nil
}

func TestLearnedSkillHandler_ListByProject(t *testing.T) {
	svc := &fakeLearnedSkillService{
		byProjectID: map[string][]models.LearnedSkill{
			"proj-1": {{ID: "s1", Title: "Test skill"}},
		},
	}
	h := NewLearnedSkillHandler(svc)

	r := chi.NewRouter()
	r.Get("/projects/{projectID}/learned-skills", h.ListLearnedSkills)

	req := httptest.NewRequest(http.MethodGet, "/projects/proj-1/learned-skills", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body struct {
		Skills []models.LearnedSkill `json:"skills"`
	}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if len(body.Skills) != 1 || body.Skills[0].ID != "s1" {
		t.Fatalf("expected 1 skill s1, got %+v", body.Skills)
	}
}

func TestLearnedSkillHandler_UpdateApprovesDraft(t *testing.T) {
	svc := &fakeLearnedSkillService{
		skills: map[string]*models.LearnedSkill{
			"s1": {ID: "s1", Status: models.LearnedSkillStatusDraft},
		},
	}
	h := NewLearnedSkillHandler(svc)

	r := chi.NewRouter()
	r.Patch("/learned-skills/{skillID}", h.UpdateLearnedSkill)

	req := httptest.NewRequest(http.MethodPatch, "/learned-skills/s1", strings.NewReader(`{"status":"active"}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if svc.skills["s1"].Status != models.LearnedSkillStatusActive {
		t.Errorf("expected status active after approve, got %s", svc.skills["s1"].Status)
	}
}

func TestLearnedSkillHandler_UpdateInvalidBody(t *testing.T) {
	h := NewLearnedSkillHandler(&fakeLearnedSkillService{skills: map[string]*models.LearnedSkill{}})

	r := chi.NewRouter()
	r.Patch("/learned-skills/{skillID}", h.UpdateLearnedSkill)

	req := httptest.NewRequest(http.MethodPatch, "/learned-skills/s1", strings.NewReader(`not-json`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid body, got %d", w.Code)
	}
}
