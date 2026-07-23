package llmrunner

import (
	"context"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

type fakeProjectResolver struct {
	project *models.Project
}

func (f *fakeProjectResolver) GetByID(ctx context.Context, id string) (*models.Project, error) {
	return f.project, nil
}

func TestRouteName_PipelineConfigRoutingOverrideWins(t *testing.T) {
	r := Runner{Projects: &fakeProjectResolver{project: &models.Project{
		ID:                "p1",
		DefaultModelLevel: "powerful",
		SmartRouting:      true,
		PipelineConfig:    []byte(`{"version":1,"policies":{"routing":{"analyze":"fast"}}}`),
	}}}
	task := &models.Task{ProjectID: "p1", Complexity: models.TaskComplexityMedium}
	agent := &models.Agent{ModelLevelGroup: "default"}

	got := r.routeName(context.Background(), task, agent, "analyze")
	if got != "fast" {
		t.Errorf("expected pipeline_config routing override 'fast' to win, got %q", got)
	}
}

func TestRouteName_NoOverrideFallsBackToMatrix(t *testing.T) {
	r := Runner{Projects: &fakeProjectResolver{project: &models.Project{
		ID:                "p1",
		DefaultModelLevel: "powerful",
		SmartRouting:      true,
	}}}
	task := &models.Task{ProjectID: "p1", Complexity: models.TaskComplexityMedium}
	agent := &models.Agent{ModelLevelGroup: "default"}

	got := r.routeName(context.Background(), task, agent, "analyze")
	if got != "fast" {
		t.Errorf("expected built-in matrix level 'fast' for analyze, got %q", got)
	}
}
