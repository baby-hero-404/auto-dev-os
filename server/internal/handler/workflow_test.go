package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"
	"time"

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

func TestStreamLogsLoop_NoLostLogsDuringTailRace(t *testing.T) {
	ch := make(chan models.TaskLog, 10)
	proceedTail := make(chan struct{})

	tail := func() ([]models.TaskLog, error) {
		<-proceedTail // block until the test says the tail read may finish
		return []models.TaskLog{{ID: "hist-1", Message: "history"}}, nil
	}

	var mu sync.Mutex
	var emitted []models.TaskLog
	emit := func(log models.TaskLog) {
		mu.Lock()
		emitted = append(emitted, log)
		mu.Unlock()
	}

	ctx, cancel := context.WithCancel(context.Background())
	loopDone := make(chan struct{})
	go func() {
		defer close(loopDone)
		_ = streamLogsLoop(ctx, ch, tail, emit)
	}()

	// Simulate a log broadcast arriving WHILE the tail read is still in flight — this is
	// exactly the window where the pre-fix code raced two consumers on ch.
	ch <- models.TaskLog{ID: "live-1", Message: "during tail"}
	time.Sleep(20 * time.Millisecond) // let the background buffering goroutine consume it
	close(proceedTail)                // let tail() return

	time.Sleep(20 * time.Millisecond)
	ch <- models.TaskLog{ID: "live-2", Message: "after tail"}
	time.Sleep(20 * time.Millisecond)

	cancel()
	<-loopDone

	mu.Lock()
	defer mu.Unlock()
	ids := make([]string, len(emitted))
	for i, e := range emitted {
		ids[i] = e.ID
	}
	want := []string{"hist-1", "live-1", "live-2"}
	if !reflect.DeepEqual(ids, want) {
		t.Fatalf("lost or misordered logs: got %v, want %v", ids, want)
	}
}
