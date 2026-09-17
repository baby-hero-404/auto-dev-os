package orchestrator

import (
	"context"
	"strings"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator/engine"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

type fakeProjectRepo struct {
	project *models.Project
}

func (f *fakeProjectRepo) GetByID(ctx context.Context, id string) (*models.Project, error) {
	return f.project, nil
}

func TestFinishCLIRun_AuthInvalidConfirmed_MarksReauth(t *testing.T) {
	credStatus := &fakeCredStatusSetter{}
	orch := New(nil, &mockWorkflowRepo{job: &models.WorkflowJob{}}, nil, nil,
		WithCredentialStatusSetter(credStatus),
	)
	orch.finishCLIRun(context.Background(), "task-1", "job-1", "code_backend", "cred-1", "", &engine.CodeStepResult{
		Success: false, AuthInvalid: true, AuthInvalidConfirmed: true, Error: "credential not authenticated",
	})
	if len(credStatus.ids) != 1 || credStatus.ids[0] != "cred-1" {
		t.Errorf("expected MarkNeedsReauth called once for cred-1, got %+v", credStatus.ids)
	}
}

// TestFinishCLIRun_AuthInvalidSuspected_DoesNotMarkReauth guards Option C's
// suspected side: a generic-fallback-only match must NOT disable the
// credential — it's left active to retry normally, with only a warning
// logged (see cli_spec_step.go's mirrored policy for the same reasoning).
func TestFinishCLIRun_AuthInvalidSuspected_DoesNotMarkReauth(t *testing.T) {
	credStatus := &fakeCredStatusSetter{}
	workflows := &mockWorkflowRepo{job: &models.WorkflowJob{}}
	orch := New(nil, workflows, nil, nil,
		WithCredentialStatusSetter(credStatus),
	)
	orch.finishCLIRun(context.Background(), "task-1", "job-1", "code_backend", "cred-1", "", &engine.CodeStepResult{
		Success: false, AuthInvalid: true, AuthInvalidConfirmed: false, Error: "possible auth failure, unconfirmed signature",
	})
	if len(credStatus.ids) != 0 {
		t.Errorf("expected MarkNeedsReauth NOT called for a merely suspected match, got %+v", credStatus.ids)
	}
	var foundWarn bool
	for _, entry := range workflows.logs {
		if entry.Level == "warn" && strings.Contains(entry.Message, "suspected auth-invalid") {
			foundWarn = true
		}
	}
	if !foundWarn {
		t.Errorf("expected a warn-level log entry about the suspected auth-invalid match, got: %+v", workflows.logs)
	}
}

type mockArtifactRepoForCLI struct {
	ArtifactRepository
	saved []string
}

func (m *mockArtifactRepoForCLI) ListByTaskID(ctx context.Context, taskID string) ([]models.WorkflowArtifact, error) {
	return nil, nil
}

func (m *mockArtifactRepoForCLI) Create(ctx context.Context, art *models.WorkflowArtifact) error {
	m.saved = append(m.saved, art.Type)
	return nil
}

func (m *mockArtifactRepoForCLI) SaveArtifact(ctx context.Context, jobID, taskID, stepID, artType string, payload any) error {
	m.saved = append(m.saved, artType)
	return nil
}

func TestFinishCLIRun_SavesSessionIDArtifact(t *testing.T) {
	arts := &mockArtifactRepoForCLI{}
	orch := New(nil, &mockWorkflowRepo{job: &models.WorkflowJob{}}, nil, nil,
		WithArtifactRepository(arts),
	)
	orch.finishCLIRun(context.Background(), "task-1", "job-1", "code_backend", "cred-1", "", &engine.CodeStepResult{
		Success: false, SessionID: "test-session-123", IdleTimeoutHit: true,
	})

	found := false
	for _, a := range arts.saved {
		if a == "cli_session_id" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected cli_session_id artifact to be saved, got %v", arts.saved)
	}
}
