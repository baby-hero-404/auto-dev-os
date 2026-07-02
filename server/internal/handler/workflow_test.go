package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"github.com/go-chi/chi/v5"
)

type mockArtifactRepo struct {
	artifacts []models.WorkflowArtifact
}

func (m *mockArtifactRepo) Create(ctx context.Context, artifact *models.WorkflowArtifact) error {
	m.artifacts = append(m.artifacts, *artifact)
	return nil
}

func (m *mockArtifactRepo) ListByJobID(ctx context.Context, jobID string) ([]models.WorkflowArtifact, error) {
	var result []models.WorkflowArtifact
	for _, a := range m.artifacts {
		if a.JobID == jobID {
			result = append(result, a)
		}
	}
	return result, nil
}

func (m *mockArtifactRepo) ListByTaskID(ctx context.Context, taskID string) ([]models.WorkflowArtifact, error) {
	return nil, nil
}

func (m *mockArtifactRepo) DeleteByTaskID(ctx context.Context, taskID string) error {
	return nil
}

func TestWorkflowHandler_Artifacts(t *testing.T) {
	repo := &mockArtifactRepo{
		artifacts: []models.WorkflowArtifact{
			{
				ID:    "art-1",
				JobID: "job-1",
				Step:  "code",
				Type:  "patch",
			},
		},
	}
	orch := orchestrator.New(nil, nil, nil, nil, orchestrator.WithArtifactRepository(repo))

	h := NewWorkflowHandler(orch)

	r := chi.NewRouter()
	r.Get("/workflows/{jobID}/artifacts", h.Artifacts)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/workflows/job-1/artifacts", nil)
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var resp []models.WorkflowArtifact
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(resp) != 1 {
		t.Errorf("expected 1 artifact, got %d", len(resp))
	}
	if resp[0].ID != "art-1" {
		t.Errorf("expected art-1, got %s", resp[0].ID)
	}
}
